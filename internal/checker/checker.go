// Package checker provides a unified checker that uses SSA-based pattern matching.
package checker

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/mpyw/goroutinectx/internal/directives/carrier"
	"github.com/mpyw/goroutinectx/internal/directives/ignore"
	"github.com/mpyw/goroutinectx/internal/patterns"
	"github.com/mpyw/goroutinectx/internal/registry"
	internalssa "github.com/mpyw/goroutinectx/internal/ssa"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Checker is the unified SSA-based checker.
type Checker struct {
	registry     *registry.Registry
	goPatterns   []patterns.GoStmtPattern
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
	ssaProg *internalssa.Program,
	carriers []carrier.Carrier,
	ignoreMaps map[string]ignore.Map,
	skipFiles map[string]bool,
	checkerNames map[string]ignore.CheckerName,
) *Checker {
	return &Checker{
		registry:     reg,
		goPatterns:   goPatterns,
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

		cctx := &patterns.CheckContext{
			Pass:    pass,
			Tracer:  c.tracer,
			SSAProg: c.ssaProg,
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
func (c *Checker) checkGoStmt(cctx *patterns.CheckContext, stmt *ast.GoStmt, scope *contextScope) {
	for _, pattern := range c.goPatterns {
		checkerName := c.getCheckerName(pattern.Name())
		if c.shouldIgnore(cctx.Pass, stmt.Pos(), checkerName) {
			continue
		}

		if !pattern.CheckGoStmt(cctx, stmt) {
			msg := pattern.Message(scope.ctxName())
			cctx.Pass.Reportf(stmt.Pos(), "%s", msg)
		}
	}
}

// checkCallExpr checks a call expression against registered API patterns.
func (c *Checker) checkCallExpr(cctx *patterns.CheckContext, call *ast.CallExpr, scope *contextScope) {
	entry, callbackArg := c.registry.Match(cctx.Pass, call)
	if entry == nil {
		return
	}

	checkerName := c.getCheckerName(entry.Pattern.Name())
	if c.shouldIgnore(cctx.Pass, call.Pos(), checkerName) {
		return
	}

	if !entry.Pattern.Check(cctx, call, callbackArg) {
		msg := entry.Pattern.Message(entry.API.FullName(), scope.ctxName())
		cctx.Pass.Reportf(call.Pos(), "%s", msg)
	}
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
