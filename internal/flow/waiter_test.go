package flow

import (
	"fmt"
	"strings"
	"testing"
	"time"

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

// waiterSendersSrc gives a function that starts n goroutines, each waiting
// and then sending on one channel with a buffer of size. Each go statement
// is in its own if when branched, so that the sites lie on one path.
func waiterSendersSrc(n, size int, branched bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package p\n\nimport \"sync\"\n\nfunc f(c []bool) {\n\tvar wg sync.WaitGroup\n\terrCh := make(chan error, %d)\n", size)
	for i := range n {
		if branched {
			fmt.Fprintf(&b, "\tif c[%d] {\n\t\tgo func() { wg.Wait(); errCh <- nil }()\n\t}\n", i)
		} else {
			b.WriteString("\tgo func() { wg.Wait(); errCh <- nil }()\n")
		}
	}
	b.WriteString("\t<-errCh\n}\n")
	return b.String()
}

// waiterSendersCount gives how many of the goroutines f starts are waiters,
// asking one Flow, as one pass does.
func waiterSendersCount(pkg *ssa.Package) int {
	f := NewFlow()
	waiters := 0
	for _, fn := range pkg.Func("f").AnonFuncs {
		if f.IsWaiter(fn) {
			waiters++
		}
	}
	return waiters
}

// TestWaiterManySenders checks that the sends of many goroutines on one
// channel are counted fast, once for the channel, and still right: every
// goroutine fits a buffer as large as their number, and none fits one a
// send smaller (#366). Each goroutine used to count every sender again,
// comparing each two, which took tens of seconds for a few hundred.
func TestWaiterManySenders(t *testing.T) {
	t.Parallel()

	const n = 400
	for _, tt := range []struct {
		name     string
		size     int
		branched bool
		want     int
	}{
		{name: "a buffer for each", size: n, want: n},
		{name: "one slot short", size: n - 1, want: 0},
		{name: "a buffer of one", size: 1, want: 0},
		{name: "branched, a buffer for each", size: n, branched: true, want: n},
		{name: "branched, one slot short", size: n - 1, branched: true, want: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pkg := buildClosurePackage(t, waiterSendersSrc(n, tt.size, tt.branched))
			start := time.Now()
			if got := waiterSendersCount(pkg); got != tt.want {
				t.Errorf("%d of %d goroutines are waiters, want %d", got, n, tt.want)
			}
			if took := time.Since(start); took > 30*time.Second {
				t.Errorf("the waiters of %d senders took %v", n, took)
			}
		})
	}
}

// BenchmarkWaiterManySenders measures the waiters of 400 goroutines, each
// in its own if, that send on one channel with a slot for each (#366).
func BenchmarkWaiterManySenders(b *testing.B) {
	pkg := buildClosurePackage(b, waiterSendersSrc(400, 400, true))
	b.ResetTimer()
	for b.Loop() {
		waiterSendersCount(pkg)
	}
}
