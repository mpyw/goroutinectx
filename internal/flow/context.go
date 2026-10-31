package flow

import (
	"slices"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/ctxkind"
)

// IsFreshContext reports whether every value v can be is a call that
// [ctxkind.IsFreshCall] accepts, as context.Background(): a context that
// carries nothing of the caller's.
func IsFreshContext(v ssa.Value) bool {
	values, ok := ResolvedValues(v)
	if !ok || len(values) == 0 {
		return false
	}
	return !slices.ContainsFunc(values, func(v ssa.Value) bool {
		call, ok := v.(*ssa.Call)
		return !ok || !ctxkind.IsFreshCall(ExtractCalledFunc(call.Common()))
	})
}

// GivesOnlyFreshContext reports whether call gives a fresh
// context.Background() or TODO(), and every other argument is sure to hold
// no context or carrier, as [ctxkind.Classifier.MayHold] judges it.
func GivesOnlyFreshContext(call *ssa.CallCommon, kinds *ctxkind.Classifier) bool {
	given := false
	for _, arg := range call.Args {
		if kinds.Is(arg.Type(), ctxkind.Context) && IsFreshContext(arg) {
			given = true
			continue
		}
		if kinds.MayHold(arg, ctxkind.Param) {
			return false
		}
	}
	return given
}
