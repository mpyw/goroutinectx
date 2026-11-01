package spawner

import (
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/directive"
	"github.com/mpyw/goroutinectx/internal/funcspec"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Fact marks a function of an imported package as a spawner. The package
// that declares the function exports it, so that //goroutinectx:spawner
// applies to calls from other packages too.
type Fact struct{}

// AFact makes Fact an analysis.Fact.
func (*Fact) AFact() {}

func (*Fact) String() string { return "spawner" }

// Analyzer finds the functions marked with //goroutinectx:spawner. It
// exports a [Fact] for each exported one, and gives the marked functions of
// its package and of the packages it imports as a [*Marked].
//
// It reads only comments, so it is cheap. It is a separate analyzer so that
// the facts, which the driver computes for every dependency, do not make
// goroutinectx build SSA for every dependency too.
var Analyzer = &analysis.Analyzer{
	Name:       "goroutinectxspawner",
	Doc:        "finds the functions marked with //goroutinectx:spawner",
	Run:        run,
	FactTypes:  []analysis.Fact{new(Fact)},
	ResultType: reflect.TypeFor[*Marked](),
}

// Marked holds the functions marked with //goroutinectx:spawner, in a
// package and in the packages it imports.
type Marked struct {
	funcs map[*types.Func]struct{}
}

func run(pass *analysis.Pass) (any, error) {
	m := &Marked{funcs: make(map[*types.Func]struct{})}
	for _, file := range pass.Files {
		buildForFile(pass, file, m.funcs)
	}
	// Only an exported function can be called from another package.
	for fn := range m.funcs {
		if fn.Exported() {
			pass.ExportObjectFact(fn, new(Fact))
		}
	}
	for _, f := range pass.AllObjectFacts() {
		if fn, ok := f.Object.(*types.Func); ok {
			m.funcs[fn] = struct{}{}
		}
	}
	return m, nil
}

// Map tracks functions marked with //goroutinectx:spawner.
type Map struct {
	marked   *Marked
	external []funcspec.Spec
}

// IsSpawner checks if a function is marked as a spawner: by a directive in
// this package or an imported one, or by -external-spawner.
func (m *Map) IsSpawner(fn *types.Func) bool {
	if m == nil {
		return false
	}
	fn = fn.Origin()

	if _, ok := m.marked.funcs[fn]; ok {
		return true
	}

	return m.matchesExternal(fn)
}

// HasDirective reports whether fn is marked by a //goroutinectx:spawner
// directive, in this package or an imported one. Unlike [Map.IsSpawner], a
// match of -external-spawner does not count.
func (m *Map) HasDirective(fn *types.Func) bool {
	if m == nil {
		return false
	}
	_, ok := m.marked.funcs[fn.Origin()]
	return ok
}

// matchesExternal checks if fn matches any external spec.
func (m *Map) matchesExternal(fn *types.Func) bool {
	return slices.ContainsFunc(m.external, func(spec funcspec.Spec) bool { return spec.Matches(fn) })
}

// Build gives the spawners of a package: the functions [Analyzer] found
// marked, and those external, the specs of -external-spawner, from
// [ParseExternal], match.
func Build(marked *Marked, external []funcspec.Spec) *Map {
	return &Map{marked: marked, external: external}
}

// ParseExternal parses the -external-spawner list. A blank entry, an entry
// that holds a comma, and a specification funcspec.Parse rejects are errors.
func ParseExternal(parts []string) ([]funcspec.Spec, error) {
	var specs []funcspec.Spec
	for i, part := range parts {
		if err := typeutil.CheckItem("-external-spawner", i, part, "function"); err != nil {
			return nil, err
		}
		part = strings.TrimSpace(part)
		spec, err := funcspec.Parse(part)
		if err != nil {
			return nil, fmt.Errorf("-external-spawner: %w", err)
		}
		specs = append(specs, spec)
	}

	return specs, nil
}

// buildForFile adds the functions of a file whose doc comment holds a
// spawner directive. The directive may sit on any line of it.
func buildForFile(pass *analysis.Pass, file *ast.File, m map[*types.Func]struct{}) {
	for _, funcDecl := range directive.Scan(file).Spawners {
		if fn, ok := pass.TypesInfo.ObjectOf(funcDecl.Name).(*types.Func); ok {
			m[fn] = struct{}{}
		}
	}
}
