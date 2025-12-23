package typeutil

import (
	"go/ast"
	"go/types"
	"strings"

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
	t = UnwrapPointer(t)

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

// UnwrapPointer returns the element type if t is a pointer, otherwise returns t.
func UnwrapPointer(t types.Type) types.Type {
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

// ShortPkgName returns the last component of a package path.
// Example: "github.com/foo/bar" -> "bar"
func ShortPkgName(pkgPath string) string {
	if idx := strings.LastIndex(pkgPath, "/"); idx >= 0 {
		return pkgPath[idx+1:]
	}
	return pkgPath
}

// MatchPkg checks if pkgPath matches targetPkg, allowing version suffixes (/v2, /v3, etc.).
// This handles Go module versioning where:
//   - github.com/foo/bar matches github.com/foo/bar
//   - github.com/foo/bar/v2 matches github.com/foo/bar
//   - github.com/foo/bar/sub does NOT match github.com/foo/bar
//   - github.com/foo/bar/vault does NOT match github.com/foo/bar
func MatchPkg(pkgPath, targetPkg string) bool {
	if pkgPath == targetPkg {
		return true
	}
	// Check for version suffix like /v2, /v3, etc.
	prefix := targetPkg + "/v"
	if !strings.HasPrefix(pkgPath, prefix) {
		return false
	}
	// After /v, there must be a digit
	rest := pkgPath[len(prefix):]
	return len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9'
}
