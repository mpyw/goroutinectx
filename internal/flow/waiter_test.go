package flow

import (
	"testing"

	"golang.org/x/tools/go/ssa"
)

// waiterLiftSrc has two functions, a closure that captures a variable, and
// a func literal of a package variable, which captures nothing.
const waiterLiftSrc = `package p

func a() {}

func b() {}

func h(n int) {
	f := func() { println(n) }
	f()
}

var v = func() { println() }
`

// TestWaiterLiftStops checks that sites stop being lifted where no site
// runs their closure: sites of two functions never meet, so they may both
// run (#354).
func TestWaiterLiftStops(t *testing.T) {
	t.Parallel()

	pkg := buildClosurePackage(t, waiterLiftSrc)
	first := func(fn *ssa.Function) ssa.Instruction { return fn.Blocks[0].Instrs[0] }
	sited := make(map[*ssa.MakeClosure]bool)

	if waiterApart(first(pkg.Func("a")), first(pkg.Func("b")), sited) {
		t.Error("sites of two functions are taken as alternatives")
	}

	closure := pkg.Func("h").AnonFuncs[0]
	if waiterLift(first(closure), sited) == nil {
		t.Error("a closure called in place has no site")
	}
	sited[waiterMakerOf(closure)] = true
	if waiterLift(first(closure), sited) != nil {
		t.Error("a closure whose site is being looked for has a site")
	}

	literal := pkg.Func("init").AnonFuncs[0]
	if waiterMakerOf(literal) != nil {
		t.Error("a func literal that captures nothing has a closure")
	}
}
