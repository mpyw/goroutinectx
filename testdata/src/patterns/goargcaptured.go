// A go statement or a spawn inside a closure or a range-over-func body,
// given a value the closure captures (#269).

package patterns

import (
	"context"
	"slices"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ===== Captured value given to a go statement =====

// [GOOD]: Captured func given to a goroutine in a deferred closure
//
// The func the goroutine runs captures ctx, as it does outside the closure.
func goodGoArgCapturedFuncInDefer(ctx context.Context) {
	f := func() { use(ctx) }
	defer func() {
		go func(g func()) { g() }(f)
	}()
}

// [BAD]: Captured func without ctx given to a goroutine in a deferred closure
//
// The func the goroutine runs does not use ctx.
func badGoArgCapturedFuncInDefer(ctx context.Context) {
	f := func() {}
	defer func() {
		go func(g func()) { g() }(f) // want `goroutine does not propagate context "ctx"`
	}()
}

// [GOOD]: Captured func given to a goroutine in a range-over-func body
//
// The loop body is a closure, and the func it gives captures ctx.
func goodGoArgCapturedFuncInRangeFunc(ctx context.Context, xs []int) {
	f := func() { use(ctx) }
	for range slices.Values(xs) {
		go func(g func()) { g() }(f)
	}
}

// [BAD]: Captured func without ctx given to a goroutine in a range-over-func body
//
// The func the goroutine runs does not use ctx.
func badGoArgCapturedFuncInRangeFunc(ctx context.Context, xs []int) {
	f := func() {}
	for range slices.Values(xs) {
		go func(g func()) { g() }(f) // want `goroutine does not propagate context "ctx"`
	}
}

// [GOOD]: Captured any holding ctx given to a goroutine in a closure
//
// The parameter is ctx in an interface, as it is outside the closure.
func goodGoArgCapturedAnyInClosure(ctx context.Context) {
	var a any = ctx
	func() {
		go func(v any) { <-v.(context.Context).Done() }(a)
	}()
}

// [BAD]: Captured any holding a fresh context given to a goroutine in a closure
//
// The parameter holds only context.Background(), not ctx.
func badGoArgCapturedFreshAnyInClosure(ctx context.Context) {
	var a any = context.Background()
	func() {
		go func(v any) { _ = v }(a) // want `goroutine does not propagate context "ctx"`
	}()
}

// [GOOD]: Captured slice of contexts given to a goroutine in a closure
//
// The slice holds ctx.
func goodGoArgCapturedSliceInClosure(ctx context.Context) {
	cs := []context.Context{ctx}
	func() {
		go func(cs []context.Context) { <-cs[0].Done() }(cs)
	}()
}

// [GOOD]: Captured interface a call made from ctx, given to a goroutine in a closure
//
// The interface the call gives may hold what it was given.
func goodGoArgCapturedIfaceFromCallInClosure(ctx context.Context) {
	r, _ := newCtxRunnerErr(ctx)
	func() {
		go func(r runner) { r.Run() }(r)
	}()
}

// [GOOD]: Captured channel of funcs that use ctx, given to a goroutine in a closure
//
// A func sent on the channel captures ctx.
func goodGoArgCapturedChanInClosure(ctx context.Context) {
	ch := make(chan func(), 1)
	ch <- func() { use(ctx) }
	func() {
		go func(ch chan func()) { (<-ch)() }(ch)
	}()
}

// [BAD]: Captured channel of funcs without ctx, given to a goroutine in a closure
//
// No func sent on the channel captures ctx.
func badGoArgCapturedPlainChanInClosure(ctx context.Context) {
	ch := make(chan func(), 1)
	ch <- func() {}
	func() {
		go func(ch chan func()) { (<-ch)() }(ch) // want `goroutine does not propagate context "ctx"`
	}()
}

// ===== Captured channel given to a factory =====

// [GOOD]: Captured channel of funcs that use ctx, given to a factory in a closure
//
// A func sent on the channel captures ctx (#240).
func goodFuncChanCtxToFactoryInClosureGoroutine(ctx context.Context) {
	ch := make(chan func(), 1)
	ch <- func() { use(ctx) }
	func() {
		go runFromChan(ch)()
	}()
}

// [GOOD]: Captured channel of funcs that use ctx, given to a factory in a closure
//
// A func sent on the channel captures ctx (#240).
func goodFuncChanCtxToFactoryInClosureErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan func(), 1)
	ch <- func() { use(ctx) }
	func() {
		g.Go(runFromChanErr(ch))
	}()
	_ = g.Wait()
}

