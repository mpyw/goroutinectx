package patterns

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/ssa"

	internalssa "github.com/mpyw/goroutinectx/internal/ssa"
)

// ClosureCapturesCtx checks that a closure captures the outer context.
// Used by: errgroup.Group.Go, sync.WaitGroup.Go, sourcegraph/conc.Pool.Go, etc.
type ClosureCapturesCtx struct{}

func (*ClosureCapturesCtx) Name() string {
	return "ClosureCapturesCtx"
}

func (*ClosureCapturesCtx) Check(cctx *CheckContext, call *ast.CallExpr, callbackArg ast.Expr) bool {
	// Always try AST fallback first if SSA is not available
	if cctx.SSAProg == nil {
		return checkClosureFromAST(cctx, callbackArg)
	}

	// Get SSA function containing this call
	ssaFn := cctx.SSAProg.EnclosingFunc(call)
	if ssaFn == nil {
		return checkClosureFromAST(cctx, callbackArg)
	}

	// Get context variables from the enclosing function
	ctxVars := internalssa.GetContextVars(ssaFn)
	if len(ctxVars) == 0 {
		return true // No context in scope, nothing to check
	}

	// Find the SSA value for the callback argument
	ssaValue := cctx.findSSAValue(ssaFn, callbackArg)
	if ssaValue == nil {
		// Try to find closure from AST if SSA tracing fails
		return checkClosureFromAST(cctx, callbackArg)
	}

	// Find the closure
	closure := cctx.Tracer.FindClosure(ssaValue)
	if closure == nil {
		// Can't find closure - check if context was passed to factory call
		if ssaCall, ok := ssaValue.(*ssa.Call); ok {
			return cctx.Tracer.CallUsesContext(ssaCall, ctxVars)
		}
		// Fall back to AST check
		return checkClosureFromAST(cctx, callbackArg)
	}

	// Check if closure uses context via SSA
	if cctx.Tracer.ClosureUsesContext(closure, ctxVars) {
		return true
	}

	// SSA FreeVars check failed - double-check with AST
	// This handles cases where SSA doesn't capture all context usage patterns
	return checkClosureFromAST(cctx, callbackArg)
}

func (*ClosureCapturesCtx) Message(apiName string, ctxName string) string {
	return fmt.Sprintf("%s() closure should use context %q", apiName, ctxName)
}

// checkClosureFromAST falls back to AST-based analysis when SSA tracing fails.
func checkClosureFromAST(cctx *CheckContext, callbackArg ast.Expr) bool {
	// For function literals, check if they reference context
	if lit, ok := callbackArg.(*ast.FuncLit); ok {
		return funcLitUsesContext(cctx, lit)
	}

	// For identifiers, try to find the function literal assignment
	if ident, ok := callbackArg.(*ast.Ident); ok {
		if obj := cctx.Pass.TypesInfo.ObjectOf(ident); obj != nil {
			// Can't trace variable assignment without SSA
			return true
		}
	}

	return true // Can't analyze, assume OK
}

// funcLitUsesContext checks if a function literal references any context variable.
func funcLitUsesContext(cctx *CheckContext, lit *ast.FuncLit) bool {
	usesCtx := false
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		if usesCtx {
			return false
		}
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		obj := cctx.Pass.TypesInfo.ObjectOf(ident)
		if obj == nil {
			return true
		}
		if isContextType(obj.Type()) {
			usesCtx = true
			return false
		}
		return true
	})
	return usesCtx
}
