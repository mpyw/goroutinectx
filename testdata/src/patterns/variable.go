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

// ===== Func value called inside a literal =====

// [NOTCHECKED]: Func from a parameter, called inside a literal
//
// The func cannot be traced, so it passes, as go fn() does (#228).
func notCheckedParamFuncInLiteralGoroutine(ctx context.Context, fn func()) {
	go func() { fn() }()
}

// [NOTCHECKED]: Func from a parameter, called inside a literal
//
// The func cannot be traced, so it passes, as g.Go(fn) does (#228).
func notCheckedParamFuncInLiteralErrgroup(ctx context.Context, fn func() error) {
	g := new(errgroup.Group)
	g.Go(func() error { return fn() })
	_ = g.Wait()
}

// [NOTCHECKED]: Func from a parameter, called inside a literal
//
// The func cannot be traced, so it passes, as wg.Go(fn) does (#228).
func notCheckedParamFuncInLiteralWaitGroup(ctx context.Context, fn func()) {
	var wg sync.WaitGroup
	wg.Go(func() { fn() })
	wg.Wait()
}

// [NOTCHECKED]: Func from a parameter, called inside a literal
//
// The func cannot be traced, so it passes, as runWithGroup(g, fn) does
// (#228).
func notCheckedParamFuncInLiteralSpawner(ctx context.Context, fn func() error) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error { return fn() })
	_ = g.Wait()
}

// [NOTCHECKED]: Func from a field of a parameter, called inside a literal
//
// The field cannot be traced, so it passes, as go h.fn() does (#228).
func notCheckedParamFieldFuncInLiteralGoroutine(ctx context.Context, h *funcHolder) {
	go func() { h.fn() }()
}

// [NOTCHECKED]: Func from a field of a parameter, called inside a literal
//
// The field cannot be traced, so it passes, as g.Go(h.fn) does (#228).
func notCheckedParamFieldFuncInLiteralErrgroup(ctx context.Context, h *errFuncHolder) {
	g := new(errgroup.Group)
	g.Go(func() error { return h.fn() })
	_ = g.Wait()
}

// [NOTCHECKED]: Func from a field of a parameter, called inside a literal
//
// The field cannot be traced, so it passes, as wg.Go(h.fn) does (#228).
func notCheckedParamFieldFuncInLiteralWaitGroup(ctx context.Context, h *funcHolder) {
	var wg sync.WaitGroup
	wg.Go(func() { h.fn() })
	wg.Wait()
}

// [NOTCHECKED]: Func from a field of a parameter, called inside a literal
//
// The field cannot be traced, so it passes, as runWithGroup(g, h.fn) does
// (#228).
func notCheckedParamFieldFuncInLiteralSpawner(ctx context.Context, h *errFuncHolder) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error { return h.fn() })
	_ = g.Wait()
}

// [GOOD]: Func from a factory given ctx, called inside a literal
//
// The factory is given ctx, as for go run() (#228).
func goodFactoryFuncInLiteralGoroutine(ctx context.Context) {
	run, _ := serveWithCtx(ctx)
	go func() { _ = run() }()
}

// [GOOD]: Func from a factory given ctx, called inside a literal
//
// The factory is given ctx, as for g.Go(run) (#228).
func goodFactoryFuncInLiteralErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	run, _ := serveWithCtx(ctx)
	g.Go(func() error { return run() })
	_ = g.Wait()
}

// [GOOD]: Func from a factory given ctx, called inside a literal
//
// The factory is given ctx, as for wg.Go(run) (#228).
func goodFactoryFuncInLiteralWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	run := makeWorkerWithCtxGoroutine(ctx)
	wg.Go(func() { run() })
	wg.Wait()
}

// [GOOD]: Func from a factory given ctx, called inside a literal
//
// The factory is given ctx, as for runWithGroup(g, run) (#228).
func goodFactoryFuncInLiteralSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	run := makeWorkerWithCtxErrgroup(ctx)
	runWithGroup(g, func() error { return run() })
	_ = g.Wait()
}

// [BAD]: Func from a named factory given only a fresh context, called inside a literal
//
// The factory gets no ctx, as for go makeWorkerWithCtxGoroutine(context.Background())().
func badFreshFactoryFuncInLiteralGoroutine(ctx context.Context) {
	run := makeWorkerWithCtxGoroutine(context.Background())
	go func() { run() }() // want `goroutine does not propagate context "ctx"`
}

// [GOOD]: Func from a slice element, called inside a literal
//
// An element at an index that is not constant is not followed, so it
// passes, as go op() does (#228). Here every element uses ctx.
func goodSliceElemFuncInLiteralGoroutine(ctx context.Context) {
	ops := []func(){
		func() { _ = ctx },
		func() { _ = ctx },
	}
	for _, op := range ops {
		go func() { op() }()
	}
}

