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
type Map struct {
	local    map[*types.Func]struct{}
	external []funcspec.Spec
}

// IsSpawner checks if a function is marked as a spawner.
func (m *Map) IsSpawner(fn *types.Func) bool {
	if m == nil {
		return false
	}

	if _, ok := m.local[fn]; ok {
		return true
	}

	return m.matchesExternal(fn)
}

// Len returns the total number of spawners.
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
		buildForFile(pass, file, m.local)
	}

	return m
}

// parseExternal parses the -external-spawner flag value.
func parseExternal(s string) []funcspec.Spec {
	if s == "" {
		return nil
	}

	var specs []funcspec.Spec
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		specs = append(specs, funcspec.Parse(part))
	}

	return specs
}

// buildForFile scans a single file for spawner directives.
func buildForFile(pass *analysis.Pass, file *ast.File, m map[*types.Func]struct{}) {
	lineComments := make(map[int]string)

	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if isSpawnerComment(c.Text) {
				line := pass.Fset.Position(c.Pos()).Line
				lineComments[line] = c.Text
			}
		}
	}

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		funcLine := pass.Fset.Position(funcDecl.Pos()).Line
		if _, hasDirective := lineComments[funcLine-1]; !hasDirective {
			continue
		}

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
