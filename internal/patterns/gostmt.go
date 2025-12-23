package patterns

import (
	"go/ast"
	"go/token"

	"github.com/mpyw/goroutinectx/internal/context"
	"github.com/mpyw/goroutinectx/internal/directives/deriver"
	"github.com/mpyw/goroutinectx/internal/directives/ignore"
)

// GoStmtCapturesCtx checks that a go statement's closure captures the outer context.
type GoStmtCapturesCtx struct{}

func (*GoStmtCapturesCtx) Name() string {
	return "GoStmtCapturesCtx"
}

func (*GoStmtCapturesCtx) CheckerName() ignore.CheckerName {
	return ignore.Goroutine
}

func (p *GoStmtCapturesCtx) Check(cctx *context.CheckContext, stmt *ast.GoStmt) GoStmtResult {
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
	return GoStmtResult{OK: p.checkFromAST(cctx, stmt)}
}

func (*GoStmtCapturesCtx) Message(ctxName string) string {
	return "goroutine does not propagate context \"" + ctxName + "\""
}

func (*GoStmtCapturesCtx) DeferMessage(_ string) string {
	return "" // Not applicable for context capture pattern
}

// checkFromAST falls back to AST-based analysis for go statements.
func (*GoStmtCapturesCtx) checkFromAST(cctx *context.CheckContext, stmt *ast.GoStmt) bool {
	call := stmt.Call

	// For go func(){}(), check the function literal
	if lit, ok := call.Fun.(*ast.FuncLit); ok {
		return cctx.FuncLitCapturesContext(lit)
	}

	// For go fn()() (higher-order), check the factory function
	if innerCall, ok := call.Fun.(*ast.CallExpr); ok {
		return cctx.FactoryCallReturnsContextUsingFunc(innerCall)
	}

	// For go fn(), try to find the function literal assignment
	if ident, ok := call.Fun.(*ast.Ident); ok {
		funcLit := cctx.FuncLitOfIdent(ident, token.NoPos)
		if funcLit == nil {
			return true // Can't trace, assume OK
		}
		return cctx.FuncLitCapturesContext(funcLit)
	}

	// For go s.handler(), check the struct field func
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		return cctx.SelectorExprCapturesContext(sel)
	}

	// For go handlers[0](), check the indexed func
	if idx, ok := call.Fun.(*ast.IndexExpr); ok {
		return cctx.IndexExprCapturesContext(idx)
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

func (*GoStmtCallsDeriver) CheckerName() ignore.CheckerName {
	return ignore.GoroutineDerive
}

func (p *GoStmtCallsDeriver) Check(cctx *context.CheckContext, stmt *ast.GoStmt) GoStmtResult {
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
		if result, ok := p.checkFromSSA(cctx, lit); ok {
			return result
		}

		// Fall back to AST-based check
		return GoStmtResult{OK: p.Matcher.SatisfiesAnyGroup(cctx.Pass, lit.Body)}
	}

	// For go fn()() (higher-order), check the factory function
	if innerCall, ok := call.Fun.(*ast.CallExpr); ok {
		return GoStmtResult{OK: p.checkHigherOrder(cctx, innerCall)}
	}

	// For go fn() where fn is an identifier, trace the variable
	if ident, ok := call.Fun.(*ast.Ident); ok {
		return GoStmtResult{OK: p.checkIdent(cctx, ident)}
	}

	return GoStmtResult{OK: true} // Can't analyze, assume OK
}

// checkFromSSA uses SSA analysis to check deriver calls.
// Returns (result, true) if SSA analysis succeeded, or (GoStmtResult{}, false) if it failed.
func (p *GoStmtCallsDeriver) checkFromSSA(cctx *context.CheckContext, lit *ast.FuncLit) (GoStmtResult, bool) {
	if cctx.SSAProg == nil || cctx.Tracer == nil {
		return GoStmtResult{}, false
	}

	ssaFn := cctx.SSAProg.FindFuncLit(lit)
	if ssaFn == nil {
		return GoStmtResult{}, false
	}

	result := cctx.Tracer.ClosureCallsDeriver(ssaFn, p.Matcher)

	if result.FoundAtStart {
		return GoStmtResult{OK: true}, true
	}

	if result.FoundOnlyInDefer {
		return GoStmtResult{OK: false, DeferOnly: true}, true
	}

	return GoStmtResult{OK: false}, true
}