// [GOOD]: Captured channel of funcs that use ctx, given to a factory in a closure
//
// A func sent on the channel captures ctx (#240).
func goodFuncChanCtxToFactoryInClosureWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	ch := make(chan func(), 1)
	ch <- func() { use(ctx) }
	func() {
		wg.Go(runFromChan(ch))
	}()
	wg.Wait()
}

// [GOOD]: Captured channel of funcs that use ctx, given to a factory in a closure
//
// A func sent on the channel captures ctx (#240).
func goodFuncChanCtxToFactoryInClosureSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan func(), 1)
	ch <- func() { use(ctx) }
	func() {
		runWithGroup(g, runFromChanErr(ch))
	}()
	_ = g.Wait()
}

// [BAD]: Captured channel of funcs without ctx, given to a factory in a closure
//
// No func sent on the channel captures ctx.
func badFuncChanPlainToFactoryInClosureGoroutine(ctx context.Context) {
	ch := make(chan func(), 1)
	ch <- func() {}
	func() {
		go runFromChan(ch)() // want `goroutine does not propagate context "ctx"`
	}()
}

// [BAD]: Captured channel of funcs without ctx, given to a factory in a closure
//
// No func sent on the channel captures ctx.
func badFuncChanPlainToFactoryInClosureErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan func(), 1)
	ch <- func() {}
	func() {
		g.Go(runFromChanErr(ch)) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	}()
	_ = g.Wait()
}

// [BAD]: Captured channel of funcs without ctx, given to a factory in a closure
//
// No func sent on the channel captures ctx.
func badFuncChanPlainToFactoryInClosureWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	ch := make(chan func(), 1)
	ch <- func() {}
	func() {
		wg.Go(runFromChan(ch)) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	}()
	wg.Wait()
}

// [BAD]: Captured channel of funcs without ctx, given to a factory in a closure
//
// No func sent on the channel captures ctx.
func badFuncChanPlainToFactoryInClosureSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan func(), 1)
	ch <- func() {}
	func() {
		runWithGroup(g, runFromChanErr(ch)) // want `runWithGroup\(\) func argument should use context "ctx"`
	}()
	_ = g.Wait()
}

// ===== Captured value joined by a Phi =====

// [GOOD]: Captured func through a Phi given to a goroutine in a deferred closure
//
// Each value the Phi joins is the func that captures ctx (#297).
func goodGoArgCapturedFuncPhiInDefer(ctx context.Context) {
	f := func() { use(ctx) }
	defer func() {
		h := f
		if conditionFlag {
			h = f
		}
		go func(g func()) { g() }(h)
	}()
}

// [BAD]: Captured func without ctx through a Phi given to a goroutine in a deferred closure
//
// No value the Phi joins uses ctx.
func badGoArgCapturedFuncPhiInDefer(ctx context.Context) {
	f := func() {}
	defer func() {
		h := f
		if conditionFlag {
			h = f
		}
		go func(g func()) { g() }(h) // want `goroutine does not propagate context "ctx"`
	}()
}

// [GOOD]: Captured channel through a Phi, given to a factory in a closure
//
// A func sent on the channel captures ctx (#297).
func goodFuncChanPhiToFactoryInClosureGoroutine(ctx context.Context) {
	ch := make(chan func(), 1)
	ch <- func() { use(ctx) }
	func() {
		c := ch
		if conditionFlag {
			c = ch
		}
		go runFromChan(c)()
	}()
}

// [GOOD]: Captured channel through a Phi, given to a factory in a closure
//
// A func sent on the channel captures ctx (#297).
func goodFuncChanPhiToFactoryInClosureErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan func(), 1)
	ch <- func() { use(ctx) }
	func() {
		c := ch
		if conditionFlag {
			c = ch
		}
		g.Go(runFromChanErr(c))
	}()
	_ = g.Wait()
}

