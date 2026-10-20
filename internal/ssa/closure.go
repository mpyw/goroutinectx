package ssa

import (
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/carrier"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/funcspec"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// ClosureCapturesContext checks if a closure captures any context.Context variable
// or a configured carrier type. A variable that only ever holds a fresh
// context.Background() or TODO() does not count. A context the closure reads
// from a field or an element of a variable it captures counts, and so does a
// captured closure that captures one.
func ClosureCapturesContext(closure *ssa.Function, carriers []carrier.Carrier) bool {
	return closureCaptures(closure, carriers, make(map[*ssa.Function]bool))
}

func closureCaptures(closure *ssa.Function, carriers []carrier.Carrier, seen map[*ssa.Function]bool) bool {
	if closure == nil || seen[closure] {
		return false
	}
	seen[closure] = true

	return slices.ContainsFunc(closure.FreeVars, func(fv *ssa.FreeVar) bool {
		switch {
		case carrier.IsCarrierType(fv.Type(), carriers):
			return true
		case typeutil.IsContextType(fv.Type()):
			return !closureFreeVarIsFresh(fv)
		}
		return closureReadsContext(fv, carriers) || closureHoldsCapturing(fv, carriers, seen)
	})
}

// closureReadsContext reports whether a context or a carrier is read from v,
// through the fields and elements of what it holds.
func closureReadsContext(v ssa.Value, carriers []carrier.Carrier) bool {
	for _, ref := range *v.Referrers() {
		part, ok := ref.(ssa.Value)
		if !ok {
			continue
		}
		switch r := ref.(type) {
		case *ssa.UnOp:
			if r.Op != token.MUL {
				continue
			}
		case *ssa.FieldAddr, *ssa.Field, *ssa.IndexAddr, *ssa.Index:
		default:
			continue
		}
		if typeutil.IsContextType(part.Type()) || carrier.IsCarrierType(part.Type(), carriers) ||
			closureReadsContext(part, carriers) {
			return true
		}
	}
	return false
}

// closureHoldsCapturing reports whether the func variable fv captures holds
// a closure that captures a context.
func closureHoldsCapturing(fv *ssa.FreeVar, carriers []carrier.Carrier, seen map[*ssa.Function]bool) bool {
	if _, isFunc := typeutil.UnwrapPointer(fv.Type()).Underlying().(*types.Signature); !isFunc {
		return false
	}
	values, ok := resolvedFreeVarValues(fv)
	return ok && slices.ContainsFunc(values, func(v ssa.Value) bool {
		mc, ok := v.(*ssa.MakeClosure)
		if !ok {
			return false
		}
		fn, _ := mc.Fn.(*ssa.Function)
		return closureCaptures(fn, carriers, seen)
	})
}

// closureFreeVarIsFresh reports whether every value fv can hold is a fresh
// context, as IsFreshContext judges it.
func closureFreeVarIsFresh(fv *ssa.FreeVar) bool {
	values, ok := resolvedFreeVarValues(fv)
	return ok && len(values) > 0 && !slices.ContainsFunc(values, func(v ssa.Value) bool { return !IsFreshContext(v) })
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
