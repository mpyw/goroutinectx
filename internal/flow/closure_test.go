package flow

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/mpyw/goroutinectx/internal/ctxkind"
)

// closureParamSrc runs a func parameter in a closure, which captures
// nothing else.
const closureParamSrc = `package p

func run(fn func()) {
	go func() { fn() }()
}
`

// TestClosureCapturesContextFuncs checks that a func value a closure runs
// counts only through the resolver it is given: with none, a parameter the
// closure calls gives it nothing.
func TestClosureCapturesContextFuncs(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", closureParamSrc, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := &types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, _, err := ssautil.BuildPackage(conf, fset, types.NewPackage("p", "p"), []*ast.File{f}, ssa.SanityCheckFunctions)
	if err != nil {
		t.Fatal(err)
	}
	closure := pkg.Func("run").AnonFuncs[0]
	kinds := ctxkind.New(pkg.Prog, nil, "")

	flow := NewFlow()
	if flow.ClosureCapturesContext(closure, kinds, nil) {
		t.Error("with no resolver, the closure counts as capturing a context")
	}
	r := &FuncResolver{Flow: flow, Closure: func(*ssa.Function) bool { return false }}
	if !flow.ClosureCapturesContext(closure, kinds, r) {
		t.Error("with a resolver, the parameter the closure calls does not pass")
	}
}
