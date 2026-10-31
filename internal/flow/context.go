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
// no context or carrier, as [ctxkind.Classifier.MayHold] judges it.
func (f *Flow) GivesOnlyFreshContext(call *ssa.CallCommon, kinds *ctxkind.Classifier) bool {
	defer f.ask()()
	given := false
	for _, arg := range call.Args {
		if kinds.Is(arg.Type(), ctxkind.Context) && f.IsFreshContext(arg) {
			given = true
			continue
		}
		if kinds.MayHold(arg, ctxkind.Param) {
			return false
		}
	}
	return given
}
