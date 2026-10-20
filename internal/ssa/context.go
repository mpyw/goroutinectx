package ssa

import (
	"slices"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/carrier"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// HasContextParam reports whether fn takes a context.Context.
func HasContextParam(fn *ssa.Function) bool {
	return slices.ContainsFunc(fn.Params, func(p *ssa.Parameter) bool { return typeutil.IsContextType(p.Type()) })
}

// ArgsInvolveContext reports whether any argument of call is built from a
// context or a carrier: one is the argument, or among the values it is made
// from, as a closure that captures one.
func ArgsInvolveContext(call *ssa.CallCommon, carriers []carrier.Carrier) bool {
	seen := make(map[ssa.Value]bool)
	return slices.ContainsFunc(call.Args, func(arg ssa.Value) bool { return contextInvolved(arg, carriers, seen) })
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
