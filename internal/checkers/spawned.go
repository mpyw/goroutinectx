package checkers

import (
	"go/types"
	"slices"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/ctxkind"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/env"
	"github.com/mpyw/goroutinectx/internal/flow"
)

// spawnedContextPolicy judges a closure by whether it takes or captures a
// context (with derivers, it must take a context of its own,
// [spawnedContextPolicy.paramKind]), or, when derivers is not empty, calls the deriver at its start.
// When derivers is empty, a closure that only waits and signals passes too
// ([spawnedWaiterPasses]). With derivers, it must still call the deriver,
// as a go statement must for the goroutinederive checker (#243). A factory
// passes when it takes a context, or is given one other than a fresh
// context.Background() or TODO(). A named function given only a fresh
// context fails, as it has no other way to get ctx.
//
// A func value a closure captures and runs ([flow.SiteCaptured]) is judged
// by the same rules, as if it were spawned directly (#228), but a closure
// that takes a context does not pass for that alone, as the closure that
// runs it gives it its arguments, and they count only when that closure
// captures them. Nor does one pass for only waiting and signaling.
type spawnedContextPolicy struct {
	e        *env.Env
	derivers *deriver.Matcher
	// goCall is the call of a go statement that runs the closure judged, or
	// nil. With one, a context parameter of the closure counts only when
	// the go statement gives it a context other than a fresh
	// context.Background() or TODO(), or gives another value that may hold
	// a context ([spawnedContextPolicy.goParamsPass]).
	goCall *ssa.CallCommon
	// onlyFresh is whether goCall gives only fresh contexts, as
	// [flow.Flow.GivesOnlyFreshContext] tells.
	onlyFresh bool
}

// newSpawnedContextPolicy gives the policy of a callback, or of a go
// statement whose call SSA does not give, with derivers.
//
//declscope:shared // goroutine.go and spawn.go check for a context alike
func newSpawnedContextPolicy(e *env.Env, derivers *deriver.Matcher) *spawnedContextPolicy {
	return &spawnedContextPolicy{e: e, derivers: derivers}
}

// newSpawnedGoContextPolicy gives the policy of a go statement whose call
// SSA gives, as call. Its parameters are judged by what call gives them.
//
//declscope:shared // goroutine.go checks a go statement for a context
func newSpawnedGoContextPolicy(e *env.Env, call *ssa.CallCommon) *spawnedContextPolicy {
	return &spawnedContextPolicy{e: e, goCall: call, onlyFresh: e.Flow.GivesOnlyFreshContext(call, e.Kinds)}
}

// Literal judges a closure as [spawnedContextPolicy] tells.
func (p *spawnedContextPolicy) Literal(fn *ssa.Function, site flow.Site) flow.Verdict {
	spawned := site == flow.SiteSpawned
	switch {
	case spawned && p.goCall == nil && p.e.Kinds.HasParam(fn, p.paramKind()),
		p.e.Flow.ClosureCapturesContext(fn, p.e.Kinds, p),
		!p.derivers.IsEmpty() && flow.ClosureCallsDeriver(fn, p.derivers).FoundAtStart,
		spawned && p.derivers.IsEmpty() && spawnedWaiterPasses(p.e, fn),
		spawned && p.goCall != nil && p.goParamsPass(fn):
		return flow.Pass
	}
	return flow.Fail
}

// paramKind gives the kinds a parameter of a spawned closure counts as a
// context by. With no deriver it is [ctxkind.Given], as a closure that gets
// a value holding ctx uses ctx. With derivers it is [ctxkind.Captured]: only
// a closure given a context of its own, not a struct that holds one, is
// left unchecked for the deriver, as for the goroutinederive checker (#256).
func (p *spawnedContextPolicy) paramKind() ctxkind.Kind {
	if p.derivers.IsEmpty() {
		return ctxkind.Given
	}
	return ctxkind.Captured
}

