package patterns

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/mpyw/goroutinectx/internal/context"
	"github.com/mpyw/goroutinectx/internal/directives/deriver"
)

// GoStmtCapturesCtx checks that a go statement's closure captures the outer context.
type GoStmtCapturesCtx struct{}

func (*GoStmtCapturesCtx) Name() string {
	return "GoStmtCapturesCtx"
}

func (*GoStmtCapturesCtx) CheckGoStmt(cctx *context.CheckContext, stmt *ast.GoStmt) GoStmtResult {
	// If no context names in scope (from AST), nothing to check
	if len(cctx.CtxNames) == 0 {
		return GoStmtResult{OK: true}
	}

	// Try SSA-based check first (more accurate, includes nested closures)
	if lit, ok := stmt.Call.Fun.(*ast.FuncLit); ok {
		if result, ok := cctx.FuncLitCapturesContextSSA(lit); ok {
			return GoStmtResult{OK: result}
		}
	}

	// Fall back to AST-based check when SSA fails
	return GoStmtResult{OK: goStmtCheckFromAST(cctx, stmt)}
}

func (*GoStmtCapturesCtx) Message(ctxName string) string {
	return "goroutine does not propagate context \"" + ctxName + "\""
}

func (*GoStmtCapturesCtx) DeferMessage(_ string) string {
	return "" // Not applicable for context capture pattern
}

// goStmtCheckFromAST falls back to AST-based analysis for go statements.
func goStmtCheckFromAST(cctx *context.CheckContext, stmt *ast.GoStmt) bool {
	call := stmt.Call

	// For go func(){}(), check the function literal
	if lit, ok := call.Fun.(*ast.FuncLit); ok {
		return cctx.FuncLitCapturesContext(lit)
	}

	// For go fn()() (higher-order), check the factory function
	if innerCall, ok := call.Fun.(*ast.CallExpr); ok {
		return goStmtCheckHigherOrder(cctx, innerCall)
	}

	// For go fn(), try to find the function
	if ident, ok := call.Fun.(*ast.Ident); ok {
		_ = ident   // Would need to look up variable definition
		return true // Can't trace without SSA
	}

	return true // Can't analyze, assume OK
}

// goStmtCheckHigherOrder checks go fn()() patterns where fn is a factory function.
// For these patterns, we need to check if ctx is passed to the factory,
// or if the factory function returns a closure that uses ctx.
// Handles arbitrary depth: go fn()(), go fn()()(), etc.
func goStmtCheckHigherOrder(cctx *context.CheckContext, innerCall *ast.CallExpr) bool {
	// Check if ctx is passed as an argument to the inner call
	for _, arg := range innerCall.Args {
		if cctx.ArgUsesContext(arg) {
			return true
		}
	}

	// Check if the inner call's function is a func literal (factory IIFE)
	if lit, ok := innerCall.Fun.(*ast.FuncLit); ok {
		if cctx.FuncLitHasContextParam(lit) {
			return true
		}
		return cctx.FactoryReturnsContextUsingFunc(lit)
	}

	// Check if the inner call's function is an identifier
	if ident, ok := innerCall.Fun.(*ast.Ident); ok {
		return goStmtCheckIdentFactory(cctx, ident)
	}

	// Handle nested CallExpr for deeper chains like go fn()()()
	if nestedCall, ok := innerCall.Fun.(*ast.CallExpr); ok {
		return goStmtCheckHigherOrder(cctx, nestedCall)
	}

	return true // Can't analyze, assume OK
}

// goStmtCheckIdentFactory checks if an identifier refers to a factory that returns ctx-using func.
func goStmtCheckIdentFactory(cctx *context.CheckContext, ident *ast.Ident) bool {
	obj := cctx.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return true // Can't trace
	}

	// Handle local variable pointing to a func literal
	if v := cctx.VarFromIdent(ident); v != nil {
		funcLit := cctx.FindFuncLitAssignment(v, token.NoPos)
		if funcLit == nil {
			return true // Can't trace
		}
		// Skip if closure has its own context parameter
		if cctx.FuncLitHasContextParam(funcLit) {
			return true
		}
		// Check if the factory returns a context-using func
		return cctx.FactoryReturnsContextUsingFunc(funcLit)
	}

	// Handle package-level function declaration
	if fn, ok := obj.(*types.Func); ok {
		funcDecl := cctx.FindFuncDecl(fn)
		if funcDecl == nil {
			return true // Can't trace
		}
		// Skip if function has context parameter
		if cctx.FuncTypeHasContextParam(funcDecl.Type) {
			return true
		}
		// Check if the factory returns a context-using func
		return cctx.BlockReturnsContextUsingFunc(funcDecl.Body, nil)
	}

	return true // Can't analyze, assume OK
}

// GoStmtCallsDeriver checks that a go statement's closure calls a deriver function.
type GoStmtCallsDeriver struct {
	// Matcher is the deriver function matcher (OR/AND semantics).
	Matcher *deriver.Matcher
}

func (*GoStmtCallsDeriver) Name() string {
	return "GoStmtCallsDeriver"
}

