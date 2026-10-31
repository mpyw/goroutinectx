package typeutil

import (
	"go/types"
	"strings"
)

const contextPkgPath = "context"

// IsContextType checks if the type is context.Context, through pointers
// and aliases. A type parameter whose constraint is, or embeds,
// context.Context is one too (see [ConstraintEmbeds]).
func IsContextType(t types.Type) bool {
	return isContextNamed(t) || ConstraintEmbeds(t, isContextNamed)
}

func isContextNamed(t types.Type) bool {
	named, ok := UnwrapPointer(t).(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}

	return obj.Pkg().Path() == contextPkgPath && obj.Name() == "Context"
}

// ConstraintEmbeds reports whether t, through pointers and aliases, is a
// type parameter whose constraint match accepts, or whose constraint is an
// interface literal that embeds such a type, as interface{ context.Context }
// does. A named interface that embeds one is not looked into, the same as a
// parameter of that type.
func ConstraintEmbeds(t types.Type, match func(types.Type) bool) bool {
	tp, ok := UnwrapPointer(t).(*types.TypeParam)
	return ok && constraintEmbeds(tp.Constraint(), match)
}

func constraintEmbeds(c types.Type, match func(types.Type) bool) bool {
	if match(c) {
		return true
	}
	iface, ok := types.Unalias(c).(*types.Interface)
	if !ok {
		return false
	}
	for i := range iface.NumEmbeddeds() {
		if constraintEmbeds(iface.EmbeddedType(i), match) {
			return true
		}
	}
	return false
}

// UnwrapPointer recursively unwraps all pointer layers, and the aliases
// around and between them.
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
//
// An alias, such as type Ctx = context.Context, is a *types.Alias and not
// the type it names, so it is unaliased at each layer.
func UnwrapPointer(t types.Type) types.Type {
	for {
		t = types.Unalias(t)
		ptr, ok := t.(*types.Pointer)
		if !ok {
			return t
		}
		t = ptr.Elem()
	}
}

// PkgPathMatches checks if pkgPath matches targetPkg, allowing a major
// version suffix such as /v2. Only the suffix may follow: neither
// /v1alpha1 nor /v2/sub matches.
func PkgPathMatches(pkgPath, targetPkg string) bool {
	if pkgPath == targetPkg {
		return true
	}
	rest, ok := strings.CutPrefix(pkgPath, targetPkg+"/v")
	return ok && rest != "" && strings.Trim(rest, "0123456789") == ""
}