// [GOOD]: Func from a slice element, called inside a literal
//
// An element at an index that is not constant is not followed, so it
// passes, as g.Go(op) does (#228). Here every element uses ctx.
func goodSliceElemFuncInLiteralErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	ops := []func() error{
		func() error { return ctx.Err() },
		func() error { return ctx.Err() },
	}
	for _, op := range ops {
		g.Go(func() error { return op() })
	}
	_ = g.Wait()
}

// [GOOD]: Func from a slice element, called inside a literal
//
// An element at an index that is not constant is not followed, so it
// passes, as wg.Go(op) does (#228). Here every element uses ctx.
func goodSliceElemFuncInLiteralWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	ops := []func(){
		func() { _ = ctx },
		func() { _ = ctx },
	}
	for _, op := range ops {
		wg.Go(func() { op() })
	}
	wg.Wait()
}

// [GOOD]: Func from a slice element, called inside a literal
//
// An element at an index that is not constant is not followed, so it
// passes, as runWithGroup(g, op) does (#228). Here every element uses ctx.
func goodSliceElemFuncInLiteralSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	ops := []func() error{
		func() error { return ctx.Err() },
		func() error { return ctx.Err() },
	}
	for _, op := range ops {
		runWithGroup(g, func() error { return op() })
	}
	_ = g.Wait()
}

// ===== Func from a captured map, called inside a literal =====

// [GOOD]: Func from a captured map, called inside a literal
//
// A func read from a map is not followed, so it passes, as one read from a
// slice at an index that is not constant does (#253). Here it uses ctx.
func goodMapFuncInLiteralGoroutine(ctx context.Context, k string) {
	handlers := map[string]func(){"a": func() { _ = ctx }}
	go func() { handlers[k]() }()
}

// [GOOD]: Func from a captured map, called inside a literal
//
// A func read from a map is not followed, so it passes, as one read from a
// slice at an index that is not constant does (#253). Here it uses ctx.
func goodMapFuncInLiteralErrgroup(ctx context.Context, k string) {
	g := new(errgroup.Group)
	handlers := map[string]func(){"a": func() { _ = ctx }}
	g.Go(func() error { handlers[k](); return nil })
	_ = g.Wait()
}

// [GOOD]: Func from a captured map, called inside a literal
//
// A func read from a map is not followed, so it passes, as one read from a
// slice at an index that is not constant does (#253). Here it uses ctx.
func goodMapFuncInLiteralWaitGroup(ctx context.Context, k string) {
	var wg sync.WaitGroup
	handlers := map[string]func(){"a": func() { _ = ctx }}
	wg.Go(func() { handlers[k]() })
	wg.Wait()
}

// [GOOD]: Func from a captured map, called inside a literal
//
// A func read from a map is not followed, so it passes, as one read from a
// slice at an index that is not constant does (#253). Here it uses ctx.
func goodMapFuncInLiteralSpawner(ctx context.Context, k string) {
	g := new(errgroup.Group)
	handlers := map[string]func(){"a": func() { _ = ctx }}
	runWithGroup(g, func() error { handlers[k](); return nil })
	_ = g.Wait()
}

// ===== Func from a range over a captured channel, called inside a literal =====

// [GOOD]: Func from a range over a captured channel, called inside a literal
//
// A func received from a channel is not followed, so it passes (#253).
// Here it uses ctx.
func goodChanFuncInLiteralGoroutine(ctx context.Context) {
	jobs := make(chan func(), 1)
	jobs <- func() { _ = ctx }
	close(jobs)
	go func() {
		for job := range jobs {
			job()
		}
	}()
}

// [GOOD]: Func from a range over a captured channel, called inside a literal
//
// A func received from a channel is not followed, so it passes (#253).
// Here it uses ctx.
func goodChanFuncInLiteralErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	jobs := make(chan func(), 1)
	jobs <- func() { _ = ctx }
	close(jobs)
	g.Go(func() error {
		for job := range jobs {
			job()
		}
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Func from a range over a captured channel, called inside a literal
//
// A func received from a channel is not followed, so it passes (#253).
// Here it uses ctx.
func goodChanFuncInLiteralWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	jobs := make(chan func(), 1)
	jobs <- func() { _ = ctx }
	close(jobs)
	wg.Go(func() {
		for job := range jobs {
			job()
		}
	})
	wg.Wait()
}

// [GOOD]: Func from a range over a captured channel, called inside a literal
//
// A func received from a channel is not followed, so it passes (#253).
// Here it uses ctx.
func goodChanFuncInLiteralSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	jobs := make(chan func(), 1)
	jobs <- func() { _ = ctx }
	close(jobs)
	runWithGroup(g, func() error {
		for job := range jobs {
			job()
		}
		return nil
	})
	_ = g.Wait()
}

// ===== Captured map of funcs, not called inside a literal =====

