// Package internal provides the unified analyzer that uses SSA-based pattern matching.
package internal

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/mpyw/goroutinectx/internal/context"
	"github.com/mpyw/goroutinectx/internal/directives/carrier"
	"github.com/mpyw/goroutinectx/internal/directives/ignore"
	"github.com/mpyw/goroutinectx/internal/directives/spawner"
	"github.com/mpyw/goroutinectx/internal/patterns"
	"github.com/mpyw/goroutinectx/internal/registry"
	internalssa "github.com/mpyw/goroutinectx/internal/ssa"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Checker is the unified SSA-based checker.
type Checker struct {
	registry     *registry.Registry
	goPatterns   []patterns.GoStmtPattern
	spawners     *spawner.Map
	ssaProg      *internalssa.Program
	tracer       *internalssa.Tracer
	carriers     []carrier.Carrier
	ignoreMaps   map[string]ignore.Map
	skipFiles    map[string]bool
	checkerNames map[string]ignore.CheckerName // pattern name -> checker name for ignore
}

// New creates a new unified checker.
func New(
	reg *registry.Registry,
	goPatterns []patterns.GoStmtPattern,
	spawners *spawner.Map,
	ssaProg *internalssa.Program,
	carriers []carrier.Carrier,
	ignoreMaps map[string]ignore.Map,
	skipFiles map[string]bool,
	checkerNames map[string]ignore.CheckerName,
) *Checker {
	return &Checker{
		registry:     reg,
		goPatterns:   goPatterns,
		spawners:     spawners,
		ssaProg:      ssaProg,
		tracer:       internalssa.NewTracer(),
		carriers:     carriers,
		ignoreMaps:   ignoreMaps,
		skipFiles:    skipFiles,
		checkerNames: checkerNames,
	}
}

// Run executes the checker on the given pass.
func (c *Checker) Run(pass *analysis.Pass, insp *inspector.Inspector) {
	// Build context scopes for functions with context parameters
	funcScopes := buildFuncScopes(pass, insp, c.carriers)

	// Node types we're interested in
	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
		(*ast.FuncLit)(nil),
		(*ast.GoStmt)(nil),
		(*ast.CallExpr)(nil),
	}

	// Check nodes within context-aware functions
	insp.WithStack(nodeFilter, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}

		filename := pass.Fset.Position(n.Pos()).Filename
		if c.skipFiles[filename] {
			return true
		}

		scope := findEnclosingScope(funcScopes, stack)
		if scope == nil {
			return true // No context in scope
		}

		cctx := &context.CheckContext{
			Pass:     pass,
			Tracer:   c.tracer,
			SSAProg:  c.ssaProg,
			CtxNames: scope.ctxNames,
			Carriers: c.carriers,
		}

		switch node := n.(type) {
		case *ast.GoStmt:
			c.checkGoStmt(cctx, node, scope)
		case *ast.CallExpr:
			c.checkCallExpr(cctx, node, scope)
		}

		return true
	})
}

// checkGoStmt checks a go statement against all registered go patterns.
func (c *Checker) checkGoStmt(cctx *context.CheckContext, stmt *ast.GoStmt, scope *contextScope) {
	for _, pattern := range c.goPatterns {
		checkerName := c.getCheckerName(pattern.Name())
		if c.shouldIgnore(cctx.Pass, stmt.Pos(), checkerName) {
			continue
		}

		result := pattern.CheckGoStmt(cctx, stmt)
		if result.OK {
			continue
		}

		var msg string
		if result.DeferOnly {
			// Deriver found but only in defer - use special message
			msg = pattern.DeferMessage(scope.ctxName())
		} else {
			msg = pattern.Message(scope.ctxName())
		}

		if msg != "" {
			cctx.Pass.Reportf(stmt.Pos(), "%s", msg)
		}
	}
}

// checkCallExpr checks a call expression against registered API patterns and spawner directives.
func (c *Checker) checkCallExpr(cctx *context.CheckContext, call *ast.CallExpr, scope *contextScope) {
	// Check against registered API patterns
	c.checkRegistryCall(cctx, call, scope)

	// Check spawner directives
	c.checkSpawnerCall(cctx, call, scope)
}

// checkRegistryCall checks a call expression against registered API patterns.
func (c *Checker) checkRegistryCall(cctx *context.CheckContext, call *ast.CallExpr, scope *contextScope) {
	entry, callbackArg := c.registry.Match(cctx.Pass, call)
	if entry == nil {
		return
	}

	checkerName := c.getCheckerName(entry.Pattern.Name())
	if c.shouldIgnore(cctx.Pass, call.Pos(), checkerName) {
		return
	}

	// Handle variadic APIs (e.g., DoAllFns(ctx, fn1, fn2, ...))
	if entry.API.Variadic {
		c.checkVariadicCallExpr(cctx, call, entry, scope)
		return
	}

	if !entry.Pattern.Check(cctx, call, callbackArg) {
		msg := entry.Pattern.Message(entry.API.FullName(), scope.ctxName())
		// Report at method selector position for chained calls
		reportPos := getCallReportPos(call)
		cctx.Pass.Reportf(reportPos, "%s", msg)
	}
}

