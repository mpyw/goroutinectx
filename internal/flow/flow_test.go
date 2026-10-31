package flow

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// sourcesSrc gives sinkFunc a value of each kind of source, and the
// other sinks a global, a function of another package and a constant.
const sourcesSrc = `package p

import "strings"

type T struct{}

func (T) M() {}

func Named() {}

var G func()

func one() func() { return nil }

func two() (func(), error) { return nil, nil }

func sinkFunc(func())             {}
func sinkGlobal(*func())          {}
func sinkExternal(func(string) string) {}
func sinkInt(int)                 {}

func use(param func(), n int) {
	f := func() {}
	switch n {
	case 0:
		f = func() { _ = n }
	case 1:
		f = Named
	case 2:
		f = T{}.M
	case 3:
		f = one()
	case 4:
		f, _ = two()
	case 5:
		f = param
	}
	sinkFunc(f)
	sinkFunc(G)
	sinkGlobal(&G)
	sinkExternal(strings.ToUpper)
	sinkInt(n + 1)
}
`

// sourcesArgs builds sourcesSrc, and gives the argument of each call in use,
// in order.
func sourcesArgs(t *testing.T) []ssa.Value {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", sourcesSrc, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := &types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, _, err := ssautil.BuildPackage(conf, fset, types.NewPackage("p", "p"), []*ast.File{f}, ssa.SanityCheckFunctions)
	if err != nil {
		t.Fatal(err)
	}
	var args []ssa.Value
	for _, block := range pkg.Func("use").Blocks {
		for _, instr := range block.Instrs {
			if call, ok := instr.(*ssa.Call); ok && len(call.Call.Args) == 1 {
				args = append(args, call.Call.Args[0])
			}
		}
	}
	return args
}

// TestFlowSources checks the kind of each source Sources gives, and that a
// load it cannot follow leaves the list not complete.
func TestFlowSources(t *testing.T) {
	t.Parallel()

	args := sourcesArgs(t)
	if len(args) < 5 {
		t.Fatalf("found %d calls, want 5", len(args))
	}
	f := NewFlow()
	tests := []struct {
		name     string
		arg      ssa.Value
		want     []SourceKind
		complete bool
	}{
		{"local", args[0], []SourceKind{SourceLiteral, SourceLiteral, SourceFunc, SourceMethod, SourceCall, SourceCall, SourceParam}, true},
		{"loaded global", args[1], nil, false},
		{"global", args[2], []SourceKind{SourceGlobal}, true},
		{"external", args[3], []SourceKind{SourceFunc}, true},
		{"constant", args[4], []SourceKind{SourceUnknown}, true},
	}
	for _, tt := range tests {
		sources, complete := f.Sources(tt.arg)
		var got []SourceKind
		for _, s := range sources {
			got = append(got, s.Kind)
		}
		slices.Sort(got)
		if !slices.Equal(got, tt.want) || complete != tt.complete {
			t.Errorf("%s: Sources = %v, %v, want %v, %v", tt.name, got, complete, tt.want, tt.complete)
		}
	}

	// A second question is answered from what the first found.
	again, _ := f.Sources(args[0])
	if first, _ := f.Sources(args[0]); len(again) != len(first) {
		t.Errorf("Sources again gave %d sources, want %d", len(again), len(first))
	}
}

// TestSourceCall checks that Call gives the call of a result, of a call
// with one result or several, and nil for any other source.
func TestSourceCall(t *testing.T) {
	t.Parallel()

	sources, _ := NewFlow().Sources(sourcesArgs(t)[0])
	calls := 0
	for _, s := range sources {
		got := s.Call()
		if (got != nil) != (s.Kind == SourceCall) {
			t.Errorf("Call of %v (kind %d) = %v", s.Value, s.Kind, got)
		}
		if got != nil {
			calls++
		}
	}
	if calls != 2 {
		t.Errorf("found %d calls, want 2", calls)
	}
}

// capturedSrc has a closure in use read a variable it captures, parts
// reads a field and elements of values, and makers returns a function.
const capturedSrc = `package p

type T struct{ f func() }

func a() {}
func b() {}

func makers(n int) func() {
	if n > 0 {
		return a
	}
	return b
}

func use(n int) {
	v := a
	if n > 0 {
		v = b
	}
	func() { v() }()
	v = nil
}

func parts(n int) {
	T{f: b}.f()
	[2]func(){a, b}[1]()
	[2]func(){a, b}[n]()
}
`

// capturedPkg builds capturedSrc.
func capturedPkg(t *testing.T) *ssa.Package {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", capturedSrc, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, _, err := ssautil.BuildPackage(&types.Config{}, fset, types.NewPackage("p", "p"), []*ast.File{f}, ssa.SanityCheckFunctions)
	if err != nil {
		t.Fatal(err)
	}
	return pkg
}

// capturedCallees gives the value each call in fn calls, in order.
func capturedCallees(fn *ssa.Function) []ssa.Value {
	var callees []ssa.Value
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if call, ok := instr.(*ssa.Call); ok {
				callees = append(callees, call.Call.Value)
			}
		}
	}
	return callees
}

// TestFlowCapturedSources checks that CapturedSources reads a variable a
// closure captures by every value stored to it, a field or an element at a
// constant index by that part, and an element at any other index not at all.
func TestFlowCapturedSources(t *testing.T) {
	t.Parallel()

	pkg := capturedPkg(t)
	closure := pkg.Func("use").AnonFuncs[0]
	loaded := capturedCallees(closure)
	parts := capturedCallees(pkg.Func("parts"))
	if len(loaded) != 1 || len(parts) != 3 {
		t.Fatalf("found %d and %d calls, want 1 and 3", len(loaded), len(parts))
	}
	f := NewFlow()
	b := pkg.Func("b")
	tests := []struct {
		name     string
		v        ssa.Value
		want     []ssa.Value
		complete bool
	}{
		// Every store counts, the one after the closure is made too.
		{"variable", closure.FreeVars[0], []ssa.Value{pkg.Func("a"), b, nil}, true},
		{"loaded", loaded[0], []ssa.Value{loaded[0]}, true},
		{"field", parts[0], []ssa.Value{b}, true},
		{"element", parts[1], []ssa.Value{b}, true},
		{"element at a variable index", parts[2], nil, false},
	}
	for _, tt := range tests {
		sources, complete := f.CapturedSources(tt.v)
		got := make([]ssa.Value, len(sources))
		for i, s := range sources {
			got[i] = s.Value
			if c, ok := s.Value.(*ssa.Const); ok && c.IsNil() {
				got[i] = nil
			}
		}
		if !slices.Equal(got, tt.want) || complete != tt.complete {
			t.Errorf("%s: CapturedSources = %v, %v, want %v, %v", tt.name, got, complete, tt.want, tt.complete)
		}
	}
}

// TestFlowReturns checks that Returns gives each value a function returns,
// with its sources, and nothing for a function with no body.
func TestFlowReturns(t *testing.T) {
	t.Parallel()

	pkg := capturedPkg(t)
	f := NewFlow()
	returned := f.Returns(pkg.Func("makers"))
	if len(returned) != 2 {
		t.Fatalf("Returns gave %d values, want 2", len(returned))
	}
	for _, ret := range returned {
		if len(ret.Sources) != 1 || ret.Sources[0].Kind != SourceFunc || ret.Sources[0].Value != ret.Value {
			t.Errorf("Returns gave %v with sources %v, want the function itself", ret.Value, ret.Sources)
		}
	}
	if got := f.Returns(&ssa.Function{}); len(got) != 0 {
		t.Errorf("Returns of a function with no body gave %v", got)
	}
}
