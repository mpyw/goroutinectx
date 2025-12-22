package patterns

import (
	"fmt"
	"go/ast"
)

// CallbackReceivesCtx checks APIs where the callback receives context as its first parameter.
// Used by: gotask.DoAllFnsSettled, ants.PoolWithFuncGeneric[context.Context], etc.
//
// For these APIs, the callback signature includes context.Context as first param,
// so we only need to verify that the context passed to the API call is properly propagated.
// The callback itself doesn't need to capture context - it receives it from the API.
type CallbackReceivesCtx struct {
	// CtxArgIdx is the index of the context argument in the API call (0-based).
	// This is the context that will be passed to callbacks.
	CtxArgIdx int
}

func (*CallbackReceivesCtx) Name() string {
	return "CallbackReceivesCtx"
}

func (p *CallbackReceivesCtx) Check(cctx *CheckContext, call *ast.CallExpr, _ ast.Expr) bool {
	// Get the context argument from the API call
	if p.CtxArgIdx < 0 || p.CtxArgIdx >= len(call.Args) {
		return true // Invalid index, assume OK
	}

	ctxArg := call.Args[p.CtxArgIdx]

	// Check if the context argument is a valid context.Context value
	// This is AST-based and doesn't need SSA
	return p.contextArgUsesVar(cctx, ctxArg)
}

// contextArgUsesVar checks if the context argument references a valid context variable.
func (*CallbackReceivesCtx) contextArgUsesVar(cctx *CheckContext, ctxArg ast.Expr) bool {
	// For simple identifier, check if it's a context type from scope
	if ident, ok := ctxArg.(*ast.Ident); ok {
		obj := cctx.Pass.TypesInfo.ObjectOf(ident)
		if obj != nil && isContextType(obj.Type()) {
			return true
		}
	}

	// For call expressions (e.g., context.WithCancel(ctx)), check the result type
	if callExpr, ok := ctxArg.(*ast.CallExpr); ok {
		typ := cctx.Pass.TypesInfo.TypeOf(callExpr)
		if typ != nil && isContextType(typ) {
			return true
		}
	}

	// For selector expressions (e.g., req.Context())
	if sel, ok := ctxArg.(*ast.SelectorExpr); ok {
		typ := cctx.Pass.TypesInfo.TypeOf(sel)
		if typ != nil && isContextType(typ) {
			return true
		}
	}

	return false
}

func (*CallbackReceivesCtx) Message(apiName string, ctxName string) string {
	return fmt.Sprintf("%s: context argument should use %s", apiName, ctxName)
}
