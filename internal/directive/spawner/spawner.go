package spawner

import (
	"go/ast"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/directive"
	"github.com/mpyw/goroutinectx/internal/funcspec"
)

// Fact marks a function of an imported package as a spawner. The package
// that declares the function exports it, so that //goroutinectx:spawner
// applies to calls from other packages too.
type Fact struct{}

// AFact makes Fact an analysis.Fact.
func (*Fact) AFact() {}

func (*Fact) String() string { return "spawner" }

// Map tracks functions marked with //goroutinectx:spawner.
type Map struct {
	pass     *analysis.Pass
	local    map[*types.Func]struct{}
	external []funcspec.Spec
}

// IsSpawner checks if a function is marked as a spawner: by a directive in
// this package or an imported one, or by -external-spawner.
func (m *Map) IsSpawner(fn *types.Func) bool {
	if m == nil {
		return false
	}
	fn = fn.Origin()

	if _, ok := m.local[fn]; ok {
		return true
	}

	if fn.Pkg() != nil && fn.Pkg() != m.pass.Pkg && m.pass.ImportObjectFact(fn, new(Fact)) {
		return true
	}

	return m.matchesExternal(fn)
}

// matchesExternal checks if fn matches any external spec.
func (m *Map) matchesExternal(fn *types.Func) bool {
	return slices.ContainsFunc(m.external, func(spec funcspec.Spec) bool { return spec.Matches(fn) })
}

// Build scans files for functions marked with the directive
// and parses the external spawner flag.
func Build(pass *analysis.Pass, externalSpawners []string) *Map {
	m := &Map{
		pass:     pass,
		local:    make(map[*types.Func]struct{}),
		external: parseExternal(externalSpawners),
	}

	for _, file := range pass.Files {
		buildForFile(pass, file, m.local)
	}
	// Only an exported function can be called from another package.
	for fn := range m.local {
		if fn.Exported() {
			pass.ExportObjectFact(fn, new(Fact))
		}
	}

	return m
}

// parseExternal parses the -external-spawner list. Blank entries are skipped.
func parseExternal(parts []string) []funcspec.Spec {
	var specs []funcspec.Spec
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		specs = append(specs, funcspec.Parse(part))
	}

	return specs
}

// buildForFile scans a single file for functions whose doc comment holds a
// spawner directive. The directive may sit on any line of it.
func buildForFile(pass *analysis.Pass, file *ast.File, m map[*types.Func]struct{}) {
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Doc == nil {
			continue
		}

		if !slices.ContainsFunc(funcDecl.Doc.List, func(c *ast.Comment) bool { return isSpawnerComment(c.Text) }) {
			continue
		}

		fn, ok := pass.TypesInfo.ObjectOf(funcDecl.Name).(*types.Func)
		if !ok {
			continue
		}

		m[fn] = struct{}{}
	}
}

// isSpawnerComment checks if a comment is a spawner directive.
func isSpawnerComment(text string) bool {
	d, ok := directive.Parse(text)
	return ok && d.Name == "spawner"
}
