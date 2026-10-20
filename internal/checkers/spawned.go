package checkers

import (
	"go/ast"
	"go/types"

	gossa "golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/funcspec"
	"github.com/mpyw/goroutinectx/internal/probe"
	"github.com/mpyw/goroutinectx/internal/ssa"
)

// spawnedFuncPasses judges the function a goroutine or a callback runs. expr
// is its AST expression, and v its SSA value, nil when SSA has none. The value
// is resolved through SSA to each function literal it can hold, and r judges
// them. A struct field or an element is followed as a part of the variable
// it is in. A literal SSA has no value for is found by its syntax.
//
//declscope:shared // goroutine.go, spawn.go and gotask.go judge spawned functions alike
func spawnedFuncPasses(cctx *probe.Context, expr ast.Expr, v gossa.Value, r *ssa.FuncResolver) bool {
	if lit, ok := ast.Unparen(expr).(*ast.FuncLit); ok && v == nil {
		fn := cctx.SSAProg.FindFuncLit(lit)
		return fn == nil || r.Closure(fn)
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
			return ssa.HasContextParam(fn, cctx.Carriers) || ssa.ClosureCapturesContext(fn, cctx.Carriers) ||
				(!derivers.IsEmpty() && ssa.ClosureCallsDeriver(fn, derivers).FoundAtStart)
		},
		FactoryCall: func(call *gossa.CallCommon) bool { return ssa.ArgsInvolveContext(call, cctx.Carriers) },
		Factory:     func(fn *gossa.Function) bool { return ssa.HasContextParam(fn, cctx.Carriers) },
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

// spawnedCallee gives the function call calls, and the number of arguments
// the call gives before the function's parameters: 1 for a method
// expression, as (*errgroup.Group).Go(g, fn), which takes the receiver
// first, and 0 otherwise. A call through a function or method value, as
// gofn := wg.Go, is found through SSA.
//
//declscope:shared // spawn.go and gotask.go find the API a call makes alike
func spawnedCallee(cctx *probe.Context, call *ast.CallExpr) (*types.Func, int) {
	if fn := funcspec.ExtractFunc(cctx.Pass, call); fn != nil {
		if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
			if s := cctx.Pass.TypesInfo.Selections[sel]; s != nil && s.Kind() == types.MethodExpr {
				return fn, 1
			}
		}
		return fn, 0
	}
	instr := cctx.SSAProg.CallAt(call)
	if instr == nil || instr.Common().IsInvoke() || instr.Common().StaticCallee() == nil {
		return nil, 0
	}
	return ssa.ExtractCalledFunc(instr.Common()), 0
}