// goParamsPass reports whether a parameter of fn ties it to ctx, as goCall
// gives it: a context other than a fresh one, as [ctxkind.Given] counts
// one, the channel of ctx.Done(), a cancel func of a context made from
// ctx, or, for a parameter of another type that may hold a context, a value
// built from a context other than a fresh one, as [ctxkind.Classifier.Involves]
// follows it: an interface, a func, a slice or an any given ctx (#252).
func (p *spawnedContextPolicy) goParamsPass(fn *ssa.Function) bool {
	args := p.goCall.Args
	return slices.ContainsFunc(fn.Params, func(param *ssa.Parameter) bool {
		i := slices.Index(fn.Params, param)
		if p.e.Kinds.Is(param.Type(), ctxkind.Given) {
			return i >= len(args) || !p.onlyFresh || !p.e.Flow.IsFreshContext(args[i])
		}
		if i >= len(args) {
			return false
		}
		arg := args[i]
		return p.e.Flow.SignalsContext(arg, p.e.Kinds) ||
			(p.e.Kinds.MayHold(param, ctxkind.Given) && p.e.Kinds.InvolvesUnless(ctxkind.Given, p.goArgPartHoldsNoContext, arg))
	})
}

// goArgPartHoldsNoContext reports whether v, a part of an argument a go
// statement gives a parameter of another type, is not to count toward ctx
// nor to be looked into ([spawnedContextPolicy.goParamsPass]):
//
//   - a context.Context made by a fresh context.Background() or TODO(), as
//     [flow.Flow.IsFreshContext] tells;
//   - the result of a call, of a type that is not of [ctxkind.Given] and not
//     an interface or a func. Such a result is data the
//     call made, as the brokers a call given ctx lists, not a holder of what
//     the call was given. A call of many results is judged by each one taken
//     from it.
func (p *spawnedContextPolicy) goArgPartHoldsNoContext(v ssa.Value) bool {
	if p.e.Kinds.Is(v.Type(), ctxkind.Context) {
		return p.e.Flow.IsFreshContext(v)
	}
	switch x := v.(type) {
	case *ssa.Call:
		if _, many := x.Type().(*types.Tuple); many {
			return false
		}
	case *ssa.Extract:
		if _, ok := x.Tuple.(*ssa.Call); !ok {
			return false
		}
	default:
		return false
	}
	if p.e.Kinds.Is(v.Type(), ctxkind.Given) {
		return false
	}
	switch v.Type().Underlying().(type) {
	case *types.Interface, *types.Signature:
		return false
	}
	return true
}

// Call fails a named function given only a fresh context
// ([spawnedGivesNamedFuncOnlyFreshContext]).
func (p *spawnedContextPolicy) Call(call *ssa.CallCommon) flow.Verdict {
	if spawnedGivesNamedFuncOnlyFreshContext(p.e, call) {
		return flow.Fail
	}
	return flow.Unknown
}

// FactoryCall passes a factory given a context other than a fresh one, as
// [ctxkind.Given] counts one.
func (p *spawnedContextPolicy) FactoryCall(call *ssa.CallCommon) bool {
	return p.e.Kinds.Involves(ctxkind.Given, call.Args...) && !p.e.Flow.GivesOnlyFreshContext(call, p.e.Kinds)
}

// Factory passes a factory that takes a context, as [ctxkind.Given] counts
// one.
func (p *spawnedContextPolicy) Factory(fn *ssa.Function) bool {
	return p.e.Kinds.HasParam(fn, ctxkind.Given)
}

// spawnedWaiterPasses passes a closure that only waits and signals, as
// [flow.Flow.IsWaiter] tells one, unless -report-waiters is set (#230). It
// has no use for ctx. Only the closure that is spawned passes this way: a
// func value it runs does not pass for being a waiter, as it holds no
// context the closure could pass on.
func spawnedWaiterPasses(e *env.Env, fn *ssa.Function) bool {
	return !e.ReportWaiters && e.Flow.IsWaiter(fn)
}

// spawnedGivesNamedFuncOnlyFreshContext reports whether call calls a named
// function, not a method, generic or not, of the analyzed package or of
// another, and gives it a fresh context.Background() or TODO() and nothing
// else that may hold a context. A named function is not looked into, but one
// given only a fresh context does not get ctx. A method is left alone, as
// its receiver may hold a context, and so is a closure, which may capture
// one.
//
//declscope:shared // goroutine.go judges a named function a go statement calls alike
func spawnedGivesNamedFuncOnlyFreshContext(e *env.Env, call *ssa.CallCommon) bool {
	if call.IsInvoke() {
		return false
	}
	fn, ok := call.Value.(*ssa.Function)
	if !ok {
		return false
	}
	switch flow.Classify(fn) {
	case flow.ClassifiedNamed:
	case flow.ClassifiedExternal:
		// A function of another package is built from type information,
		// and so is a method of one.
		if fn.Signature.Recv() != nil {
			return false
		}
	default:
		return false
	}
	return e.Flow.GivesOnlyFreshContext(call, e.Kinds)
}
