// A func returned by a factory call.

package patterns

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ===== Factory call =====

// [BAD]: go fn()() - double higher-order
//
// go fn()() - higher-order without ctx
func badGoHigherOrder(ctx context.Context) {
	go makeWorkerGoroutine()() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Higher-order func
//
// Function returned by factory does not use context.
func badHigherOrderFuncErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(makeWorkerErrgroup()) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Higher-order func
//
// Function returned by factory does not use context.
func badHigherOrderFuncWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(makeWorkerWaitGroup()) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Factory function with context
//
// Factory function returns func not using context.
func badFactoryFunctionNoCtx(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, makeWorkerSpawner()) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// [GOOD]: go fn()() - double higher-order
//
// go fn(ctx)() - higher-order with ctx
func goodGoHigherOrder(ctx context.Context) {
	go makeWorkerWithCtxGoroutine(ctx)()
}

// [GOOD]: Higher-order func
//
// Factory function is called with context, enabling propagation.
func goodHigherOrderFuncWithCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(makeWorkerWithCtxErrgroup(ctx)) // OK - makeWorkerWithCtx captures ctx
	_ = g.Wait()
}

// [GOOD]: Higher-order func
//
// Factory function is called with context, enabling propagation.
func goodHigherOrderFuncWithCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(makeWorkerWithCtxWaitGroup(ctx)) // OK - makeWorkerWithCtx captures ctx
	wg.Wait()
}

// [GOOD]: Factory function with context
//
// Factory function that takes context and returns func using it.
func goodFactoryFunctionWithCtx(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, makeWorkerWithCtxSpawner(ctx)) // OK - factory takes ctx and returns func using it
	_ = g.Wait()
}

// ===== Factory returns a variable =====

// [GOOD]: Higher-order returns variable - with ctx
//
// Factory function returns a variable that captures context.
func goodHigherOrderReturnsVariableWithCtxGoroutine(ctx context.Context) {
	makeWorker := func() func() {
		worker := func() {
			_ = ctx // worker uses ctx
		}
		return worker // Returns variable, not literal
	}
	go makeWorker()()
}

// [GOOD]: Higher-order returns variable - with ctx
//
// Factory function returns a variable that captures context.
func goodHigherOrderReturnsVariableWithCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	makeWorker := func() func() error {
		worker := func() error {
			_ = ctx // worker uses ctx
			return nil
		}
		return worker // Returns variable, not literal
	}
	g.Go(makeWorker())
	_ = g.Wait()
}

// [GOOD]: Higher-order returns variable - with ctx
//
// Factory function returns a variable that captures context.
func goodHigherOrderReturnsVariableWithCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	makeWorker := func() func() {
		worker := func() {
			_ = ctx // worker uses ctx
		}
		return worker // Returns variable, not literal
	}
	wg.Go(makeWorker())
	wg.Wait()
}

// [BAD]: Higher-order returns variable - without ctx
//
// Factory function returns a variable that does not capture context.
func badHigherOrderReturnsVariableWithoutCtxGoroutine(ctx context.Context) {
	makeWorker := func() func() {
		worker := func() {
			fmt.Println("no ctx")
		}
		return worker // Returns variable, not literal
	}
	go makeWorker()() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Higher-order returns variable - without ctx
//
// Factory function returns a variable that does not capture context.
func badHigherOrderReturnsVariableWithoutCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	makeWorker := func() func() error {
		worker := func() error {
			fmt.Println("no ctx")
			return nil
		}
		return worker // Returns variable, not literal
	}
	g.Go(makeWorker()) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Higher-order returns variable - without ctx
//
// Factory function returns a variable that does not capture context.
func badHigherOrderReturnsVariableWithoutCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	makeWorker := func() func() {
		worker := func() {
			fmt.Println("no ctx")
		}
		return worker // Returns variable, not literal
	}
	wg.Go(makeWorker()) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [GOOD]: Higher-order returns reassigned variable - with ctx
//
// Factory function returns a reassigned variable that captures context.
func goodHigherOrderReturnsReassignedVariableWithCtxGoroutine(ctx context.Context) {
	makeWorker := func() func() {
		worker := func() {
			fmt.Println("first assignment")
		}
		worker = func() {
			_ = ctx // Last assignment uses ctx
		}
		return worker
	}
	go makeWorker()()
}

// [GOOD]: Higher-order returns reassigned variable - with ctx
//
// Factory function returns a reassigned variable that captures context.
func goodHigherOrderReturnsReassignedVariableWithCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	makeWorker := func() func() error {
		worker := func() error {
			fmt.Println("first assignment")
			return nil
		}
		worker = func() error {
			_ = ctx // Last assignment uses ctx
			return nil
		}
		return worker
	}
	g.Go(makeWorker())
	_ = g.Wait()
}

