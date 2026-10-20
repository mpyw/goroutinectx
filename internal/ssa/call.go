package ssa

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// ExtractCalledFunc extracts the types.Func from a CallCommon. A function
// or method of a generic type is given as its generic origin.
func ExtractCalledFunc(call *ssa.CallCommon) *types.Func {
	if call.IsInvoke() {
		return call.Method.Origin()
	}

	if fn := call.StaticCallee(); fn != nil {
		if obj, ok := fn.Object().(*types.Func); ok {
			return obj.Origin()
		}
		if origin := fn.Origin(); origin != nil {
			if obj, ok := origin.Object().(*types.Func); ok {
				return obj
			}
		}
	}

	return nil
}

// CallHasFuncArgs checks if the call has func-typed arguments starting from startIdx.
func CallHasFuncArgs(call *ssa.CallCommon, startIdx int) bool {
	args := call.Args
	if startIdx < 0 || startIdx >= len(args) {
		return false
	}

	for i := startIdx; i < len(args); i++ {
		underlying := args[i].Type().Underlying()
		if _, isFunc := underlying.(*types.Signature); isFunc {
			return true
		}
		if slice, ok := underlying.(*types.Slice); ok {
			if _, isFunc := slice.Elem().Underlying().(*types.Signature); isFunc {
				return true
			}
		}
	}
	return false
}