func (p *GoStmtCallsDeriver) CheckGoStmt(cctx *context.CheckContext, stmt *ast.GoStmt) GoStmtResult {
	if p.Matcher == nil || p.Matcher.IsEmpty() {
		return GoStmtResult{OK: true} // No deriver configured
	}

	call := stmt.Call

	// For go func(){}(), check the function body
	if lit, ok := call.Fun.(*ast.FuncLit); ok {
		// Skip if closure has its own context parameter
		if cctx.FuncLitHasContextParam(lit) {
			return GoStmtResult{OK: true}
		}

		// Try SSA-based check first (detects IIFE, distinguishes defer)
		if result, ok := p.checkDeriverFromSSA(cctx, lit); ok {
			return result
		}

		// Fall back to AST-based check
		return GoStmtResult{OK: p.Matcher.SatisfiesAnyGroup(cctx.Pass, lit.Body)}
	}

	// For go fn()() (higher-order), check the factory function
	if innerCall, ok := call.Fun.(*ast.CallExpr); ok {
		return GoStmtResult{OK: p.checkHigherOrderDeriver(cctx, innerCall)}
	}

	// For go fn() where fn is an identifier, trace the variable
	if ident, ok := call.Fun.(*ast.Ident); ok {
		return GoStmtResult{OK: p.checkIdentDeriver(cctx, ident)}
	}

	return GoStmtResult{OK: true} // Can't analyze, assume OK
}

// checkDeriverFromSSA uses SSA analysis to check deriver calls.
// Returns (result, true) if SSA analysis succeeded, or (GoStmtResult{}, false) if it failed.
func (p *GoStmtCallsDeriver) checkDeriverFromSSA(cctx *context.CheckContext, lit *ast.FuncLit) (GoStmtResult, bool) {
	if cctx.SSAProg == nil || cctx.Tracer == nil {
		return GoStmtResult{}, false
	}

	ssaFn := cctx.SSAProg.FindFuncLit(lit)
	if ssaFn == nil {
		return GoStmtResult{}, false
	}

	result := cctx.Tracer.CheckDeriverCalls(ssaFn, p.Matcher)

	if result.FoundAtStart {
		return GoStmtResult{OK: true}, true
	}

	if result.FoundOnlyInDefer {
		return GoStmtResult{OK: false, DeferOnly: true}, true
	}

	return GoStmtResult{OK: false}, true
}

// checkIdentDeriver checks go fn() patterns where fn is a variable.
func (p *GoStmtCallsDeriver) checkIdentDeriver(cctx *context.CheckContext, ident *ast.Ident) bool {
	funcLit := cctx.FindIdentFuncLitAssignment(ident, token.NoPos)
	if funcLit == nil {
		return true // Can't trace
	}

	// Skip if closure has its own context parameter
	if cctx.FuncLitHasContextParam(funcLit) {
		return true
	}

	return p.Matcher.SatisfiesAnyGroup(cctx.Pass, funcLit.Body)
}

// checkHigherOrderDeriver checks go fn()() patterns for deriver calls.
// The deriver must be called in the returned function, not in the factory.
func (p *GoStmtCallsDeriver) checkHigherOrderDeriver(cctx *context.CheckContext, innerCall *ast.CallExpr) bool {
	// Check if the inner call's function is a func literal
	if lit, ok := innerCall.Fun.(*ast.FuncLit); ok {
		// Skip if closure has its own context parameter
		if cctx.FuncLitHasContextParam(lit) {
			return true
		}
		// Check if the factory's returns call the deriver
		return p.factoryReturnsDeriverCallingFunc(cctx, lit)
	}

	// Check if the inner call's function is a variable pointing to a func literal
	if ident, ok := innerCall.Fun.(*ast.Ident); ok {
		funcLit := cctx.FindIdentFuncLitAssignment(ident, token.NoPos)
		if funcLit == nil {
			return true // Can't trace
		}
		// Skip if closure has its own context parameter
		if cctx.FuncLitHasContextParam(funcLit) {
			return true
		}
		// Check if the factory's returns call the deriver
		return p.factoryReturnsDeriverCallingFunc(cctx, funcLit)
	}

	return true // Can't analyze, assume OK
}

// factoryReturnsDeriverCallingFunc checks if a factory function's return statements
// return functions that call the deriver.
func (p *GoStmtCallsDeriver) factoryReturnsDeriverCallingFunc(cctx *context.CheckContext, factory *ast.FuncLit) bool {
	callsDeriver := false

	ast.Inspect(factory.Body, func(n ast.Node) bool {
		if callsDeriver {
			return false
		}
		// Skip nested func literals (they have their own returns)
		if fl, ok := n.(*ast.FuncLit); ok && fl != factory {
			// Check if this nested func lit calls the deriver - it might be the returned value
			if p.Matcher.SatisfiesAnyGroup(cctx.Pass, fl.Body) {
				callsDeriver = true
				return false
			}
			return false // Don't descend into nested func literals
		}

		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		for _, result := range ret.Results {
			if p.returnedValueCallsDeriver(cctx, result) {
				callsDeriver = true
				return false
			}
		}
		return true
	})

	return callsDeriver
}

// returnedValueCallsDeriver checks if a returned value is a func that calls the deriver.
func (p *GoStmtCallsDeriver) returnedValueCallsDeriver(cctx *context.CheckContext, result ast.Expr) bool {
	// If it's a func literal, check directly
	if innerFuncLit, ok := result.(*ast.FuncLit); ok {
		return p.Matcher.SatisfiesAnyGroup(cctx.Pass, innerFuncLit.Body)
	}

	// If it's an identifier (variable), find its assignment
	ident, ok := result.(*ast.Ident)
	if !ok {
		return false
	}

	innerFuncLit := cctx.FindIdentFuncLitAssignment(ident, token.NoPos)
	if innerFuncLit == nil {
		return false
	}

	return p.Matcher.SatisfiesAnyGroup(cctx.Pass, innerFuncLit.Body)
}

func (p *GoStmtCallsDeriver) Message(_ string) string {
	return "goroutine should call " + p.Matcher.Original + " to derive context"
}

func (p *GoStmtCallsDeriver) DeferMessage(_ string) string {
	return "goroutine calls " + p.Matcher.Original + " in defer, but it should be called at goroutine start"
}