// checkIdent checks go fn() patterns where fn is a variable.
func (p *GoStmtCallsDeriver) checkIdent(cctx *context.CheckContext, ident *ast.Ident) bool {
	funcLit := cctx.FuncLitOfIdent(ident, token.NoPos)
	if funcLit == nil {
		return true // Can't trace
	}

	// Skip if closure has its own context parameter
	if cctx.FuncLitHasContextParam(funcLit) {
		return true
	}

	return p.Matcher.SatisfiesAnyGroup(cctx.Pass, funcLit.Body)
}

// checkHigherOrder checks go fn()() patterns for deriver calls.
// The deriver must be called in the returned function, not in the factory.
func (p *GoStmtCallsDeriver) checkHigherOrder(cctx *context.CheckContext, innerCall *ast.CallExpr) bool {
	// Unwrap ParenExpr if present: (func() func() { ... })()
	fun := innerCall.Fun
	if paren, ok := fun.(*ast.ParenExpr); ok {
		fun = paren.X
	}

	// Check if the inner call's function is a func literal
	if lit, ok := fun.(*ast.FuncLit); ok {
		// Skip if closure has its own context parameter
		if cctx.FuncLitHasContextParam(lit) {
			return true
		}
		// Check if the factory's returns call the deriver
		return p.factoryReturnsCallingFunc(cctx, lit)
	}

	// Check if the inner call's function is a variable pointing to a func literal
	if ident, ok := fun.(*ast.Ident); ok {
		funcLit := cctx.FuncLitOfIdent(ident, token.NoPos)
		if funcLit == nil {
			return true // Can't trace
		}
		// Skip if closure has its own context parameter
		if cctx.FuncLitHasContextParam(funcLit) {
			return true
		}
		// Check if the factory's returns call the deriver
		return p.factoryReturnsCallingFunc(cctx, funcLit)
	}

	return true // Can't analyze, assume OK
}

// factoryReturnsCallingFunc checks if a factory function's return statements
// return functions that call the deriver.
func (p *GoStmtCallsDeriver) factoryReturnsCallingFunc(cctx *context.CheckContext, factory *ast.FuncLit) bool {
	callsDeriver := false

	ast.Inspect(factory.Body, func(n ast.Node) bool {
		if callsDeriver {
			return false
		}
		// Skip nested func literals (they have their own returns)
		if fl, ok := n.(*ast.FuncLit); ok && fl != factory {
			// Skip if nested func has its own context parameter
			if cctx.FuncLitHasContextParam(fl) {
				callsDeriver = true // Has own ctx, no need to check for deriver
				return false
			}
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
			if p.returnedValueCalls(cctx, result) {
				callsDeriver = true
				return false
			}
		}
		return true
	})

	return callsDeriver
}

// returnedValueCalls checks if a returned value is a func that calls the deriver.
func (p *GoStmtCallsDeriver) returnedValueCalls(cctx *context.CheckContext, result ast.Expr) bool {
	// If it's a func literal, check directly
	if innerFuncLit, ok := result.(*ast.FuncLit); ok {
		// Skip if func has its own context parameter
		if cctx.FuncLitHasContextParam(innerFuncLit) {
			return true
		}
		return p.Matcher.SatisfiesAnyGroup(cctx.Pass, innerFuncLit.Body)
	}

	// If it's an identifier (variable), find its assignment
	ident, ok := result.(*ast.Ident)
	if !ok {
		return false
	}

	innerFuncLit := cctx.FuncLitOfIdent(ident, token.NoPos)
	if innerFuncLit == nil {
		return false
	}

	// Skip if func has its own context parameter
	if cctx.FuncLitHasContextParam(innerFuncLit) {
		return true
	}
	return p.Matcher.SatisfiesAnyGroup(cctx.Pass, innerFuncLit.Body)
}

func (p *GoStmtCallsDeriver) Message(_ string) string {
	return "goroutine should call " + p.Matcher.Original + " to derive context"
}

func (p *GoStmtCallsDeriver) DeferMessage(_ string) string {
	return "goroutine calls " + p.Matcher.Original + " in defer, but it should be called at goroutine start"
}
