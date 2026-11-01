package flow

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/mpyw/goroutinectx/internal/ctxkind"
)

// closurePolicy judges a function literal by literal, and leaves every
// call to the built-in rules.
type closurePolicy struct {
	literal func(fn *ssa.Function) Verdict
}

func (p *closurePolicy) Literal(fn *ssa.Function, _ Site) Verdict { return p.literal(fn) }

func (*closurePolicy) Call(*ssa.CallCommon) Verdict { return Unknown }

func (*closurePolicy) FactoryCall(*ssa.CallCommon) bool { return false }

func (*closurePolicy) Factory(*ssa.Function) bool { return false }

// closureParamSrc runs a func parameter in a closure, which captures
// nothing else.
const closureParamSrc = `package p

func run(fn func()) {
	go func() { fn() }()
}
`

// TestClosureCapturesContextFuncs checks that a func value a closure runs
// counts only through the policy it is given: with none, a parameter the
// closure calls gives it nothing.
func TestClosureCapturesContextFuncs(t *testing.T) {
	t.Parallel()

	pkg := buildClosurePackage(t, closureParamSrc)
	closure := pkg.Func("run").AnonFuncs[0]
	kinds := ctxkind.New(pkg.Prog, nil, "")

	flow := NewFlow()
	if flow.ClosureCapturesContext(closure, kinds, nil) {
		t.Error("with no policy, the closure counts as capturing a context")
	}
	p := &closurePolicy{literal: func(*ssa.Function) Verdict { return Fail }}
	if !flow.ClosureCapturesContext(closure, kinds, p) {
		t.Error("with a policy, the parameter the closure calls does not pass")
	}
}

// closureCycleSrc has two closures that run each other: a runs bf through
// two variables, and bf runs a. Only use captures ctx.
const closureCycleSrc = `package p

import "context"

func h(ctx context.Context) {
	var a, b, c func()
	bf := func() { a() }
	b, c = bf, bf
	a = func() { b(); c() }
	use := func() { _ = ctx; a() }
	go a()
	go use()
}
`

// TestClosureCapturesContextCycle checks that closures that run each other
// are judged the same wherever the question starts, though a verdict cut
// at a closure further up the stack is not remembered (#239).
func TestClosureCapturesContextCycle(t *testing.T) {
	t.Parallel()

	pkg := buildClosurePackage(t, closureCycleSrc)
	kinds := ctxkind.New(pkg.Prog, nil, "")
	anon := pkg.Func("h").AnonFuncs
	bf, a, use := anon[0], anon[1], anon[2]

	flow := NewFlow()
	if flow.ClosureCapturesContext(nil, kinds, nil) {
		t.Error("no closure captures a context")
	}
	for _, fn := range []*ssa.Function{a, bf, a, bf} {
		if flow.ClosureCapturesContext(fn, kinds, nil) {
			t.Errorf("%s captures a context, but neither closure of the cycle captures one", fn.Name())
		}
	}
	if !flow.ClosureCapturesContext(use, kinds, nil) {
		t.Error("use does not capture the context it reads")
	}
}

// TestClosureChainIsLinear checks that a chain of closures that call each
// other is judged in time linear in its length: each closure is judged
// once, and the policy is asked about each a few times at most. Before the
// verdicts were remembered (#239), 16 closures took about 55s, and 10 that
// each capture the one before twice took about 13s.
func TestClosureChainIsLinear(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		n        int
		captures int
	}{
		{name: "single", n: 40, captures: 1},
		{name: "double", n: 20, captures: 2},
		{name: "triple", n: 20, captures: 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pkg := buildClosurePackage(t, closureChainSource(tt.n, tt.captures))
			kinds := ctxkind.New(pkg.Prog, nil, "")
			flow := NewFlow()
			asked := 0
			p := &closurePolicy{}
			p.literal = func(fn *ssa.Function) Verdict {
				asked++
				if flow.ClosureCapturesContext(fn, kinds, p) {
					return Pass
				}
				return Fail
			}
			h := pkg.Func("h")
			spawned := h.AnonFuncs[len(h.AnonFuncs)-1]
			if flow.ClosureCapturesContext(spawned, kinds, p) {
				t.Error("the chain captures a context, but none of its closures captures one")
			}
			if limit := 4 * tt.n * tt.captures; asked > limit {
				t.Errorf("the policy judged a closure %d times, want at most %d", asked, limit)
			}
		})
	}
}

// closureChainSource gives a function whose closure f<i> calls f<i-1>, n
// times over, through captures variables that each hold f<i-1>, and a
// goroutine that runs the last.
func closureChainSource(n, captures int) string {
	var src strings.Builder
	src.WriteString("package p\n\nimport \"context\"\n\nfunc h(ctx context.Context) {\n\tf0 := func() {}\n")
	for i := 1; i <= n; i++ {
		var calls []string
		for c := range captures {
			fmt.Fprintf(&src, "\tv%d_%d := f%d\n", i, c, i-1)
			calls = append(calls, fmt.Sprintf("v%d_%d()", i, c))
		}
		fmt.Fprintf(&src, "\tf%d := func() { %s }\n", i, strings.Join(calls, "; "))
	}
	fmt.Fprintf(&src, "\tgo func() { f%d() }()\n}\n", n)
	return src.String()
}

// buildClosurePackage builds src as the package p, with its SSA.
func buildClosurePackage(t *testing.T, src string) *ssa.Package {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := &types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, _, err := ssautil.BuildPackage(conf, fset, types.NewPackage("p", "p"), []*ast.File{f}, ssa.SanityCheckFunctions)
	if err != nil {
		t.Fatal(err)
	}
	return pkg
}
