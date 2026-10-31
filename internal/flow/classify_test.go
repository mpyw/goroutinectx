package flow

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// classifySrc has a function of each kind Classify tells apart.
const classifySrc = `package p

import "strings"

type T struct{}

func (T) M() {}

type E struct{ T }

func Named() {}

func Generic[X any]() func() { return func() {} }

func seq(yield func() bool) {}

func use() {
	lit := func() {}
	lit()
	_ = T{}.M
	thunk := T.M
	thunk(T{})
	_ = strings.ToUpper
	_ = Generic[int]()
	for range seq {
	}
	var i interface{ M() } = E{}
	i.M()
}
`

// classifyFuncs builds classifySrc, generic instances included, and gives
// every function of the program by its name.
func classifyFuncs(t *testing.T) map[string]*ssa.Function {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", classifySrc, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := &types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, _, err := ssautil.BuildPackage(conf, fset, types.NewPackage("p", "p"), []*ast.File{f},
		ssa.SanityCheckFunctions|ssa.InstantiateGenerics)
	if err != nil {
		t.Fatal(err)
	}
	funcs := make(map[string]*ssa.Function)
	for fn := range ssautil.AllFunctions(pkg.Prog) {
		funcs[fn.String()] = fn
	}
	return funcs
}

// TestClassify checks the kind Classify gives each function.
func TestClassify(t *testing.T) {
	t.Parallel()

	funcs := classifyFuncs(t)
	tests := []struct {
		name string
		want ClassifiedKind
	}{
		{"p.use$1", ClassifiedLiteral},
		{"p.Generic$1", ClassifiedLiteral},
		{"p.Generic[int]$1", ClassifiedLiteral},
		{"p.use$2", ClassifiedRangeBody},
		{"p.Named", ClassifiedNamed},
		{"p.Generic", ClassifiedNamed},
		{"p.Generic[int]", ClassifiedNamed},
		{"(p.T).M", ClassifiedMethod},
		{"(p.T).M$bound", ClassifiedBound},
		{"(p.T).M$thunk", ClassifiedThunk},
		{"(p.E).M", ClassifiedWrapper},
		{"strings.ToUpper", ClassifiedExternal},
		{"p.init", ClassifiedOther},
	}
	for _, tt := range tests {
		fn := funcs[tt.name]
		if fn == nil {
			var names []string
			for name := range funcs {
				if strings.HasPrefix(name, "p.") || strings.HasPrefix(name, "(p.") {
					names = append(names, name)
				}
			}
			t.Fatalf("no function %s in %v", tt.name, names)
		}
		if got := Classify(fn); got != tt.want {
			t.Errorf("Classify(%s) = %d, want %d", tt.name, got, tt.want)
		}
	}
	if got := Classify(nil); got != ClassifiedOther {
		t.Errorf("Classify(nil) = %d, want %d", got, ClassifiedOther)
	}
}
