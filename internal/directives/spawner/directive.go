// Package spawner handles //goroutinectx:spawner directives and -external-spawner flag.
package spawner

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/funcspec"
)

// Map tracks functions marked with //goroutinectx:spawner.
// These functions are expected to spawn goroutines with their func arguments.
type Map struct {
	local    map[*types.Func]struct{} // from directives
	external []funcspec.Spec          // from -external-spawner flag
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
		if spec.Matches(fn) {
			return true
		}
	}

	return false
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
func parseExternal(s string) []funcspec.Spec {
	if s == "" {
		return nil
	}

	var specs []funcspec.Spec

	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		spec := funcspec.Parse(part)
		specs = append(specs, spec)
	}

	return specs
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
	return funcspec.ExtractFunc(pass, call)
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
