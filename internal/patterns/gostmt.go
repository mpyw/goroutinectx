package patterns

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/mpyw/goroutinectx/internal/directives/deriver"
)

// GoStmtCapturesCtx checks that a go statement's closure captures the outer context.
type GoStmtCapturesCtx struct{}

func (*GoStmtCapturesCtx) Name() string {
	return "GoStmtCapturesCtx"
}

func (*GoStmtCapturesCtx) CheckGoStmt(cctx *CheckContext, stmt *ast.GoStmt) GoStmtResult {
	// If no context names in scope (from AST), nothing to check
	if len(cctx.CtxNames) == 0 {
		return GoStmtResult{OK: true}
	}

	// Try SSA-based check first (more accurate, includes nested closures)
	if result, ok := goStmtCheckFromSSA(cctx, stmt); ok {
		return GoStmtResult{OK: result}
	}

	// Fall back to AST-based check when SSA fails
	return GoStmtResult{OK: goStmtCheckFromAST(cctx, stmt)}
}

// goStmtCheckFromSSA uses SSA analysis to check if a goroutine captures context.
// Returns (result, true) if SSA analysis succeeded, or (false, false) if it failed.
func goStmtCheckFromSSA(cctx *CheckContext, stmt *ast.GoStmt) (bool, bool) {
	if cctx.SSAProg == nil || cctx.Tracer == nil {
		return false, false
	}

	call := stmt.Call

	// For go func(){}(), find the SSA function and check FreeVars
	if lit, ok := call.Fun.(*ast.FuncLit); ok {
		// Skip if closure has its own context parameter
		if cctx.funcLitHasContextParam(lit) {
			return true, true
		}

		ssaFn := cctx.SSAProg.FindFuncLit(lit)
		if ssaFn == nil {
			return false, false // SSA lookup failed
		}

		return cctx.Tracer.ClosureCapturesContext(ssaFn, cctx.Carriers), true
	}

	// For other cases (go fn()(), go fn()), fall back to AST
	return false, false
}

func (*GoStmtCapturesCtx) Message(ctxName string) string {
	return "goroutine does not propagate context \"" + ctxName + "\""
}

func (*GoStmtCapturesCtx) DeferMessage(_ string) string {
	return "" // Not applicable for context capture pattern
}

