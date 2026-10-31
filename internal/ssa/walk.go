package ssa

import (
	"golang.org/x/tools/go/ssa"
)

// WalkedCall is a call instruction a [CallWalker] reaches.
type WalkedCall struct {
	// Call is the call made.
	Call *ssa.CallCommon
	// inDefer is true for a deferred call, and for each call inside a
	// deferred IIFE.
	inDefer bool //declscope:shared // closure.go tells a deferred deriver from one at the start
	// isGo is true for a go statement.
	isGo bool //declscope:shared // closure.go skips go statements
}

// CallWalker walks the Call, Defer and Go instructions of a function. It goes
// into each immediately invoked function literal, into the body of each
// range-over-func loop, and with Goroutines into
// each function literal a go statement runs. A walker walks each function once as deferred
// and once as not, however many times it is asked to, so a function both
// deferred and called directly is seen called directly too.
type CallWalker struct {
	// Goroutines makes the walk go into the function literal a go statement
	// runs.
	Goroutines bool

	visited map[walkVisit]bool
}

// walkVisit is a function walked, and whether it was walked as deferred.
type walkVisit struct {
	fn      *ssa.Function
	inDefer bool
}

// Walk calls visit on each call fn makes, in instruction order, until visit
// returns false. It reports whether the walk ran to the end.
func (w *CallWalker) Walk(fn *ssa.Function, visit func(WalkedCall) bool) bool {
	return w.walk(fn, false, visit)
}

func (w *CallWalker) walk(fn *ssa.Function, inDefer bool, visit func(WalkedCall) bool) bool {
	key := walkVisit{fn, inDefer}
	if fn == nil || w.visited[key] {
		return true
	}
	if w.visited == nil {
		w.visited = make(map[walkVisit]bool)
	}
	w.visited[key] = true

	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			switch v := instr.(type) {
			case *ssa.Call:
				if !visit(WalkedCall{Call: &v.Call, inDefer: inDefer}) ||
					!w.walk(calleeToWalk(&v.Call), inDefer, visit) ||
					!w.walk(yieldToWalk(&v.Call), inDefer, visit) {
					return false
				}

			case *ssa.Defer:
				if !visit(WalkedCall{Call: &v.Call, inDefer: true}) ||
					!w.walk(calleeToWalk(&v.Call), true, visit) {
					return false
				}

			case *ssa.Go:
				if !visit(WalkedCall{Call: &v.Call, inDefer: inDefer, isGo: true}) {
					return false
				}
				if w.Goroutines && !w.walk(calleeToWalk(&v.Call), false, visit) {
					return false
				}
			}
		}
	}

	return true
}

// calleeToWalk returns the invoked function if the CallCommon is an IIFE.
func calleeToWalk(call *ssa.CallCommon) *ssa.Function {
	if call.IsInvoke() {
		return nil
	}

	if mc, ok := call.Value.(*ssa.MakeClosure); ok {
		if fn, ok := mc.Fn.(*ssa.Function); ok {
			return fn
		}
	}

	if fn, ok := call.Value.(*ssa.Function); ok {
		if fn.Parent() != nil {
			return fn
		}
	}

	return nil
}

// yieldToWalk returns the body of a range-over-func loop when the call is the
// iterator call that runs it. SSA builds the body as a synthetic yield
// closure and passes it to the iterator, which calls it in the same
// goroutine.
func yieldToWalk(call *ssa.CallCommon) *ssa.Function {
	for _, arg := range call.Args {
		fn, ok := arg.(*ssa.Function)
		if mc, isClosure := arg.(*ssa.MakeClosure); isClosure {
			fn, ok = mc.Fn.(*ssa.Function)
		}
		if ok && fn.Synthetic == "range-over-func yield" {
			return fn
		}
	}
	return nil
}