// [GOOD]: Captured channel through a Phi, given to a factory in a closure
//
// A func sent on the channel captures ctx (#297).
func goodFuncChanPhiToFactoryInClosureWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	ch := make(chan func(), 1)
	ch <- func() { use(ctx) }
	func() {
		c := ch
		if conditionFlag {
			c = ch
		}
		wg.Go(runFromChan(c))
	}()
	wg.Wait()
}

// [GOOD]: Captured channel through a Phi, given to a factory in a closure
//
// A func sent on the channel captures ctx (#297).
func goodFuncChanPhiToFactoryInClosureSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan func(), 1)
	ch <- func() { use(ctx) }
	func() {
		c := ch
		if conditionFlag {
			c = ch
		}
		runWithGroup(g, runFromChanErr(c))
	}()
	_ = g.Wait()
}

// [BAD]: Captured channel without ctx through a Phi, given to a factory in a closure
//
// No func sent on the channel captures ctx.
func badFuncChanPlainPhiToFactoryInClosureGoroutine(ctx context.Context) {
	ch := make(chan func(), 1)
	ch <- func() {}
	func() {
		c := ch
		if conditionFlag {
			c = ch
		}
		go runFromChan(c)() // want `goroutine does not propagate context "ctx"`
	}()
}

// [BAD]: Captured channel without ctx through a Phi, given to a factory in a closure
//
// No func sent on the channel captures ctx.
func badFuncChanPlainPhiToFactoryInClosureErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan func(), 1)
	ch <- func() {}
	func() {
		c := ch
		if conditionFlag {
			c = ch
		}
		g.Go(runFromChanErr(c)) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	}()
	_ = g.Wait()
}

// [BAD]: Captured channel without ctx through a Phi, given to a factory in a closure
//
// No func sent on the channel captures ctx.
func badFuncChanPlainPhiToFactoryInClosureWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	ch := make(chan func(), 1)
	ch <- func() {}
	func() {
		c := ch
		if conditionFlag {
			c = ch
		}
		wg.Go(runFromChan(c)) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	}()
	wg.Wait()
}

// [BAD]: Captured channel without ctx through a Phi, given to a factory in a closure
//
// No func sent on the channel captures ctx.
func badFuncChanPlainPhiToFactoryInClosureSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan func(), 1)
	ch <- func() {}
	func() {
		c := ch
		if conditionFlag {
			c = ch
		}
		runWithGroup(g, runFromChanErr(c)) // want `runWithGroup\(\) func argument should use context "ctx"`
	}()
	_ = g.Wait()
}

// ===== Factory result called in place in a closure =====

// [GOOD]: Closure calls in place what a factory given a channel of funcs that use ctx returns
//
// It passes as go runFromChan(ch)() does (#300).
func goodFuncChanCtxFactoryCalledInClosureGoroutine(ctx context.Context) {
	ch := make(chan func(), 1)
	ch <- func() { use(ctx) }
	go func() {
		runFromChan(ch)()
	}()
}

// [GOOD]: Closure calls in place, from a nested closure, what a factory given a channel of funcs that use ctx returns
//
// A closure the goroutine runs in place is looked into too.
func goodFuncChanCtxFactoryCalledInNestedClosureGoroutine(ctx context.Context) {
	ch := make(chan func(), 1)
	ch <- func() { use(ctx) }
	go func() {
		func() {
			runFromChan(ch)()
		}()
	}()
}

// [GOOD]: Callback calls in place what a factory given a channel of funcs that use ctx returns
func goodFuncChanCtxFactoryCalledInClosureErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan func(), 1)
	ch <- func() { use(ctx) }
	g.Go(func() error {
		return runFromChanErr(ch)()
	})
	_ = g.Wait()
}

// [BAD]: Closure calls in place what a factory given a channel of funcs without ctx returns
//
// No func sent on the channel captures ctx.
func badFuncChanPlainFactoryCalledInClosureGoroutine(ctx context.Context) {
	ch := make(chan func(), 1)
	ch <- func() {}
	go func() { // want `goroutine does not propagate context "ctx"`
		runFromChan(ch)()
	}()
}

// [BAD]: Callback calls in place what a factory given a channel of funcs without ctx returns
//
// No func sent on the channel captures ctx.
func badFuncChanPlainFactoryCalledInClosureErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan func(), 1)
	ch <- func() {}
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		return runFromChanErr(ch)()
	})
	_ = g.Wait()
}
