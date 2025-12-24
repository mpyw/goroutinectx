package typeutil

import (
	"go/types"
)

const contextPkgPath = "context"

// IsContextType checks if the type is context.Context.
func IsContextType(t types.Type) bool {
	t = unwrapPointer(t)

	named, ok := t.(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}

	return obj.Pkg().Path() == contextPkgPath && obj.Name() == "Context"
}

// unwrapPointer returns the element type if t is a pointer, otherwise returns t.
func unwrapPointer(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}