// goStmtCheckFromAST falls back to AST-based analysis for go statements.
func goStmtCheckFromAST(cctx *CheckContext, stmt *ast.GoStmt) bool {
	call := stmt.Call

	// For go func(){}(), check the function literal
	if lit, ok := call.Fun.(*ast.FuncLit); ok {
		// Skip if closure has its own context parameter
		if cctx.funcLitHasContextParam(lit) {
			return true
		}
		return cctx.FuncLitUsesContext(lit)
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
func goStmtCheckHigherOrder(cctx *CheckContext, innerCall *ast.CallExpr) bool {
	// Check if ctx is passed as an argument to the inner call
	for _, arg := range innerCall.Args {
		if cctx.argUsesContext(arg) {
			return true
		}
	}

	// Check if the inner call's function is a func literal
	if lit, ok := innerCall.Fun.(*ast.FuncLit); ok {
		// Skip if closure has its own context parameter
		if cctx.funcLitHasContextParam(lit) {
			return true
		}
		// Check if the factory returns a context-using func
		return cctx.factoryReturnsContextUsingFunc(lit)
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
func goStmtCheckIdentFactory(cctx *CheckContext, ident *ast.Ident) bool {
	obj := cctx.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return true // Can't trace
	}

	// Handle local variable pointing to a func literal
	if v, ok := obj.(*types.Var); ok {
		funcLit := cctx.FindFuncLitAssignment(v, token.NoPos)
		if funcLit == nil {
			return true // Can't trace
		}
		// Skip if closure has its own context parameter
		if cctx.funcLitHasContextParam(funcLit) {
			return true
		}
		// Check if the factory returns a context-using func
		return cctx.factoryReturnsContextUsingFunc(funcLit)
	}

	// Handle package-level function declaration
	if fn, ok := obj.(*types.Func); ok {
		funcDecl := goStmtFindFuncDecl(cctx, fn)
		if funcDecl == nil {
			return true // Can't trace
		}
		// Skip if function has context parameter
		if goStmtFuncDeclHasContextParam(cctx, funcDecl) {
			return true
		}
		// Check if the factory returns a context-using func
		return goStmtFactoryDeclReturnsCtxFunc(cctx, funcDecl)
	}

	return true // Can't analyze, assume OK
}

// goStmtFindFuncDecl finds the FuncDecl for a types.Func.
func goStmtFindFuncDecl(cctx *CheckContext, fn *types.Func) *ast.FuncDecl {
	pos := fn.Pos()
	for _, f := range cctx.Pass.Files {
		if f.Pos() > pos || pos >= f.End() {
			continue
		}
		for _, decl := range f.Decls {
			if funcDecl, ok := decl.(*ast.FuncDecl); ok {
				if funcDecl.Name.Pos() == pos {
					return funcDecl
				}
			}
		}
	}
	return nil
}

// goStmtFuncDeclHasContextParam checks if a function declaration has a context.Context parameter.
func goStmtFuncDeclHasContextParam(cctx *CheckContext, decl *ast.FuncDecl) bool {
	return cctx.funcTypeHasContextParam(decl.Type)
}

// goStmtFactoryDeclReturnsCtxFunc checks if a function declaration returns funcs that use context.
func goStmtFactoryDeclReturnsCtxFunc(cctx *CheckContext, decl *ast.FuncDecl) bool {
	return cctx.blockReturnsContextUsingFunc(decl.Body, nil)
}

// GoStmtCallsDeriver checks that a go statement's closure calls a deriver function.
type GoStmtCallsDeriver struct {
	// Matcher is the deriver function matcher (OR/AND semantics).
	Matcher *deriver.Matcher
}

func (*GoStmtCallsDeriver) Name() string {
	return "GoStmtCallsDeriver"
}

func (p *GoStmtCallsDeriver) CheckGoStmt(cctx *CheckContext, stmt *ast.GoStmt) GoStmtResult {
	if p.Matcher == nil || p.Matcher.IsEmpty() {
		return GoStmtResult{OK: true} // No deriver configured
	}

	call := stmt.Call

	// For go func(){}(), check the function body
	if lit, ok := call.Fun.(*ast.FuncLit); ok {
		// Skip if closure has its own context parameter
		if cctx.funcLitHasContextParam(lit) {
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
func (p *GoStmtCallsDeriver) checkDeriverFromSSA(cctx *CheckContext, lit *ast.FuncLit) (GoStmtResult, bool) {
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
func (p *GoStmtCallsDeriver) checkIdentDeriver(cctx *CheckContext, ident *ast.Ident) bool {
	obj := cctx.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return true // Can't trace
	}

	v, ok := obj.(*types.Var)
	if !ok {
		return true // Not a variable
	}

	funcLit := cctx.FindFuncLitAssignment(v, token.NoPos)
	if funcLit == nil {
		return true // Can't trace
	}

	// Skip if closure has its own context parameter
	if cctx.funcLitHasContextParam(funcLit) {
		return true
	}

	return p.Matcher.SatisfiesAnyGroup(cctx.Pass, funcLit.Body)
}

// checkHigherOrderDeriver checks go fn()() patterns for deriver calls.
// The deriver must be called in the returned function, not in the factory.
func (p *GoStmtCallsDeriver) checkHigherOrderDeriver(cctx *CheckContext, innerCall *ast.CallExpr) bool {
	// Check if the inner call's function is a func literal
	if lit, ok := innerCall.Fun.(*ast.FuncLit); ok {
		// Skip if closure has its own context parameter
		if cctx.funcLitHasContextParam(lit) {
			return true
		}
		// Check if the factory's returns call the deriver
		return p.factoryReturnsDeriverCallingFunc(cctx, lit)
	}

	// Check if the inner call's function is a variable pointing to a func literal
	if ident, ok := innerCall.Fun.(*ast.Ident); ok {
		obj := cctx.Pass.TypesInfo.ObjectOf(ident)
		if obj == nil {
			return true // Can't trace
		}
		v, ok := obj.(*types.Var)
		if !ok {
			return true // Not a variable (could be a function)
		}
		funcLit := cctx.FindFuncLitAssignment(v, token.NoPos)
		if funcLit == nil {
			return true // Can't trace
		}
		// Skip if closure has its own context parameter
		if cctx.funcLitHasContextParam(funcLit) {
			return true
		}
		// Check if the factory's returns call the deriver
		return p.factoryReturnsDeriverCallingFunc(cctx, funcLit)
	}

	return true // Can't analyze, assume OK
}

// factoryReturnsDeriverCallingFunc checks if a factory function's return statements
// return functions that call the deriver.
func (p *GoStmtCallsDeriver) factoryReturnsDeriverCallingFunc(cctx *CheckContext, factory *ast.FuncLit) bool {
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
func (p *GoStmtCallsDeriver) returnedValueCallsDeriver(cctx *CheckContext, result ast.Expr) bool {
	// If it's a func literal, check directly
	if innerFuncLit, ok := result.(*ast.FuncLit); ok {
		return p.Matcher.SatisfiesAnyGroup(cctx.Pass, innerFuncLit.Body)
	}

	// If it's an identifier (variable), find its assignment
	ident, ok := result.(*ast.Ident)
	if !ok {
		return false
	}

	obj := cctx.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return false
	}

	v, ok := obj.(*types.Var)
	if !ok {
		return false
	}

	innerFuncLit := cctx.FindFuncLitAssignment(v, token.NoPos)
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
