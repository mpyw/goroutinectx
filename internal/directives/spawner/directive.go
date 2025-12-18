// Package spawner handles //goroutinectx:spawner directives and -external-spawner flag.
package spawner

import (
	"go/ast"
	"go/types"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

// FuncSpec holds parsed components of a spawner function specification.
type FuncSpec struct {
	PkgPath  string
	TypeName string // empty for package-level functions
	FuncName string
}

// Map tracks functions marked with //goroutinectx:spawner.
// These functions are expected to spawn goroutines with their func arguments.
type Map struct {
	local    map[*types.Func]struct{} // from directives
	external []FuncSpec               // from -external-spawner flag
}

// IsSpawner checks if a function is marked as a spawner.
func (m *Map) IsSpawner(fn *types.Func) bool {
	if m == nil {
		return false
	}

	// Check local map first (directive-based)
	if _, ok := m.local[fn]; ok {
		return true
	}

	// Check external specs (flag-based)
	return m.matchesExternal(fn)
}

// Len returns the total number of spawners (local + external).
func (m *Map) Len() int {
	if m == nil {
		return 0
	}

	return len(m.local) + len(m.external)
}

// matchesExternal checks if fn matches any external spec.
func (m *Map) matchesExternal(fn *types.Func) bool {
	for _, spec := range m.external {
		if matchesSpec(fn, spec) {
			return true
		}
	}

	return false
}

// matchesSpec checks if a function matches a FuncSpec.
func matchesSpec(fn *types.Func, spec FuncSpec) bool {
	if fn.Name() != spec.FuncName {
		return false
	}

	pkg := fn.Pkg()
	if pkg == nil || pkg.Path() != spec.PkgPath {
		return false
	}

	// Check if it's a method
	sig := fn.Type().(*types.Signature)
	recv := sig.Recv()

	if spec.TypeName == "" {
		// Package-level function: should have no receiver
		return recv == nil
	}

	// Method: should have receiver of correct type
	if recv == nil {
		return false
	}

	recvType := recv.Type()
	// Handle pointer receivers
	if ptr, ok := recvType.(*types.Pointer); ok {
		recvType = ptr.Elem()
	}

	named, ok := recvType.(*types.Named)
	if !ok {
		return false
	}

	return named.Obj().Name() == spec.TypeName
}

// Build scans files for functions marked with the directive
// and parses the external spawner flag.
func Build(pass *analysis.Pass, externalSpawners string) *Map {
	m := &Map{
		local:    make(map[*types.Func]struct{}),
		external: parseExternal(externalSpawners),
	}

	for _, file := range pass.Files {
		buildSpawnersForFile(pass, file, m.local)
	}

	return m
}

// parseExternal parses the -external-spawner flag value.
// Format: comma-separated list of "pkg/path.Func" or "pkg/path.Type.Method".
func parseExternal(s string) []FuncSpec {
	if s == "" {
		return nil
	}

	var specs []FuncSpec

	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		spec := parseFunc(part)
		specs = append(specs, spec)
	}

	return specs
}

// parseFunc parses a single spawner function string into components.
// Format: "pkg/path.Func" or "pkg/path.Type.Method".
func parseFunc(s string) FuncSpec {
	spec := FuncSpec{}

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

// buildSpawnersForFile scans a single file for spawner directives.
func buildSpawnersForFile(pass *analysis.Pass, file *ast.File, m map[*types.Func]struct{}) {
	// Build a map of line -> comment for quick lookup
	lineComments := make(map[int]string)

	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if isSpawnerComment(c.Text) {
				line := pass.Fset.Position(c.Pos()).Line
				lineComments[line] = c.Text
			}
		}
	}

	// Find function declarations that have the directive on the previous line
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		funcLine := pass.Fset.Position(funcDecl.Pos()).Line

		// Check if directive is on previous line
		if _, hasDirective := lineComments[funcLine-1]; !hasDirective {
			continue
		}

		// Get the types.Func for this declaration
		obj := pass.TypesInfo.ObjectOf(funcDecl.Name)
		if obj == nil {
			continue
		}

		fn, ok := obj.(*types.Func)
		if !ok {
			continue
		}

		m[fn] = struct{}{}
	}
}

// isSpawnerComment checks if a comment is a spawner directive.
func isSpawnerComment(text string) bool {
	text = strings.TrimPrefix(text, "//")
	text = strings.TrimSpace(text)

	return strings.HasPrefix(text, "goroutinectx:spawner")
}

// GetFuncFromCall extracts the *types.Func from a call expression if possible.
// Returns nil if the callee cannot be determined statically.
func GetFuncFromCall(pass *analysis.Pass, call *ast.CallExpr) *types.Func {
	var ident *ast.Ident

	switch fun := call.Fun.(type) {
	case *ast.Ident:
		ident = fun
	case *ast.SelectorExpr:
		ident = fun.Sel
	default:
		return nil
	}

	obj := pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return nil
	}

	fn, ok := obj.(*types.Func)
	if !ok {
		return nil
	}

	return fn
}

// FindFuncArgs finds all arguments in a call that are func types.
// Returns the indices and the arguments themselves.
func FindFuncArgs(pass *analysis.Pass, call *ast.CallExpr) []ast.Expr {
	var funcArgs []ast.Expr

	for _, arg := range call.Args {
		tv, ok := pass.TypesInfo.Types[arg]
		if !ok {
			continue
		}

		// Check if argument is a function type
		if _, isFunc := tv.Type.Underlying().(*types.Signature); isFunc {
			funcArgs = append(funcArgs, arg)
		}
	}

	return funcArgs
}
