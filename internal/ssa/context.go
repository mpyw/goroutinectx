package ssa

import (
	"slices"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/carrier"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// HasContextParam reports whether fn takes a context.Context or one of
// carriers.
func HasContextParam(fn *ssa.Function, carriers []carrier.Carrier) bool {
	return slices.ContainsFunc(fn.Params, func(p *ssa.Parameter) bool {
		return typeutil.IsContextType(p.Type()) || carrier.IsCarrierType(p.Type(), carriers)
	})
}

// IsFreshContext reports whether every value v can be is a call of
// context.Background or context.TODO: a context that carries nothing of the
// caller's.
func IsFreshContext(v ssa.Value) bool {
	values, ok := ResolvedValues(v)
	if !ok || len(values) == 0 {
		return false
	}
	return !slices.ContainsFunc(values, func(v ssa.Value) bool {
		call, ok := v.(*ssa.Call)
		if !ok {
			return true
		}
		fn := ExtractCalledFunc(call.Common())
		return fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != "context" ||
			(fn.Name() != "Background" && fn.Name() != "TODO")
	})
}

// ArgsInvolveContext reports whether any argument of call is built from a
// context or a carrier: one is the argument, or among the values it is made
// from, as a closure that captures one.
func ArgsInvolveContext(call *ssa.CallCommon, carriers []carrier.Carrier) bool {
	seen := make(map[ssa.Value]bool)
	return slices.ContainsFunc(call.Args, func(arg ssa.Value) bool { return contextInvolved(arg, carriers, seen) })
}

// GivesOnlyFreshContext reports whether call gives a context argument, and
// every argument built from a context or a carrier is a fresh
// context.Background() or TODO() itself.
func GivesOnlyFreshContext(call *ssa.CallCommon, carriers []carrier.Carrier) bool {
	given := false
	for _, arg := range call.Args {
		if !contextInvolved(arg, carriers, make(map[ssa.Value]bool)) {
			continue
		}
		if !typeutil.IsContextType(arg.Type()) || !IsFreshContext(arg) {
			return false
		}
		given = true
	}
	return given
}

func contextInvolved(v ssa.Value, carriers []carrier.Carrier, seen map[ssa.Value]bool) bool {
	if v == nil || seen[v] {
		return false
	}
	seen[v] = true
	if typeutil.IsContextType(v.Type()) || carrier.IsCarrierType(v.Type(), carriers) {
		return true
	}
	instr, ok := v.(ssa.Instruction)
	if !ok {
		return false
	}
	for _, op := range instr.Operands(nil) {
		if op != nil && contextInvolved(*op, carriers, seen) {
			return true
		}
	}
	return false
}
