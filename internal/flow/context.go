package flow

import (
	"slices"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/ctxkind"
)

// IsFreshContext reports whether every source of v is a call that
// [ctxkind.IsFreshCall] accepts, as context.Background(): a context that
// carries nothing of the caller's.
func (f *Flow) IsFreshContext(v ssa.Value) bool {
	sources, complete := f.Sources(v)
	if !complete || len(sources) == 0 {
		return false
	}
	return !slices.ContainsFunc(sources, func(s Source) bool {
		call, ok := s.Value.(*ssa.Call)
		return !ok || !ctxkind.IsFreshCall(ExtractCalledFunc(call.Common()))
	})
}

// GivesOnlyFreshContext reports whether call gives a fresh
// context.Background() or TODO(), and every other argument is sure to hold
// no context or carrier, as [ctxkind.Classifier.MayHold] judges it, and is
// no signal of a context, as [Flow.SignalsContext] judges it: the channel of
// ctx.Done() ties the callee to ctx (#241).
func (f *Flow) GivesOnlyFreshContext(call *ssa.CallCommon, kinds *ctxkind.Classifier) bool {
	defer f.ask()()
	given := false
	for _, arg := range call.Args {
		if kinds.Is(arg.Type(), ctxkind.Context) && f.IsFreshContext(arg) {
			given = true
			continue
		}
		if kinds.MayHold(arg, ctxkind.Param) || f.SignalsContext(arg, kinds) {
			return false
		}
	}
	return given
}

// SignalsContext reports whether one of the sources of v is a signal of a
// context that is not fresh, as [Flow.IsFreshContext] judges it: the
// channel of its Done method, or the func that cancels a context made from
// it, as context.WithCancel gives second ([ctxkind.IsCancelCall]). A
// goroutine that waits on the one or calls the other is tied to the context.
func (f *Flow) SignalsContext(v ssa.Value, kinds *ctxkind.Classifier) bool {
	defer f.ask()()
	sources, _ := f.Sources(v)
	return slices.ContainsFunc(sources, func(s Source) bool {
		parent, ok := contextSignalled(s.Value, kinds)
		return ok && !f.IsFreshContext(parent)
	})
}

// contextSignalled gives the context v is a signal of, as
// [contextDoneOf] or [contextCancelOf] gives it.
func contextSignalled(v ssa.Value, kinds *ctxkind.Classifier) (ssa.Value, bool) {
	if parent, ok := contextCancelOf(v); ok {
		return parent, true
	}
	return contextDoneOf(v, kinds)
}

// contextDoneOf gives the context whose Done method v is the result of.
func contextDoneOf(v ssa.Value, kinds *ctxkind.Classifier) (ssa.Value, bool) {
	x, ok := v.(*ssa.Call)
	if !ok {
		return nil, false
	}
	call := x.Common()
	fn := ExtractCalledFunc(call)
	// A method value, as done := ctx.Done, is called with no receiver.
	if fn == nil || fn.Name() != "Done" || fn.Signature().Recv() == nil || (!call.IsInvoke() && len(call.Args) == 0) {
		return nil, false
	}
	recv := call.Value
	if !call.IsInvoke() {
		recv = call.Args[0]
	}
	return recv, kinds.Is(recv.Type(), ctxkind.Context|ctxkind.Implements)
}

// contextCancelOf gives the context given to the call whose cancel func v
// is, the second result of a call that [ctxkind.IsCancelCall] accepts.
//
//declscope:shared // resolve.go judges a cancel func by the context it cancels
func contextCancelOf(v ssa.Value) (ssa.Value, bool) {
	x, ok := v.(*ssa.Extract)
	if !ok || x.Index != 1 {
		return nil, false
	}
	call, ok := x.Tuple.(*ssa.Call)
	if !ok || !ctxkind.IsCancelCall(ExtractCalledFunc(call.Common())) {
		return nil, false
	}
	return call.Common().Args[0], true
}
