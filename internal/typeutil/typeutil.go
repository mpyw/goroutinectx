package typeutil

import (
	"go/types"
	"strings"
)

const contextPkgPath = "context"

// IsContextType checks if the type is context.Context.
func IsContextType(t types.Type) bool {
	t = UnwrapPointer(t)

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

// UnwrapPointer recursively unwraps all pointer layers.
//
// This is critical for SSA-based carrier type matching. When a closure captures
// a pointer variable, SSA represents it with an additional level of indirection:
//
//	func handler(ctx *CarrierType) {
//	    go func() {
//	        _ = ctx  // SSA FreeVars: **CarrierType (not *CarrierType)
//	    }()
//	}
//
// Therefore, we must unwrap ALL pointer layers to match against the registered
// carrier type (CarrierType, no pointer). Single-layer unwrapping would leave
// *CarrierType, which wouldn't match.
func UnwrapPointer(t types.Type) types.Type {
	for {
		ptr, ok := t.(*types.Pointer)
		if !ok {
			return t
		}
		t = ptr.Elem()
	}
}

// PkgPathMatches checks if pkgPath matches targetPkg, allowing version suffixes.
func PkgPathMatches(pkgPath, targetPkg string) bool {
	if pkgPath == targetPkg {
		return true
	}
	// Check for version suffix like /v2, /v3, etc.
	rest, ok := strings.CutPrefix(pkgPath, targetPkg+"/v")
	return ok && len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9'
}
