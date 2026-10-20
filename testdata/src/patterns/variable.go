// A func given through a local variable, a reassigned variable or a parameter.

package patterns

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ===== Variable func =====

// [BAD]: Variable func
//
// Function stored in variable does not capture context.
func badVariableFuncErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error {
		fmt.Println("no ctx")
		return nil
	}
	g.Go(fn) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Variable func
//
// Function stored in variable does not capture context.
func badVariableFuncWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	fn := func() {
		fmt.Println("no ctx")
	}
	wg.Go(fn) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Basic errgroup func context usage
//
// Basic - func doesn't use ctx
func badBasicErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error {
		fmt.Println("no ctx")
		return nil
	}
	runWithGroup(g, fn) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Basic waitgroup func context usage
//
// Basic - func doesn't use ctx (waitgroup)
func badBasicWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	fn := func() {
		fmt.Println("no ctx")
	}
	runWithWaitGroup(&wg, fn) // want `runWithWaitGroup\(\) func argument should use context "ctx"`
	wg.Wait()
}

// [GOOD]: Variable func
//
// Function stored in variable captures and uses context.
func goodVariableFuncWithCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error {
		_ = ctx
		return nil
	}
	g.Go(fn) // OK - fn uses ctx
	_ = g.Wait()
}

// [GOOD]: Variable func
//
// Function stored in variable captures and uses context.
func goodVariableFuncWithCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	fn := func() {
		_ = ctx
	}
	wg.Go(fn) // OK - fn uses ctx
	wg.Wait()
}

// [GOOD]: Basic errgroup func context usage
//
// Basic - func uses ctx
func goodBasicErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error {
		_ = ctx
		return nil
	}
	runWithGroup(g, fn) // OK - fn uses ctx
	_ = g.Wait()
}

// [GOOD]: Basic waitgroup func context usage
//
// Basic - func uses ctx (waitgroup)
func goodBasicWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	fn := func() {
		_ = ctx
	}
	runWithWaitGroup(&wg, fn) // OK - fn uses ctx
	wg.Wait()
}

// ===== Conditional reassignment =====

// [BAD]: Conditional reassignment - first uses ctx, conditional doesn't
//
// First assignment uses context, but conditional reassignment doesn't.
// If condition is true, goroutine won't propagate context.
// All paths must be safe, so this should warn.
func badConditionalReassignFirstUsesCtxGoroutine(ctx context.Context) {
	fn := func() { _ = ctx } // First assignment uses ctx
	if conditionFlag {
		fn = func() { fmt.Println("no ctx") } // Conditional reassignment without ctx
	}
	go fn() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Conditional reassignment - first uses ctx, conditional doesn't
//
// First assignment uses context, but conditional reassignment doesn't.
func badConditionalReassignFirstUsesCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error {
		_ = ctx
		return nil
	}
	if conditionFlag {
		fn = func() error {
			fmt.Println("no ctx")
			return nil
		}
	}
	g.Go(fn) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Conditional reassignment - first doesn't use ctx, conditional does
//
// First assignment ignores context, conditional reassignment uses it.
// If condition is false, goroutine won't propagate context.
// All paths must be safe, so this should warn.
func badConditionalReassignFirstNoCtxGoroutine(ctx context.Context) {
	fn := func() { fmt.Println("no ctx") } // First assignment without ctx
	if conditionFlag {
		fn = func() { _ = ctx } // Conditional reassignment with ctx
	}
	go fn() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Conditional reassignment - first doesn't use ctx, conditional does
//
// First assignment ignores context, conditional reassignment uses it.
func badConditionalReassignFirstNoCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error {
		fmt.Println("no ctx")
		return nil
	}
	if conditionFlag {
		fn = func() error {
			_ = ctx
			return nil
		}
	}
	g.Go(fn) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [GOOD]: Conditional reassignment - all paths use ctx
//
// All assignments use context, so all paths propagate it.
func goodConditionalReassignAllUseCtxGoroutine(ctx context.Context) {
	fn := func() { _ = ctx } // First assignment uses ctx
	if conditionFlag {
		fn = func() { _ = ctx } // Conditional reassignment also uses ctx
	}
	go fn() // All paths safe
}

// [GOOD]: Conditional reassignment - all paths use ctx
//
// All assignments use context, so all paths propagate it.
func goodConditionalReassignAllUseCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error {
		_ = ctx
		return nil
	}
	if conditionFlag {
		fn = func() error {
			_ = ctx
			return nil
		}
	}
	g.Go(fn) // OK - all assignments use ctx
	_ = g.Wait()
}

// ===== Read in a closure =====

// [GOOD]: Read in a closure, replaced before it is made
//
// Only the stores that reach the closure, or come after it, count (#146).
func goodReplacedBeforeClosureGoroutine(ctx context.Context) {
	fn := func() {}
	fn = func() { _ = ctx }
	func() {
		go fn()
	}()
}

// [BAD]: Read in a closure, replaced after it is made
//
// The closure may run after the later store, so that store counts.
func badReplacedAfterClosureGoroutine(ctx context.Context) {
	fn := func() { _ = ctx }
	run := func() {
		go fn() // want `goroutine does not propagate context "ctx"`
	}
	fn = func() {}
	run()
}

// [GOOD]: Read in a closure, replaced before it is made
//
// Only the stores that reach the closure, or come after it, count (#146).
func goodReplacedBeforeClosureErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error { return nil }
	fn = func() error { _ = ctx; return nil }
	func() {
		g.Go(fn)
	}()
	_ = g.Wait()
}

