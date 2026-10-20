package checkers

import (
	"go/ast"

	gossa "golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/probe"
	"github.com/mpyw/goroutinectx/internal/ssa"
)

// spawnedFuncPasses judges the function a goroutine or a callback runs. expr
// is its AST expression, and v its SSA value, nil when SSA has none. The value
// is resolved through SSA to each function literal it can hold, and r judges
// them. A struct field or an indexed element is looked up on the AST instead,
// when fieldLookup is set; otherwise it passes.
//
//declscope:shared // goroutine.go, spawn.go and gotask.go judge spawned functions alike
func spawnedFuncPasses(cctx *probe.Context, expr ast.Expr, v gossa.Value, r *ssa.FuncResolver, fieldLookup bool) bool {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		// LIMITATION: a parenthesized function is not traced.
		return true
	case *ast.SelectorExpr:
		return !fieldLookup || cctx.SelectorExprCapturesContext(e)
	case *ast.IndexExpr:
		return !fieldLookup || cctx.IndexExprCapturesContext(e)
	case *ast.FuncLit:
		if v == nil {
			fn := cctx.SSAProg.FindFuncLit(e)
			return fn == nil || r.Closure(fn)
		}
	}
	if v == nil {
		return true
	}
	return r.Passes(v)
}

// spawnedContextResolver judges a closure by whether it takes or captures a
// context, or, when derivers is not empty, calls the deriver at its start. A
// factory passes when it takes a context, or is given one.
//
//declscope:shared // goroutine.go and spawn.go check for a context alike
func spawnedContextResolver(cctx *probe.Context, derivers *deriver.Matcher) *ssa.FuncResolver {
	return &ssa.FuncResolver{
		Closure: func(fn *gossa.Function) bool {
			return ssa.HasContextParam(fn) || ssa.ClosureCapturesContext(fn, cctx.Carriers) ||
				(!derivers.IsEmpty() && ssa.ClosureCallsDeriver(fn, derivers).FoundAtStart)
		},
		FactoryCall: func(call *gossa.CallCommon) bool { return ssa.ArgsInvolveContext(call, cctx.Carriers) },
		Factory:     ssa.HasContextParam,
	}
}

// spawnedCallArgs gives the SSA value of each argument of call, or nil.
//
//declscope:shared // spawn.go and gotask.go read the arguments of a call
func spawnedCallArgs(cctx *probe.Context, call *ast.CallExpr) []gossa.Value {
	instr := cctx.SSAProg.CallAt(call)
	if instr == nil {
		return nil
	}
	return ssa.ResolvedArgs(instr.Common(), call)
}