// [GOOD]: Higher-order returns reassigned variable - with ctx
//
// Factory function returns a reassigned variable that captures context.
func goodHigherOrderReturnsReassignedVariableWithCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	makeWorker := func() func() {
		worker := func() {
			fmt.Println("first assignment")
		}
		worker = func() {
			_ = ctx // Last assignment uses ctx
		}
		return worker
	}
	wg.Go(makeWorker())
	wg.Wait()
}

// ===== Factory given one of several ctx =====

// [GOOD]: Higher-order with multiple ctx - factory receives ctx1
//
// Factory function receives first context parameter.
func goodHigherOrderMultipleCtxGoroutine(ctx1, ctx2 context.Context) {
	go makeWorkerWithCtxGoroutine(ctx1)() // factory uses ctx1
}

// [GOOD]: Higher-order with multiple ctx - factory receives ctx1
//
// Factory function receives first context parameter.
func goodHigherOrderMultipleCtxErrgroup(ctx1, ctx2 context.Context) {
	g := new(errgroup.Group)
	g.Go(makeWorkerWithCtxErrgroup(ctx1)) // factory uses ctx1
	_ = g.Wait()
}

// [GOOD]: Higher-order with multiple ctx - factory receives ctx1
//
// Factory function receives first context parameter.
func goodHigherOrderMultipleCtxWaitGroup(ctx1, ctx2 context.Context) {
	var wg sync.WaitGroup
	wg.Go(makeWorkerWithCtxWaitGroup(ctx1)) // factory uses ctx1
	wg.Wait()
}

// [GOOD]: Higher-order with multiple ctx - factory receives ctx2
//
// Factory function receives second context parameter.
func goodHigherOrderMultipleCtxSecondGoroutine(ctx1, ctx2 context.Context) {
	go makeWorkerWithCtxGoroutine(ctx2)() // factory uses ctx2
}

// [GOOD]: Higher-order with multiple ctx - factory receives ctx2
//
// Factory function receives second context parameter.
func goodHigherOrderMultipleCtxSecondErrgroup(ctx1, ctx2 context.Context) {
	g := new(errgroup.Group)
	g.Go(makeWorkerWithCtxErrgroup(ctx2)) // factory uses ctx2
	_ = g.Wait()
}

// [GOOD]: Higher-order with multiple ctx - factory receives ctx2
//
// Factory function receives second context parameter.
func goodHigherOrderMultipleCtxSecondWaitGroup(ctx1, ctx2 context.Context) {
	var wg sync.WaitGroup
	wg.Go(makeWorkerWithCtxWaitGroup(ctx2)) // factory uses ctx2
	wg.Wait()
}

// ===== Immediately invoked factory =====

// [BAD]: IIFE factory with parentheses
//
// The parenthesized factory is resolved through SSA like any other.
func badIIFEFactoryWithParenthesesErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go((func() func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		return func() error {
			fmt.Println("no ctx")
			return nil
		}
	})())
	_ = g.Wait()
}

// [BAD]: IIFE factory with parentheses
//
// The parenthesized factory is resolved through SSA like any other.
func badIIFEFactoryWithParenthesesSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, (func() func() error { // want `runWithGroup\(\) func argument should use context "ctx"`
		return func() error {
			fmt.Println("no ctx")
			return nil
		}
	})())
	_ = g.Wait()
}

// [GOOD]: Inline factory variable with context
//
// Inline factory variable that captures context.
func goodInlineFactoryVariableWithCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	makeWorker := func() func() error {
		return func() error {
			_ = ctx
			return nil
		}
	}
	g.Go(makeWorker())
	_ = g.Wait()
}