// [BAD]: Read in a closure, replaced after it is made
//
// The closure may run after the later store, so that store counts.
func badReplacedAfterClosureErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error { _ = ctx; return nil }
	run := func() {
		g.Go(fn) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	}
	fn = func() error { return nil }
	run()
	_ = g.Wait()
}

// [GOOD]: Read in a closure, replaced before it is made
//
// Only the stores that reach the closure, or come after it, count (#146).
func goodReplacedBeforeClosureWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	fn := func() {}
	fn = func() { _ = ctx }
	func() {
		wg.Go(fn)
	}()
	wg.Wait()
}

// [BAD]: Read in a closure, replaced after it is made
//
// The closure may run after the later store, so that store counts.
func badReplacedAfterClosureWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	fn := func() { _ = ctx }
	run := func() {
		wg.Go(fn) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	}
	fn = func() {}
	run()
	wg.Wait()
}

// [GOOD]: Read in a closure, replaced before it is made
//
// Only the stores that reach the closure, or come after it, count (#146).
func goodReplacedBeforeClosureSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error { return nil }
	fn = func() error { _ = ctx; return nil }
	func() {
		runWithGroup(g, fn)
	}()
	_ = g.Wait()
}

// [BAD]: Read in a closure, replaced after it is made
//
// The closure may run after the later store, so that store counts.
func badReplacedAfterClosureSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error { _ = ctx; return nil }
	run := func() {
		runWithGroup(g, fn) // want `runWithGroup\(\) func argument should use context "ctx"`
	}
	fn = func() error { return nil }
	run()
	_ = g.Wait()
}

// [GOOD]: Read in a range-over-func body, replaced before the loop
//
// The loop body is a closure, made after the store that replaced the func.
func goodReplacedBeforeRangeFuncErrgroup(ctx context.Context, seq func(func(int) bool)) {
	g := new(errgroup.Group)
	fn := func() error { return nil }
	fn = func() error { _ = ctx; return nil }
	for range seq {
		g.Go(fn)
	}
	_ = g.Wait()
}

// [GOOD]: Struct field read in a closure, replaced before it is made
//
// A part of a variable is judged the same way.
func goodFieldReplacedBeforeClosureGoroutine(ctx context.Context) {
	s := struct{ fn func() }{fn: func() {}}
	s.fn = func() { _ = ctx }
	func() {
		go s.fn()
	}()
}

// ===== Func from a parameter =====

// [LIMITATION]: Function from parameter - cannot trace
//
// Function passed as parameter cannot be traced statically.
// We assume it's OK (zero false positives policy).
func limitationFuncFromParameterErrgroup(ctx context.Context, fn func() error) {
	g := new(errgroup.Group)
	// fn comes from parameter - can't trace its body
	g.Go(fn) // No error - can't trace parameter function
	_ = g.Wait()
}

// [LIMITATION]: Function from parameter - cannot trace
//
// Function passed as parameter cannot be traced statically.
// We assume it's OK (zero false positives policy).
func limitationFuncFromParameterSpawner(ctx context.Context, fn func() error) {
	g := new(errgroup.Group)
	// fn comes from parameter - can't trace its body
	runWithGroup(g, fn) // No error - can't trace parameter function
	_ = g.Wait()
}

// [LIMITATION]: Function from variadic parameter - cannot trace
//
// Functions passed as variadic parameter cannot be traced statically.
func limitationFuncFromVariadicParameterErrgroup(ctx context.Context, fns ...func() error) {
	g := new(errgroup.Group)
	for _, fn := range fns {
		g.Go(fn) // No error - can't trace parameter function
	}
	_ = g.Wait()
}

// [LIMITATION]: Function from variadic parameter - cannot trace
//
// Functions passed as variadic parameter cannot be traced statically.
func limitationFuncFromVariadicParameterSpawner(ctx context.Context, fns ...func()) {
	var wg sync.WaitGroup
	for _, fn := range fns {
		runWithWaitGroup(&wg, fn) // No error - can't trace parameter function
	}
	wg.Wait()
}

//goroutinectx:spawner
func runWithWaitGroup(wg *sync.WaitGroup, fn func()) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		fn()
	}()
}

var conditionFlag bool
