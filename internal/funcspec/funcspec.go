package funcspec

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/types"
	"slices"
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

// Parse parses a single function specification string. It rejects a
// specification with no package, or whose package is a bare name that is not
// a standard library package, as apm.NewGoroutineContext: such a
// specification never matches, since a package is matched by its full
// import path.
func Parse(s string) (Spec, error) {
	spec := parseSpec(s)
	if spec.PkgPath == "" || spec.FuncName == "" {
		return Spec{}, fmt.Errorf("%q: want pkg/path.Func or pkg/path.Type.Method", s)
	}
	if err := CheckPkgPath(s, spec.PkgPath); err != nil {
		return Spec{}, err
	}
	return spec, nil
}

// CheckPkgPath rejects pkgPath, the package of the name s, when it is a bare
// name that is not a standard library package. Such a name never matches,
// since a package is matched by its full import path.
func CheckPkgPath(s, pkgPath string) error {
	if !strings.Contains(pkgPath, "/") && !isStdPackage(pkgPath) {
		return fmt.Errorf("%q: %q is not an import path; write the full path of the package, as github.com/org/%s",
			s, pkgPath, s)
	}
	return nil
}

// stdTopLevel lists the standard library packages whose path has no slash.
var stdTopLevel = []string{
	"bufio", "bytes", "cmp", "context", "crypto", "embed", "encoding", "errors", "expvar", "flag", "fmt",
	"hash", "html", "image", "io", "iter", "log", "maps", "math", "mime", "net", "os", "path", "plugin",
	"reflect", "regexp", "runtime", "slices", "sort", "strconv", "strings", "structs", "sync", "syscall",
	"testing", "time", "unicode", "unique", "unsafe", "uuid", "weak",
}

// isStdPackage reports whether path, which has no slash, is a standard
// library package: one listed, or one GOROOT holds, for a package newer than
// the list.
func isStdPackage(path string) bool {
	if slices.Contains(stdTopLevel, path) {
		return true
	}
	pkg, err := build.Default.Import(path, "", build.FindOnly)
	return err == nil && pkg.Goroot
}

func parseSpec(s string) Spec {
	spec := Spec{}

	lastDot := strings.LastIndex(s, ".")
	if lastDot == -1 {
		spec.FuncName = s
		return spec
	}

	spec.FuncName = s[lastDot+1:]
	prefix := s[:lastDot]

	// Check if there's another dot (indicating Type.Method)
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
func (s Spec) FullName() string {
	shortPkg := shortPkgName(s.PkgPath)
	if s.TypeName != "" {
		return shortPkg + "." + s.TypeName + "." + s.FuncName
	}
	return shortPkg + "." + s.FuncName
}

// Matches checks if a types.Func matches this specification.
func (s Spec) Matches(fn *types.Func) bool {
	if fn.Name() != s.FuncName {
		return false
	}

	pkg := fn.Pkg()
	if pkg == nil || !typeutil.PkgPathMatches(pkg.Path(), s.PkgPath) {
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

// ExtractFunc extracts the types.Func from a call expression. Explicit type
// arguments are looked through, and a function or method of a generic type
// is given as its generic origin.
func ExtractFunc(pass *analysis.Pass, call *ast.CallExpr) *types.Func {
	if fn := extractCalledFunc(pass, call); fn != nil {
		return fn.Origin()
	}
	return nil
}

func extractCalledFunc(pass *analysis.Pass, call *ast.CallExpr) *types.Func {
	fun := ast.Unparen(call.Fun)
	switch index := fun.(type) {
	case *ast.IndexExpr:
		fun = index.X
	case *ast.IndexListExpr:
		fun = index.X
	}

	switch fun := fun.(type) {
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

// shortPkgName returns the last component of a package path.
func shortPkgName(pkgPath string) string {
	if idx := strings.LastIndex(pkgPath, "/"); idx >= 0 {
		return pkgPath[idx+1:]
	}
	return pkgPath
}