// [BAD]: Captured map of funcs, not called inside a literal
//
// A func that is only counted, not run, gives no context.
func badMapFuncNotCalledInLiteralGoroutine(ctx context.Context) {
	handlers := map[string]func(){"a": func() { _ = ctx }}
	go func() { _ = len(handlers) }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Captured map of funcs, not called inside a literal
//
// A func that is only counted, not run, gives no context.
func badMapFuncNotCalledInLiteralErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	handlers := map[string]func(){"a": func() { _ = ctx }}
	g.Go(func() error { _ = len(handlers); return nil }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Captured map of funcs, not called inside a literal
//
// A func that is only counted, not run, gives no context.
func badMapFuncNotCalledInLiteralWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	handlers := map[string]func(){"a": func() { _ = ctx }}
	wg.Go(func() { _ = len(handlers) }) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Captured map of funcs, not called inside a literal
//
// A func that is only counted, not run, gives no context.
func badMapFuncNotCalledInLiteralSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	handlers := map[string]func(){"a": func() { _ = ctx }}
	runWithGroup(g, func() error { _ = len(handlers); return nil }) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// [GOOD]: Func copied from a variable set on two branches, called inside a literal
//
// Each closure the copy can hold uses ctx.
func goodBranchCopyFuncInLiteralGoroutine(ctx context.Context) {
	var f func()
	if conditionFlag {
		f = func() { _ = ctx }
	} else {
		f = func() { use(ctx) }
	}
	g := f
	go func() { g() }()
}

// [GOOD]: Func copied from a variable set on two branches, called inside a literal
//
// Each closure the copy can hold uses ctx.
func goodBranchCopyFuncInLiteralErrgroup(ctx context.Context) {
	eg := new(errgroup.Group)
	var f func() error
	if conditionFlag {
		f = func() error { return ctx.Err() }
	} else {
		f = func() error { use(ctx); return nil }
	}
	g := f
	eg.Go(func() error { return g() })
	_ = eg.Wait()
}

// [GOOD]: Func copied from a variable set on two branches, called inside a literal
//
// Each closure the copy can hold uses ctx.
func goodBranchCopyFuncInLiteralWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	var f func()
	if conditionFlag {
		f = func() { _ = ctx }
	} else {
		f = func() { use(ctx) }
	}
	g := f
	wg.Go(func() { g() })
	wg.Wait()
}

// [GOOD]: Func copied from a variable set on two branches, called inside a literal
//
// Each closure the copy can hold uses ctx.
func goodBranchCopyFuncInLiteralSpawner(ctx context.Context) {
	eg := new(errgroup.Group)
	var f func() error
	if conditionFlag {
		f = func() error { return ctx.Err() }
	} else {
		f = func() error { use(ctx); return nil }
	}
	g := f
	runWithGroup(eg, func() error { return g() })
	_ = eg.Wait()
}

// [BAD]: Func from a field of a local struct, called inside a literal
//
// The field is followed to a closure that does not use ctx.
func badLocalFieldFuncInLiteralGoroutine(ctx context.Context) {
	h := &funcHolder{fn: func() { fmt.Println("no ctx") }}
	go func() { h.fn() }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Func from a field of a local struct, called inside a literal
//
// The field is followed to a closure that does not use ctx.
func badLocalFieldFuncInLiteralErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	h := &errFuncHolder{fn: func() error { return nil }}
	g.Go(func() error { return h.fn() }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Func from a field of a local struct, called inside a literal
//
// The field is followed to a closure that does not use ctx.
func badLocalFieldFuncInLiteralWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	h := &funcHolder{fn: func() { fmt.Println("no ctx") }}
	wg.Go(func() { h.fn() }) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Func from a field of a local struct, called inside a literal
//
// The field is followed to a closure that does not use ctx.
func badLocalFieldFuncInLiteralSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	h := &errFuncHolder{fn: func() error { return nil }}
	runWithGroup(g, func() error { return h.fn() }) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Func variable copied from a closure without ctx, called inside a literal
//
// Each closure the copy can hold is followed, and none uses ctx.
func badBranchCopyNoCtxFuncInLiteralGoroutine(ctx context.Context) {
	var f func()
	if conditionFlag {
		f = func() { fmt.Println("a") }
	} else {
		f = func() { fmt.Println("b") }
	}
	g := f
	go func() { g() }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Func from a parameter, only compared inside a literal
//
// The func is not run, so it gives the goroutine nothing.
func badParamFuncComparedInLiteralGoroutine(ctx context.Context, fn func()) {
	go func() { // want `goroutine does not propagate context "ctx"`
		if fn != nil {
			fmt.Println("set")
		}
	}()
}

// serveWithCtx gives a func that waits for ctx, and an error.
func serveWithCtx(ctx context.Context) (func() error, error) {
	return func() error { <-ctx.Done(); return nil }, nil
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
