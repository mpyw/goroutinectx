package ssa

import (
	"go/types"
	"slices"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/carrier"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/funcspec"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// ClosureCapturesContext checks if a closure captures any context.Context variable
// or a configured carrier type.
func ClosureCapturesContext(closure *ssa.Function, carriers []carrier.Carrier) bool {
	if closure == nil {
		return false
	}

	return slices.ContainsFunc(closure.FreeVars, func(fv *ssa.FreeVar) bool {
		return typeutil.IsContextType(fv.Type()) || carrier.IsCarrierType(fv.Type(), carriers)
	})
}

// ClosureDerivation is where a closure calls the deriver.
type ClosureDerivation struct {
	FoundAtStart     bool
	FoundOnlyInDefer bool
}

// ClosureCallsDeriver checks if a closure calls any of the required deriver functions.
func ClosureCallsDeriver(closure *ssa.Function, matcher *deriver.Matcher) ClosureDerivation {
	if closure == nil || matcher.IsEmpty() {
		return ClosureDerivation{FoundAtStart: true}
	}

	var calls []closureDeriverCall
	(&CallWalker{}).Walk(closure, func(site WalkedCall) bool {
		if site.isGo {
			return true
		}
		if fn := ExtractCalledFunc(site.Call); fn != nil {
			calls = append(calls, closureDeriverCall{fn: fn, inDefer: site.inDefer})
		}
		return true
	})

	// Check if any OR group is satisfied at start
	for _, andGroup := range matcher.OrGroups {
		if closureCallsGroup(calls, andGroup, false) {
			return ClosureDerivation{FoundAtStart: true}
		}
	}

	// Check if deriver is only in defer
	for _, andGroup := range matcher.OrGroups {
		if closureCallsGroup(calls, andGroup, true) {
			return ClosureDerivation{FoundOnlyInDefer: true}
		}
	}

	return ClosureDerivation{}
}

type closureDeriverCall struct {
	fn      *types.Func
	inDefer bool
}

func closureCallsGroup(calls []closureDeriverCall, andGroup []funcspec.Spec, includeDefer bool) bool {
	for _, spec := range andGroup {
		matches := func(call closureDeriverCall) bool {
			return (includeDefer || !call.inDefer) && call.fn != nil && spec.Matches(call.fn)
		}
		if !slices.ContainsFunc(calls, matches) {
			return false
		}
	}
	return true
}
