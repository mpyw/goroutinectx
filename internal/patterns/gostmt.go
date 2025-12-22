package patterns

import (
	"go/ast"

	"github.com/mpyw/goroutinectx/internal/directives/deriver"
	internalssa "github.com/mpyw/goroutinectx/internal/ssa"
)

// GoStmtPattern defines the interface for go statement patterns.
type GoStmtPattern interface {
	// Name returns a human-readable name for the pattern.
	Name() string

	// CheckGoStmt checks if the pattern is satisfied for the given go statement.
	// Returns true if the pattern is satisfied (no error).
	CheckGoStmt(cctx *CheckContext, stmt *ast.GoStmt) bool

	// Message returns the diagnostic message when the pattern is violated.
	Message(ctxName string) string
}

// GoStmtCapturesCtx checks that a go statement's closure captures the outer context.
type GoStmtCapturesCtx struct{}

func (*GoStmtCapturesCtx) Name() string {
	return "GoStmtCapturesCtx"
}

func (*GoStmtCapturesCtx) CheckGoStmt(cctx *CheckContext, stmt *ast.GoStmt) bool {
	// Always try AST fallback first if SSA is not available
	if cctx.SSAProg == nil {
		return checkGoStmtFromAST(cctx, stmt)
	}

	// Get SSA function containing this go statement
	ssaFn := cctx.SSAProg.EnclosingFunc(stmt)
	if ssaFn == nil {
		return checkGoStmtFromAST(cctx, stmt)
	}

	// Get context variables from the enclosing function
	ctxVars := internalssa.GetContextVars(ssaFn)
	if len(ctxVars) == 0 {
		return true // No context in scope, nothing to check
	}

	// Extract the function being called in the go statement
	callExpr := extractGoCallExpr(stmt)
	if callExpr == nil {
		return checkGoStmtFromAST(cctx, stmt)
	}

	// Get the function expression
	fnExpr := extractFnFromCall(callExpr)
	if fnExpr == nil {
		return checkGoStmtFromAST(cctx, stmt)
	}

	// Find the SSA value for the function
	ssaValue := cctx.findSSAValue(ssaFn, fnExpr)
	if ssaValue == nil {
		// Try AST-based check
		return checkGoStmtFromAST(cctx, stmt)
	}

	// Find the closure
	closure := cctx.Tracer.FindClosure(ssaValue)
	if closure == nil {
		return checkGoStmtFromAST(cctx, stmt)
	}

	// Check if closure uses context via SSA
	if cctx.Tracer.ClosureUsesContext(closure, ctxVars) {
		return true
	}

	// SSA FreeVars check failed - double-check with AST
	return checkGoStmtFromAST(cctx, stmt)
}

func (*GoStmtCapturesCtx) Message(ctxName string) string {
	return "goroutine does not propagate context \"" + ctxName + "\""
}

// extractGoCallExpr extracts the call expression from a go statement.
// go func(){}() -> the outer CallExpr
// go fn() -> the CallExpr
func extractGoCallExpr(stmt *ast.GoStmt) *ast.CallExpr {
	return stmt.Call
}

// extractFnFromCall extracts the function expression from a call.
// For func(){}() -> the FuncLit
// For fn() -> the Ident
// For fn()() -> recurse into inner call's Fun
func extractFnFromCall(call *ast.CallExpr) ast.Expr {
	switch fn := call.Fun.(type) {
	case *ast.FuncLit:
		return fn
	case *ast.Ident:
		return fn
	case *ast.CallExpr:
		// go fn()() - higher-order function
		return fn.Fun
	case *ast.SelectorExpr:
		return fn
	}
	return call.Fun
}

// checkGoStmtFromAST falls back to AST-based analysis for go statements.
func checkGoStmtFromAST(cctx *CheckContext, stmt *ast.GoStmt) bool {
	call := stmt.Call

	// For go func(){}(), check the function literal
	if lit, ok := call.Fun.(*ast.FuncLit); ok {
		return funcLitUsesContext(cctx, lit)
	}

	// For go fn()() (higher-order), check if inner call is a func literal
	if innerCall, ok := call.Fun.(*ast.CallExpr); ok {
		if lit, ok := innerCall.Fun.(*ast.FuncLit); ok {
			return funcLitUsesContext(cctx, lit)
		}
	}

	// For go fn(), try to find the function
	if ident, ok := call.Fun.(*ast.Ident); ok {
		_ = ident   // Would need to look up variable definition
		return true // Can't trace without SSA
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

func (p *GoStmtCallsDeriver) CheckGoStmt(cctx *CheckContext, stmt *ast.GoStmt) bool {
	if p.Matcher == nil || p.Matcher.IsEmpty() {
		return true // No deriver configured
	}

	call := stmt.Call

	// For go func(){}(), check the function body
	if lit, ok := call.Fun.(*ast.FuncLit); ok {
		return p.Matcher.SatisfiesAnyGroup(cctx.Pass, lit.Body)
	}

	// For go fn()() (higher-order), check the inner function
	if innerCall, ok := call.Fun.(*ast.CallExpr); ok {
		if lit, ok := innerCall.Fun.(*ast.FuncLit); ok {
			return p.Matcher.SatisfiesAnyGroup(cctx.Pass, lit.Body)
		}
	}

	return true // Can't analyze, assume OK
}

func (p *GoStmtCallsDeriver) Message(_ string) string {
	return "goroutine should call " + p.Matcher.Original + " to derive context"
}