// [GOOD]: Inline factory variable with context
//
// Inline factory variable that captures context.
func goodInlineFactoryVariableWithCtxSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	makeWorker := func() func() error {
		return func() error {
			_ = ctx
			return nil
		}
	}
	runWithGroup(g, makeWorker())
	_ = g.Wait()
}

// [BAD]: Inline factory variable without context
//
// Inline factory variable that does not capture context.
func badInlineFactoryVariableWithoutCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	makeWorker := func() func() error {
		return func() error {
			fmt.Println("no ctx")
			return nil
		}
	}
	g.Go(makeWorker()) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Inline factory variable without context
//
// Inline factory variable that does not capture context.
func badInlineFactoryVariableWithoutCtxSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	makeWorker := func() func() error {
		return func() error {
			fmt.Println("no ctx")
			return nil
		}
	}
	runWithGroup(g, makeWorker()) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

func makeWorkerGoroutine() func() {
	return func() {
		fmt.Println("worker")
	}
}

func makeWorkerWithCtxGoroutine(ctx context.Context) func() {
	return func() {
		_ = ctx
	}
}

func makeWorkerErrgroup() func() error {
	return func() error {
		fmt.Println("worker")
		return nil
	}
}

func makeWorkerWithCtxErrgroup(ctx context.Context) func() error {
	return func() error {
		_ = ctx
		return nil
	}
}

func makeWorkerWaitGroup() func() {
	return func() {
		fmt.Println("worker")
	}
}

func makeWorkerWithCtxWaitGroup(ctx context.Context) func() {
	return func() {
		_ = ctx
	}
}

func makeWorkerSpawner() func() error {
	return func() error {
		fmt.Println("no ctx")
		return nil
	}
}

func makeWorkerWithCtxSpawner(ctx context.Context) func() error {
	return func() error {
		_ = ctx
		return nil
	}
}

// ===== Decorator =====

// [GOOD]: Decorator
//
// A decorator runs the func it is given, here from a struct field, so that
// func is judged (#148).
func goodDecoratorGoroutine(ctx context.Context) {
	job := struct{ fn func() }{fn: func() { _ = ctx }}
	go recoverWrap(job.fn)()
}

// [BAD]: Decorator without ctx
//
// The func the decorator is given does not use ctx.
func badDecoratorGoroutine(ctx context.Context) {
	go recoverWrap(func() { fmt.Println("no ctx") })() // want `goroutine does not propagate context "ctx"`
}

// [GOOD]: Decorator
//
// A decorator runs the func it is given, here from a struct field, so that
// func is judged (#148).
func goodDecoratorErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	job := struct{ fn func() error }{fn: func() error { _ = ctx; return nil }}
	g.Go(recoverWrapErr(job.fn))
	_ = g.Wait()
}

// [BAD]: Decorator without ctx
//
// The func the decorator is given does not use ctx.
func badDecoratorErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(recoverWrapErr(func() error { fmt.Println("no ctx"); return nil })) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [GOOD]: Decorator
//
// A decorator runs the func it is given, here from a struct field, so that
// func is judged (#148).
func goodDecoratorWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	job := struct{ fn func() }{fn: func() { _ = ctx }}
	wg.Go(recoverWrap(job.fn))
	wg.Wait()
}

// [BAD]: Decorator without ctx
//
// The func the decorator is given does not use ctx.
func badDecoratorWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(recoverWrap(func() { fmt.Println("no ctx") })) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [GOOD]: Decorator
//
// A decorator runs the func it is given, here from a struct field, so that
// func is judged (#148).
func goodDecoratorSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	job := struct{ fn func() error }{fn: func() error { _ = ctx; return nil }}
	runWithGroup(g, recoverWrapErr(job.fn))
	_ = g.Wait()
}

// [BAD]: Decorator without ctx
//
// The func the decorator is given does not use ctx.
func badDecoratorSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, recoverWrapErr(func() error { fmt.Println("no ctx"); return nil })) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// recoverWrap runs f, recovering a panic.
func recoverWrap(f func()) func() {
	return func() {
		defer func() { _ = recover() }()
		f()
	}
}

// recoverWrapErr runs f, recovering a panic.
func recoverWrapErr(f func() error) func() error {
	return func() error {
		defer func() { _ = recover() }()
		return f()
	}
}