// checkSpawnerCall checks if this is a call to a spawner-marked function.
func (c *Checker) checkSpawnerCall(cctx *context.CheckContext, call *ast.CallExpr, scope *contextScope) {
	if c.spawners == nil || c.spawners.Len() == 0 {
		return
	}

	fn := spawner.GetFuncFromCall(cctx.Pass, call)
	if fn == nil || !c.spawners.IsSpawner(fn) {
		return
	}

	if c.shouldIgnore(cctx.Pass, call.Pos(), ignore.Spawner) {
		return
	}

	// Check all func-type arguments
	funcArgs := spawner.FindFuncArgs(cctx.Pass, call)
	pattern := &patterns.ClosureCapturesCtx{}

	for _, arg := range funcArgs {
		if !pattern.Check(cctx, call, arg) {
			cctx.Pass.Reportf(arg.Pos(), "%s() func argument should use context %q", fn.Name(), scope.ctxName())
		}
	}
}

// getCallReportPos returns the best position to report for a call expression.
// For method calls, this is the selector (method name) position.
// For other calls, this is the call position.
func getCallReportPos(call *ast.CallExpr) token.Pos {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		return sel.Sel.Pos()
	}
	return call.Pos()
}

// checkVariadicCallExpr checks each callback argument in a variadic API call.
func (c *Checker) checkVariadicCallExpr(
	cctx *context.CheckContext,
	call *ast.CallExpr,
	entry *registry.Entry,
	scope *contextScope,
) {
	startIdx := entry.API.CallbackArgIdx
	if startIdx < 0 || startIdx >= len(call.Args) {
		return
	}

	// Check if this is a variadic expansion (e.g., DoAllFns(ctx, slice...))
	isVariadicExpansion := call.Ellipsis.IsValid()

	for i := startIdx; i < len(call.Args); i++ {
		arg := call.Args[i]
		if !entry.Pattern.Check(cctx, call, arg) {
			var msg string
			if isVariadicExpansion {
				// For variadic expansion, we can't determine the position
				msg = entry.API.FullName() + "() variadic argument should call goroutine deriver"
			} else {
				// Create message with argument position (1-based for human readability)
				argNum := i + 1
				msg = formatVariadicMessage(entry.API.FullName(), argNum)
			}
			// Report at the call position (where the // want comment is)
			cctx.Pass.Reportf(call.Pos(), "%s", msg)
		}
	}
}

// formatVariadicMessage formats a diagnostic message with argument position.
func formatVariadicMessage(apiName string, argNum int) string {
	return apiName + "() " + ordinal(argNum) + " argument should call goroutine deriver"
}

// ordinal returns the ordinal form of a number (1st, 2nd, 3rd, 4th, etc.)
func ordinal(n int) string {
	suffix := "th"
	switch n % 10 {
	case 1:
		if n%100 != 11 {
			suffix = "st"
		}
	case 2:
		if n%100 != 12 {
			suffix = "nd"
		}
	case 3:
		if n%100 != 13 {
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", n, suffix)
}

// shouldIgnore checks if the position should be ignored for the given checker.
func (c *Checker) shouldIgnore(pass *analysis.Pass, pos token.Pos, checkerName ignore.CheckerName) bool {
	filename := pass.Fset.Position(pos).Filename
	ignoreMap, ok := c.ignoreMaps[filename]
	if !ok {
		return false
	}
	line := pass.Fset.Position(pos).Line
	return ignoreMap.ShouldIgnore(line, checkerName)
}

// getCheckerName maps pattern name to ignore checker name.
func (c *Checker) getCheckerName(patternName string) ignore.CheckerName {
	if name, ok := c.checkerNames[patternName]; ok {
		return name
	}
	return ignore.CheckerName(patternName)
}

// contextScope holds context information for a function scope.
type contextScope struct {
	ctxNames []string
}

func (s *contextScope) ctxName() string {
	if len(s.ctxNames) > 0 {
		return s.ctxNames[0]
	}
	return "ctx"
}

// buildFuncScopes identifies functions with context parameters.
func buildFuncScopes(
	pass *analysis.Pass,
	insp *inspector.Inspector,
	carriers []carrier.Carrier,
) map[ast.Node]*contextScope {
	funcScopes := make(map[ast.Node]*contextScope)

	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil), (*ast.FuncLit)(nil)}, func(n ast.Node) {
		var fnType *ast.FuncType

		switch fn := n.(type) {
		case *ast.FuncDecl:
			fnType = fn.Type
		case *ast.FuncLit:
			fnType = fn.Type
		}

		if scope := findContextScope(pass, fnType, carriers); scope != nil {
			funcScopes[n] = scope
		}
	})

	return funcScopes
}

// findContextScope checks if the function has context parameters.
func findContextScope(pass *analysis.Pass, fnType *ast.FuncType, carriers []carrier.Carrier) *contextScope {
	if fnType == nil || fnType.Params == nil {
		return nil
	}

	var ctxNames []string

	for _, field := range fnType.Params.List {
		typ := pass.TypesInfo.TypeOf(field.Type)
		if typ == nil {
			continue
		}

		if typeutil.IsContextOrCarrierType(typ, carriers) {
			for _, name := range field.Names {
				ctxNames = append(ctxNames, name.Name)
			}
		}
	}

	if len(ctxNames) == 0 {
		return nil
	}

	return &contextScope{ctxNames: ctxNames}
}

// findEnclosingScope finds the closest enclosing function with a context parameter.
func findEnclosingScope(funcScopes map[ast.Node]*contextScope, stack []ast.Node) *contextScope {
	for i := len(stack) - 1; i >= 0; i-- {
		if scope, ok := funcScopes[stack[i]]; ok {
			return scope
		}
	}

	return nil
}
