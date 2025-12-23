package patterns

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/mpyw/goroutinectx/internal/context"
	"github.com/mpyw/goroutinectx/internal/directives/ignore"
)

// ClosureCapturesCtx checks that a closure captures the outer context.
// Used by: errgroup.Group.Go, sync.WaitGroup.Go, sourcegraph/conc.Pool.Go, etc.
type ClosureCapturesCtx struct {
	// checkerName is the ignore checker name for this pattern instance.
	checkerName ignore.CheckerName
}

// NewClosureCapturesCtx creates a new ClosureCapturesCtx with the given checker name.
func NewClosureCapturesCtx(checkerName ignore.CheckerName) *ClosureCapturesCtx {
	return &ClosureCapturesCtx{checkerName: checkerName}
}

func (*ClosureCapturesCtx) Name() string {
	return "ClosureCapturesCtx"
}

func (p *ClosureCapturesCtx) CheckerName() ignore.CheckerName {
	return p.checkerName
}

func (p *ClosureCapturesCtx) Check(cctx *context.CheckContext, arg ast.Expr, _ *TaskConstructorConfig) bool {
	// If no context names in scope (from AST), nothing to check
	if len(cctx.CtxNames) == 0 {
		return true
	}

	// Try SSA-based check first (more accurate, includes nested closures)
	if lit, ok := arg.(*ast.FuncLit); ok {
		if result, ok := cctx.FuncLitCapturesContextSSA(lit); ok {
			return result
		}
	}

	// Fall back to AST-based check when SSA fails
	return p.checkFromAST(cctx, arg)
}

func (*ClosureCapturesCtx) Message(apiName string, ctxName string) string {
	return fmt.Sprintf("%s() closure should use context %q", apiName, ctxName)
}

// checkFromAST falls back to AST-based analysis when SSA tracing fails.
// Design principle: "zero false positives" - if we can't trace, assume OK.
func (*ClosureCapturesCtx) checkFromAST(cctx *context.CheckContext, callbackArg ast.Expr) bool {
	// For function literals, check if they reference context
	if lit, ok := callbackArg.(*ast.FuncLit); ok {
		return cctx.FuncLitCapturesContext(lit)
	}

	// For identifiers, try to find the function literal assignment
	if ident, ok := callbackArg.(*ast.Ident); ok {
		funcLit := cctx.FuncLitOfIdent(ident, token.NoPos)
		if funcLit == nil {
			return true // Can't trace, assume OK
		}
		return cctx.FuncLitCapturesContext(funcLit)
	}

	// For call expressions, check if ctx is passed as argument
	if call, ok := callbackArg.(*ast.CallExpr); ok {
		return cctx.FactoryCallReturnsContextUsingFunc(call)
	}

	// For selector expressions (struct field access), check the field's func
	if sel, ok := callbackArg.(*ast.SelectorExpr); ok {
		return cctx.SelectorExprCapturesContext(sel)
	}

	// For index expressions (slice/map access), check the indexed func
	if idx, ok := callbackArg.(*ast.IndexExpr); ok {
		return cctx.IndexExprCapturesContext(idx)
	}

	return true // Can't analyze, assume OK
}
