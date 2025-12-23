// Package funcspec provides shared function specification parsing and matching.
package funcspec

import (
	"go/ast"
	"go/types"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Spec holds parsed components of a function specification.
// Format: "pkg/path.Func" or "pkg/path.Type.Method".
type Spec struct {
	PkgPath  string
	TypeName string // empty for package-level functions
	FuncName string
}

// Parse parses a single function specification string into components.
// Format: "pkg/path.Func" or "pkg/path.Type.Method".
func Parse(s string) Spec {
	spec := Spec{}

	lastDot := strings.LastIndex(s, ".")
	if lastDot == -1 {
		spec.FuncName = s

		return spec
	}

	spec.FuncName = s[lastDot+1:]
	prefix := s[:lastDot]

	// Check if there's another dot (indicating Type.Method)
	// Type names start with uppercase in Go.
	secondLastDot := strings.LastIndex(prefix, ".")
	if secondLastDot != -1 {
		possibleType := prefix[secondLastDot+1:]
		if len(possibleType) > 0 && unicode.IsUpper(rune(possibleType[0])) {
			spec.TypeName = possibleType
			spec.PkgPath = prefix[:secondLastDot]

			return spec
		}
	}

	spec.PkgPath = prefix

	return spec
}

// FullName returns the full API name for message formatting.
// Format: "pkg.Func" or "pkg.Type.Method" where pkg is the short package name.
func (s Spec) FullName() string {
	shortPkg := typeutil.ShortPkgName(s.PkgPath)
	if s.TypeName != "" {
		return shortPkg + "." + s.TypeName + "." + s.FuncName
	}
	return shortPkg + "." + s.FuncName
}

// Matches checks if a types.Func matches this specification.
// Uses MatchPkg to handle version suffixes (v2, v3, etc.).
func (s Spec) Matches(fn *types.Func) bool {
	if fn.Name() != s.FuncName {
		return false
	}

	pkg := fn.Pkg()
	if pkg == nil || !typeutil.MatchPkg(pkg.Path(), s.PkgPath) {
		return false
	}

	// Check if it's a method
	sig := fn.Type().(*types.Signature)
	recv := sig.Recv()

	if s.TypeName == "" {
		// Package-level function: should have no receiver
		return recv == nil
	}

	// Method: should have receiver of correct type
	if recv == nil {
		return false
	}

	recvType := typeutil.UnwrapPointer(recv.Type())

	named, ok := recvType.(*types.Named)
	if !ok {
		return false
	}

	return named.Obj().Name() == s.TypeName
}

// ExtractFunc extracts the types.Func from a call expression.
// Returns nil if the callee cannot be determined statically.
func ExtractFunc(pass *analysis.Pass, call *ast.CallExpr) *types.Func {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		obj := pass.TypesInfo.ObjectOf(fun)
		if f, ok := obj.(*types.Func); ok {
			return f
		}

	case *ast.SelectorExpr:
		sel := pass.TypesInfo.Selections[fun]
		if sel != nil {
			if f, ok := sel.Obj().(*types.Func); ok {
				return f
			}
		} else {
			obj := pass.TypesInfo.ObjectOf(fun.Sel)
			if f, ok := obj.(*types.Func); ok {
				return f
			}
		}
	}

	return nil
}
