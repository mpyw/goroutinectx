package flow

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
