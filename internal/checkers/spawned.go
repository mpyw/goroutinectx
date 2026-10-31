package checkers

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/ctxkind"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/env"
	"github.com/mpyw/goroutinectx/internal/flow"
	"github.com/mpyw/goroutinectx/internal/funcspec"
)

// spawnedFuncPasses judges the function a goroutine or a callback runs. expr
// is its AST expression, and v its SSA value, nil when SSA has none. The value
// is resolved through SSA to each function literal it can hold, and r judges
// them. A struct field or an element is followed as a part of the variable
// it is in. A literal SSA has no value for is found by its syntax.
//
//declscope:shared // goroutine.go, spawn.go and gotask.go judge spawned functions alike
func spawnedFuncPasses(e *env.Env, expr ast.Expr, v ssa.Value, r *flow.FuncResolver) bool {
	if lit, ok := ast.Unparen(expr).(*ast.FuncLit); ok && v == nil {
		fn := e.SSA.FindFuncLit(lit)
		return fn == nil || r.Closure(fn)
	}
	if v == nil {
		return true
	}
	return r.Passes(v)
}

// spawnedContextResolver judges a closure by whether it takes or captures a
// context, or, when derivers is not empty, calls the deriver at its start. A
// factory passes when it takes a context, or is given one other than a fresh
// context.Background() or TODO(). A named function given only a fresh context
// fails, as it has no other way to get ctx.
//
//declscope:shared // goroutine.go and spawn.go check for a context alike
func spawnedContextResolver(e *env.Env, derivers *deriver.Matcher) *flow.FuncResolver {
	return &flow.FuncResolver{
		Closure: func(fn *ssa.Function) bool {
			return e.Kinds.HasParam(fn, ctxkind.Param) || flow.ClosureCapturesContext(fn, e.Kinds) ||
				(!derivers.IsEmpty() && flow.ClosureCallsDeriver(fn, derivers).FoundAtStart)
		},
		Call: func(call *ssa.CallCommon) (passes, handled bool) {
			return false, spawnedGivesNamedFuncOnlyFreshContext(e, call)
		},
		FactoryCall: func(call *ssa.CallCommon) bool {
			return e.Kinds.Involves(ctxkind.Param, call.Args...) && !flow.GivesOnlyFreshContext(call, e.Kinds)
		},
		Factory: func(fn *ssa.Function) bool { return e.Kinds.HasParam(fn, ctxkind.Param) },
	}
}

// spawnedGivesNamedFuncOnlyFreshContext reports whether call calls a named
// function, not a method, generic or not, and gives it a fresh
// context.Background() or TODO() and nothing else that may hold a context.
// A named function is not looked into, but one given only a fresh context
// does not get ctx. A method is left alone, as
// its receiver may hold a context, and so is a closure, which may capture
// one.
//
//declscope:shared // goroutine.go judges a named function a go statement calls alike
func spawnedGivesNamedFuncOnlyFreshContext(e *env.Env, call *ssa.CallCommon) bool {
	if call.IsInvoke() {
		return false
	}
	fn, ok := call.Value.(*ssa.Function)
	return ok && flow.Classify(fn) == flow.ClassifiedNamed && flow.GivesOnlyFreshContext(call, e.Kinds)
}

// spawnedCallArgs gives the SSA value of each argument of call, or nil.
//
//declscope:shared // spawn.go and gotask.go read the arguments of a call
func spawnedCallArgs(e *env.Env, call *ast.CallExpr) []ssa.Value {
	instr := e.SSA.CallAt(call)
	if instr == nil {
		return nil
	}
	return flow.ResolvedArgs(instr.Common(), call)
}

// spawnedCallee gives the function call calls, and the number of arguments
// the call gives before the function's parameters: 1 for a method
// expression, as (*errgroup.Group).Go(g, fn), which takes the receiver
// first, and 0 otherwise. A call through a function or method value, as
// gofn := wg.Go, or through a method expression in a variable, as
// gofn := (*errgroup.Group).Go, is found through SSA.
//
//declscope:shared // spawn.go and gotask.go find the API a call makes alike
func spawnedCallee(e *env.Env, call *ast.CallExpr) (*types.Func, int) {
	if fn := funcspec.ExtractFunc(e.Pass, call); fn != nil {
		if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
			if s := e.Pass.TypesInfo.Selections[sel]; s != nil && s.Kind() == types.MethodExpr {
				return fn, 1
			}
		}
		return fn, 0
	}
	instr := e.SSA.CallAt(call)
	if instr == nil || instr.Common().IsInvoke() || instr.Common().StaticCallee() == nil {
		return nil, 0
	}
	common := instr.Common()
	fn := flow.ExtractCalledFunc(common)
	// A method value is a bound wrapper that holds the receiver. A method
	// called any other way is given the receiver first.
	if flow.Classify(common.StaticCallee()) != flow.ClassifiedBound && fn != nil && fn.Signature().Recv() != nil {
		return fn, 1
	}
	return fn, 0
}
