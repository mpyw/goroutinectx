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
)

// Runner is the unified SSA-based checker.
type Runner struct {
	registry   *registry.Registry
	spawners   *spawner.Map
	ssaProg    *internalssa.Program
	tracer     *internalssa.Tracer
	carriers   []carrier.Carrier
	ignoreMaps map[string]ignore.Map
	skipFiles  map[string]bool
}

// NewRunner creates a new unified checker.
func NewRunner(
	reg *registry.Registry,
	spawners *spawner.Map,
	ssaProg *internalssa.Program,
	carriers []carrier.Carrier,
	ignoreMaps map[string]ignore.Map,
	skipFiles map[string]bool,
) *Runner {
	return &Runner{
		registry:   reg,
		spawners:   spawners,
		ssaProg:    ssaProg,
		tracer:     internalssa.NewTracer(),
		carriers:   carriers,
		ignoreMaps: ignoreMaps,
		skipFiles:  skipFiles,
	}
}

// Run executes the checker on the given pass.
func (c *Runner) Run(pass *analysis.Pass, insp *inspector.Inspector) {
	// Build context scopes for functions with context parameters
	funcScopes := context.BuildFuncScopes(pass, insp, c.carriers)

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

		scope := context.FindEnclosingScope(funcScopes, stack)
		if scope == nil {
			return true // No context in scope
		}

		cctx := &context.CheckContext{
			Pass:     pass,
			Tracer:   c.tracer,
			SSAProg:  c.ssaProg,
			CtxNames: scope.CtxNames,
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
func (c *Runner) checkGoStmt(cctx *context.CheckContext, stmt *ast.GoStmt, scope *context.Scope) {
	for _, pattern := range c.registry.GoStmtPatterns() {
		if c.shouldIgnore(cctx.Pass, stmt.Pos(), pattern.CheckerName()) {
			continue
		}

		result := pattern.Check(cctx, stmt)
		if result.OK {
			continue
		}

		var msg string
		if result.DeferOnly {
			// Deriver found but only in defer - use special message
			msg = pattern.DeferMessage(scope.CtxName())
		} else {
			msg = pattern.Message(scope.CtxName())
		}

		if msg != "" {
			cctx.Pass.Reportf(stmt.Pos(), "%s", msg)
		}
	}
}

// checkCallExpr checks a call expression against registered API patterns and spawner directives.
func (c *Runner) checkCallExpr(cctx *context.CheckContext, call *ast.CallExpr, scope *context.Scope) {
	// Check against registered API patterns
	c.checkRegistryCall(cctx, call, scope)

	// Check spawner directives
	c.checkSpawnerCall(cctx, call, scope)
}

// checkRegistryCall checks a call expression against registered API patterns.
func (c *Runner) checkRegistryCall(cctx *context.CheckContext, call *ast.CallExpr, scope *context.Scope) {
	// Try CallArgPattern match first
	c.checkCallArgPattern(cctx, call, scope)

	// Try TaskSourcePattern match
	c.checkTaskSourcePattern(cctx, call, scope)
}

// checkCallArgPattern checks a call against registered CallArg APIs.
func (c *Runner) checkCallArgPattern(cctx *context.CheckContext, call *ast.CallExpr, scope *context.Scope) {
	entry, callbackArg := c.registry.MatchCallArg(cctx.Pass, call)
	if entry == nil {
		return
	}

	// Skip if no patterns (API registered for detection only, e.g., for spawnerlabel)
	if len(entry.Patterns) == 0 {
		return
	}

	// Handle variadic APIs (e.g., DoAllFns(ctx, fn1, fn2, ...))
	if entry.Variadic {
		c.checkVariadicCallExpr(cctx, call, entry, scope)
		return
	}

	// Check each pattern - all must be satisfied
	for _, pattern := range entry.Patterns {
		if c.shouldIgnore(cctx.Pass, call.Pos(), pattern.CheckerName()) {
			continue
		}

		if !pattern.Check(cctx, callbackArg, entry.TaskConstructor) {
			msg := pattern.Message(entry.Spec.FullName(), scope.CtxName())
			// Report at method selector position for chained calls
			reportPos := getCallReportPos(call)
			cctx.Pass.Reportf(reportPos, "%s", msg)
		}
	}
}

// checkTaskSourcePattern checks a call against registered TaskSource APIs.
func (c *Runner) checkTaskSourcePattern(cctx *context.CheckContext, call *ast.CallExpr, scope *context.Scope) {
	entry := c.registry.MatchTaskSource(cctx.Pass, call)
	if entry == nil {
		return
	}

	// Skip if no patterns
	if len(entry.Patterns) == 0 {
		return
	}

	// Build TaskCheckContext
	tcctx := &patterns.TaskCheckContext{
		CheckContext: cctx,
		Constructor:  entry.TaskConstructor,
	}

	// Check each pattern
	for _, pattern := range entry.Patterns {
		if c.shouldIgnore(cctx.Pass, call.Pos(), pattern.CheckerName()) {
			continue
		}

		if !pattern.Check(tcctx, call) {
			msg := pattern.Message(entry.Spec.FullName(), scope.CtxName())
			reportPos := getCallReportPos(call)
			cctx.Pass.Reportf(reportPos, "%s", msg)
		}
	}
}

// checkSpawnerCall checks if this is a call to a spawner-marked function.
func (c *Runner) checkSpawnerCall(cctx *context.CheckContext, call *ast.CallExpr, scope *context.Scope) {
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
		if !pattern.Check(cctx, arg, nil) {
			cctx.Pass.Reportf(arg.Pos(), "%s() func argument should use context %q", fn.Name(), scope.CtxName())
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
func (c *Runner) checkVariadicCallExpr(
	cctx *context.CheckContext,
	call *ast.CallExpr,
	entry *registry.CallArgEntry,
	scope *context.Scope,
) {
	startIdx := entry.CallbackArgIdx
	if startIdx < 0 || startIdx >= len(call.Args) {
		return
	}

	// Check if this is a variadic expansion (e.g., DoAllFns(ctx, slice...))
	isVariadicExpansion := call.Ellipsis.IsValid()

	for i := startIdx; i < len(call.Args); i++ {
		arg := call.Args[i]
		// Check each pattern for this argument
		for _, pattern := range entry.Patterns {
			if c.shouldIgnore(cctx.Pass, call.Pos(), pattern.CheckerName()) {
				continue
			}

			if !pattern.Check(cctx, arg, entry.TaskConstructor) {
				var msg string
				if isVariadicExpansion {
					// For variadic expansion, we can't determine the position
					msg = entry.Spec.FullName() + "() variadic argument should call goroutine deriver"
				} else {
					// Create message with argument position (1-based for human readability)
					argNum := i + 1
					msg = formatVariadicMessage(entry.Spec.FullName(), argNum)
				}
				// Report at the call position (where the // want comment is)
				cctx.Pass.Reportf(call.Pos(), "%s", msg)
			}
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
func (c *Runner) shouldIgnore(pass *analysis.Pass, pos token.Pos, checkerName ignore.CheckerName) bool {
	filename := pass.Fset.Position(pos).Filename
	ignoreMap, ok := c.ignoreMaps[filename]
	if !ok {
		return false
	}
	line := pass.Fset.Position(pos).Line
	return ignoreMap.ShouldIgnore(line, checkerName)
}
