package flow

import (
	"iter"

	"golang.org/x/tools/go/ssa"
)

// RunCall is a call instruction [Runs] gives.
type RunCall struct {
	// Call is the call made.
	Call *ssa.CallCommon
	// inDefer is true for a deferred call, and for each call inside a
	// deferred IIFE.
	inDefer bool //declscope:shared // closure.go tells a deferred deriver from one at the start
	// isGo is true for a go statement.
	isGo bool //declscope:shared // closure.go skips go statements
}

// IsGo reports whether the call is a go statement.
func (c RunCall) IsGo() bool {
	return c.isGo
}

// RunOptions is which functions [Runs] goes into, past the IIFEs and the
// range-over-func loop bodies it always goes into.
type RunOptions struct {
	// Goroutines goes into the function literal a go statement runs.
	Goroutines bool
	// Returned goes into each function literal a call of a static callee
	// returns, as any result of any of its returns, when the call's result
	// is called, as factory()() does.
	Returned bool
	// Flow follows the called value back to the call, for Returned, and
	// to the function literals it can be, so that a closure called through
	// a Phi is gone into as one called directly is (#306).
	Flow *Flow
}

// Runs gives the Call, Defer and Go instructions fn runs, in instruction
// order. Each call is given before the functions it goes into for it. It
// goes into each function once as deferred and once as not, so a function
// both deferred and called directly is seen called directly too.
func Runs(fn *ssa.Function, opts RunOptions) iter.Seq[RunCall] {
	return func(yield func(RunCall) bool) {
		r := &runner{opts: opts, yield: yield, visited: make(map[runVisit]bool)}
		r.walk(fn, false)
	}
}

// runner is the state of one [Runs].
type runner struct {
	opts    RunOptions
	yield   func(RunCall) bool
	visited map[runVisit]bool
}

// runVisit is a function walked, and whether it was walked as deferred.
type runVisit struct {
	fn      *ssa.Function
	inDefer bool
}

// walk gives the calls fn makes. It reports false when yield stopped it.
func (r *runner) walk(fn *ssa.Function, inDefer bool) bool {
	key := runVisit{fn, inDefer}
	if fn == nil || r.visited[key] {
		return true
	}
	r.visited[key] = true

	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			switch v := instr.(type) {
			case *ssa.Call:
				if !r.call(RunCall{Call: &v.Call, inDefer: inDefer}) ||
					!r.walkCallee(&v.Call, inDefer) ||
					!r.walk(runRangeBody(&v.Call), inDefer) {
					return false
				}

			case *ssa.Defer:
				if !r.call(RunCall{Call: &v.Call, inDefer: true}) ||
					!r.walkCallee(&v.Call, true) {
					return false
				}

			case *ssa.Go:
				if !r.call(RunCall{Call: &v.Call, inDefer: inDefer, isGo: true}) {
					return false
				}
				if r.opts.Goroutines && !r.walkCallee(&v.Call, false) {
					return false
				}
			}
		}
	}

	return true
}

// walkCallee walks the function call runs in place, as [runCallee] gives
// it, or with Flow, each function literal the called value can be, as
// [Flow.Sources] follows it. It reports false when yield stopped it.
func (r *runner) walkCallee(call *ssa.CallCommon, inDefer bool) bool {
	if r.opts.Flow == nil || call.IsInvoke() {
		return r.walk(runCallee(call), inDefer)
	}
	sources, _ := r.opts.Flow.Sources(call.Value)
	for _, s := range sources {
		if !r.walk(runLiteral(s.Value), inDefer) {
			return false
		}
	}
	return true
}

// call gives site, and with Returned walks the closures its callee returns.
// They are walked as not deferred, whatever site is.
func (r *runner) call(site RunCall) bool {
	if !r.yield(site) {
		return false
	}
	if !r.opts.Returned || site.Call.IsInvoke() {
		return true
	}
	sources, _ := r.opts.Flow.Sources(site.Call.Value)
	for _, s := range sources {
		inner, ok := s.Value.(*ssa.Call)
		if !ok || inner.Call.StaticCallee() == nil {
			continue
		}
		for _, closure := range runReturnedClosures(inner.Call.StaticCallee()) {
			if !r.walk(closure, false) {
				return false
			}
		}
	}
	return true
}

// runReturnedClosures gives the function literals fn returns, as any result
// of any of its returns. An instantiation of a generic function, a
// synthetic wrapper or one with no body of its own, gives those of its
// origin (#303).
func runReturnedClosures(fn *ssa.Function) []*ssa.Function {
	if fn.Origin() != nil {
		fn = fn.Origin()
	}
	var closures []*ssa.Function
	for _, result := range returns(fn, returnedEvery) {
		switch r := result.(type) {
		case *ssa.MakeClosure:
			if closureFn, ok := r.Fn.(*ssa.Function); ok {
				closures = append(closures, closureFn)
			}
		case *ssa.Function:
			// A function literal that captures nothing.
			if runIsBody(r) {
				closures = append(closures, r)
			}
		}
	}
	return closures
}

// runCallee gives the function a call runs in place, an IIFE: a closure it
// makes, or a function literal that captures nothing.
//
//declscope:shared // waiter.go goes into the IIFEs a waiter runs, as Runs does
func runCallee(call *ssa.CallCommon) *ssa.Function {
	if call.IsInvoke() {
		return nil
	}
	return runLiteral(call.Value)
}

// runLiteral gives the function literal v is: the function of a closure,
// or a function literal that captures nothing, or nil.
func runLiteral(v ssa.Value) *ssa.Function {
	if mc, ok := v.(*ssa.MakeClosure); ok {
		fn, _ := mc.Fn.(*ssa.Function)
		return fn
	}

	if fn, ok := v.(*ssa.Function); ok && runIsBody(fn) {
		return fn
	}

	return nil
}

// runIsBody reports whether fn is a function literal or the body of a
// range-over-func loop: a function nested in another.
func runIsBody(fn *ssa.Function) bool {
	kind := Classify(fn)
	return kind == ClassifiedLiteral || kind == ClassifiedRangeBody
}

// runRangeBody gives the body of a range-over-func loop when the call is the
// iterator call that runs it. SSA builds the body as a synthetic yield
// closure and passes it to the iterator, which calls it in the same
// goroutine.
//
//declscope:shared // closure.go leaves the iterator call to the body Runs goes into
func runRangeBody(call *ssa.CallCommon) *ssa.Function {
	for _, arg := range call.Args {
		fn, ok := arg.(*ssa.Function)
		if mc, isClosure := arg.(*ssa.MakeClosure); isClosure {
			fn, ok = mc.Fn.(*ssa.Function)
		}
		if ok && Classify(fn) == ClassifiedRangeBody {
			return fn
		}
	}
	return nil
}
