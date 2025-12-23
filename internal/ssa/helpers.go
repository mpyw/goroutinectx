package ssa

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// ExtractCalledFunc extracts the types.Func from a CallCommon.
// Handles interface method calls, static calls, and generic function instantiations.
func ExtractCalledFunc(call *ssa.CallCommon) *types.Func {
	if call.IsInvoke() {
		// Interface method call
		return call.Method
	}

	// Static call
	if fn := call.StaticCallee(); fn != nil {
		// Try to get the Object directly
		if obj, ok := fn.Object().(*types.Func); ok {
			return obj
		}

		// For generic function instantiations, Object() returns nil.
		// Use Origin() to get the generic function before instantiation.
		if origin := fn.Origin(); origin != nil {
			if obj, ok := origin.Object().(*types.Func); ok {
				return obj
			}
		}
	}

	return nil
}

// ExtractIIFE checks if a CallCommon is an IIFE (immediately invoked function expression).
// Returns the called function if it's an IIFE, nil otherwise.
func ExtractIIFE(call *ssa.CallCommon) *ssa.Function {
	if call.IsInvoke() {
		return nil
	}

	// Check if the callee is a MakeClosure
	if mc, ok := call.Value.(*ssa.MakeClosure); ok {
		if fn, ok := mc.Fn.(*ssa.Function); ok {
			return fn
		}
	}

	// Check if the callee is a direct function reference (anonymous function)
	if fn, ok := call.Value.(*ssa.Function); ok {
		if fn.Parent() != nil {
			return fn
		}
	}

	return nil
}

// HasFuncArgs checks if the call has func-typed arguments starting from startIdx.
// Handles both direct function arguments and variadic slices of functions.
func HasFuncArgs(call *ssa.CallCommon, startIdx int) bool {
	args := call.Args
	if startIdx < 0 || startIdx >= len(args) {
		return false
	}

	for i := startIdx; i < len(args); i++ {
		underlying := args[i].Type().Underlying()
		// Direct function argument
		if _, isFunc := underlying.(*types.Signature); isFunc {
			return true
		}
		// Variadic slice of functions (e.g., ...func(ctx) error)
		if slice, ok := underlying.(*types.Slice); ok {
			if _, isFunc := slice.Elem().Underlying().(*types.Signature); isFunc {
				return true
			}
		}
	}
	return false
}
