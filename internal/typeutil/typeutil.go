package typeutil

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/directives/carrier"
)

const contextPkgPath = "context"

// IsNamedType checks if the expression has the given named type.
// It handles pointer types automatically.
func IsNamedType(pass *analysis.Pass, expr ast.Expr, pkgPath, typeName string) bool {
	tv, ok := pass.TypesInfo.Types[expr]
	if !ok {
		return false
	}

	return isNamedTypeFromType(tv.Type, pkgPath, typeName)
}

// isNamedTypeFromType checks if the type matches the given package path and type name.
func isNamedTypeFromType(t types.Type, pkgPath, typeName string) bool {
	t = unwrapPointer(t)

	named, ok := t.(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}

	return obj.Pkg().Path() == pkgPath && obj.Name() == typeName
}

// unwrapPointer returns the element type if t is a pointer, otherwise returns t.
func unwrapPointer(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}

	return t
}

// IsContextType checks if the type is context.Context.
func IsContextType(t types.Type) bool {
	return isNamedTypeFromType(t, contextPkgPath, "Context")
}

// IsContextOrCarrierType checks if the type is context.Context or a configured carrier type.
func IsContextOrCarrierType(t types.Type, carriers []carrier.Carrier) bool {
	if IsContextType(t) {
		return true
	}

	for _, c := range carriers {
		if isNamedTypeFromType(t, c.PkgPath, c.TypeName) {
			return true
		}
	}

	return false
}
