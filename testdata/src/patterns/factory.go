// A func returned by a factory call.

package patterns

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/example/ctxworker"
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

// ===== Factory given a fresh context =====

// [BAD]: Factory given a fresh context
//
// context.Background() carries nothing of ctx, so the func the factory
// returns does not get ctx (#183).
func badFactoryFreshCtxGoroutine(ctx context.Context) {
	go makeWorkerWithCtxGoroutine(context.Background())() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Factory given a fresh context
//
// context.TODO() carries nothing of ctx, so the func the factory returns
// does not get ctx (#183).
func badFactoryFreshCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(makeWorkerWithCtxErrgroup(context.TODO())) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Factory given a fresh context
//
// context.Background() carries nothing of ctx, so the func the factory
// returns does not get ctx (#183).
func badFactoryFreshCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(makeWorkerWithCtxWaitGroup(context.Background())) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Factory given a fresh context
//
// A fresh context in a variable is followed to context.Background() (#183).
func badFactoryFreshCtxSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	bg := context.Background()
	runWithGroup(g, makeWorkerWithCtxSpawner(bg)) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// [GOOD]: Factory given ctx and a fresh context
//
// One of the contexts the factory is given is ctx.
func goodFactoryCtxAndFreshCtxGoroutine(ctx context.Context) {
	go makeWorkerWithTwoCtxGoroutine(ctx, context.Background())()
}

// [GOOD]: Factory given ctx and a fresh context
//
// One of the contexts the factory is given is ctx.
func goodFactoryCtxAndFreshCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(makeWorkerWithTwoCtxErrgroup(context.TODO(), ctx))
	_ = g.Wait()
}

// [GOOD]: Factory given ctx and a fresh context
//
// One of the contexts the factory is given is ctx.
func goodFactoryCtxAndFreshCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(makeWorkerWithTwoCtxGoroutine(ctx, context.Background()))
	wg.Wait()
}

// [GOOD]: Factory given ctx and a fresh context
//
// One of the contexts the factory is given is ctx.
func goodFactoryCtxAndFreshCtxSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, makeWorkerWithTwoCtxErrgroup(ctx, context.Background()))
	_ = g.Wait()
}

// [GOOD]: Factory given a context derived from ctx
//
// A context derived from ctx is not fresh.
func goodFactoryDerivedCtxGoroutine(ctx context.Context) {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	go makeWorkerWithCtxGoroutine(c)()
}

// [GOOD]: Factory given a context derived from ctx
//
// A context derived from ctx is not fresh.
func goodFactoryDerivedCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(makeWorkerWithCtxErrgroup(context.WithoutCancel(ctx)))
	_ = g.Wait()
}

// [GOOD]: Factory given a context derived from ctx
//
// A context derived from ctx is not fresh.
func goodFactoryDerivedCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	wg.Go(makeWorkerWithCtxWaitGroup(c))
	wg.Wait()
}

// [GOOD]: Factory given a context derived from ctx
//
// A context derived from ctx is not fresh.
func goodFactoryDerivedCtxSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	runWithGroup(g, makeWorkerWithCtxSpawner(c))
	_ = g.Wait()
}

// [GOOD]: Factory closure given a fresh context
//
// A factory closure may capture ctx, so it is not judged by what it is given.
func goodFactoryClosureFreshCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	makeWorker := func(c context.Context) func() error {
		return func() error {
			_ = c
			_ = ctx
			return nil
		}
	}
	g.Go(makeWorker(context.Background()))
	_ = g.Wait()
}

// ===== Function of another package given a fresh context =====

// [BAD]: Function of another package given a fresh context
//
// A function of another package is judged by what it is given, as one of
// this package is: context.Background() carries nothing of ctx (#223).
func badOtherPkgFuncFreshCtxGoroutine(ctx context.Context) {
	go ctxworker.Work(context.Background()) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Factory of another package given a fresh context
//
// The func the factory returns does not get ctx (#223).
func badOtherPkgFactoryFreshCtxGoroutine(ctx context.Context) {
	go ctxworker.Make(context.Background())() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Factory of another package given a fresh context
//
// The func the factory returns does not get ctx (#223).
func badOtherPkgFactoryFreshCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(ctxworker.MakeErr(context.TODO())) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Factory of another package given a fresh context
//
// The func the factory returns does not get ctx (#223).
func badOtherPkgFactoryFreshCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(ctxworker.Make(context.Background())) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Factory of another package given a fresh context
//
// A fresh context in a variable is followed to context.Background(), and a
// generic factory is judged as a plain one (#223).
func badOtherPkgFactoryFreshCtxSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	bg := context.Background()
	runWithGroup(g, ctxworker.MakeErrOf(bg, 1)) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// [GOOD]: Function of another package given ctx
func goodOtherPkgFuncCtxGoroutine(ctx context.Context) {
	go ctxworker.Work(ctx)
}

// [GOOD]: Factory of another package given ctx
func goodOtherPkgFactoryCtxGoroutine(ctx context.Context) {
	go ctxworker.Make(ctx)()
}

// [GOOD]: Factory of another package given ctx
func goodOtherPkgFactoryCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(ctxworker.MakeErr(ctx))
	_ = g.Wait()
}

// [GOOD]: Factory of another package given ctx
func goodOtherPkgFactoryCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(ctxworker.Make(ctx))
	wg.Wait()
}

// [GOOD]: Factory of another package given ctx
func goodOtherPkgFactoryCtxSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, ctxworker.MakeErrOf(ctx, 1))
	_ = g.Wait()
}

// [GOOD]: Function of another package given a fresh context and a value that holds ctx
//
// The struct the function is given holds ctx (#192).
func goodOtherPkgFuncFreshCtxAndDepsGoroutine(ctx context.Context) {
	go ctxworker.WorkWithDeps(context.Background(), &ctxworker.Deps{Ctx: ctx})
}

// [GOOD]: Factory of another package given a fresh context and a value that holds ctx
//
// The struct the factory is given holds ctx (#192).
func goodOtherPkgFactoryFreshCtxAndDepsGoroutine(ctx context.Context) {
	go ctxworker.MakeWithDeps(context.Background(), &ctxworker.Deps{Ctx: ctx})()
}

// [GOOD]: Factory of another package given a fresh context and a value that holds ctx
//
// The struct the factory is given holds ctx (#192).
func goodOtherPkgFactoryFreshCtxAndDepsErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(ctxworker.MakeErrWithDeps(context.Background(), &ctxworker.Deps{Ctx: ctx}))
	_ = g.Wait()
}

// [GOOD]: Factory of another package given a fresh context and a value that holds ctx
//
// The struct the factory is given holds ctx (#192).
func goodOtherPkgFactoryFreshCtxAndDepsWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(ctxworker.MakeWithDeps(context.TODO(), &ctxworker.Deps{Ctx: ctx}))
	wg.Wait()
}

// [GOOD]: Factory of another package given a fresh context and a value that holds ctx
//
// The struct the factory is given holds ctx (#192).
func goodOtherPkgFactoryFreshCtxAndDepsSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, ctxworker.MakeErrWithDeps(context.Background(), &ctxworker.Deps{Ctx: ctx}))
	_ = g.Wait()
}

// [GOOD]: Method of another package given a fresh context
//
// A method may get ctx from its receiver, so it is not judged by what it is
// given, as a method of this package is not.
func goodOtherPkgMethodFreshCtxGoroutine(ctx context.Context) {
	s := &ctxworker.Svc{Ctx: ctx}
	go s.Run(context.Background())
}

// [GOOD]: Factory method of another package given a fresh context
//
// A method may get ctx from its receiver, so it is not judged by what it is
// given.
func goodOtherPkgFactoryMethodFreshCtxGoroutine(ctx context.Context) {
	s := &ctxworker.Svc{Ctx: ctx}
	go s.Make(context.Background())()
}

// [GOOD]: Factory method of another package given a fresh context
//
// A method may get ctx from its receiver, so it is not judged by what it is
// given.
func goodOtherPkgFactoryMethodFreshCtxErrgroup(ctx context.Context) {
	s := &ctxworker.Svc{Ctx: ctx}
	g := new(errgroup.Group)
	g.Go(s.MakeErr(context.Background()))
	_ = g.Wait()
}

// [GOOD]: Factory method of another package given a fresh context
//
// A method may get ctx from its receiver, so it is not judged by what it is
// given.
func goodOtherPkgFactoryMethodFreshCtxWaitGroup(ctx context.Context) {
	s := &ctxworker.Svc{Ctx: ctx}
	var wg sync.WaitGroup
	wg.Go(s.Make(context.TODO()))
	wg.Wait()
}

// [GOOD]: Factory method of another package given a fresh context
//
// A method may get ctx from its receiver, so it is not judged by what it is
// given.
func goodOtherPkgFactoryMethodFreshCtxSpawner(ctx context.Context) {
	s := &ctxworker.Svc{Ctx: ctx}
	g := new(errgroup.Group)
	runWithGroup(g, s.MakeErr(context.Background()))
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

func makeWorkerWithTwoCtxGoroutine(ctx1, ctx2 context.Context) func() {
	return func() {
		_, _ = ctx1, ctx2
	}
}

func makeWorkerWithTwoCtxErrgroup(ctx1, ctx2 context.Context) func() error {
	return func() error {
		_, _ = ctx1, ctx2
		return nil
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

// [BAD]: Factory that returns the func it is given, without ctx
//
// The parameter holds the func the call gives, which does not use ctx, so it
// does not pass for being a parameter.
func badIdentityFactoryGoroutine(ctx context.Context) {
	go identityFunc(func() { fmt.Println("no ctx") })() // want `goroutine does not propagate context "ctx"`
}

// [GOOD]: Factory that returns the func it is given
//
// The func the call gives uses ctx.
func goodIdentityFactoryGoroutine(ctx context.Context) {
	go identityFunc(func() { _ = ctx })()
}

// identityFunc returns f.
func identityFunc(f func()) func() { return f }

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

// ===== Factory given a fresh context and a value that may hold ctx =====

// ctxDeps holds a context for a worker.
type ctxDeps struct{ ctx context.Context }

// ctxSvc runs work with the context it holds.
type ctxSvc struct{ ctx context.Context }

func (s *ctxSvc) run() error {
	_ = s.ctx
	return nil
}

func (s *ctxSvc) runNoErr() {
	_ = s.ctx
}

// [GOOD]: Named function given a fresh context and a value that holds ctx
//
// The struct the function is given holds ctx (#192).
func goodNamedFuncFreshCtxAndDepsGoroutine(ctx context.Context) {
	go runWithDeps(context.Background(), &ctxDeps{ctx: ctx})
}

// [GOOD]: Closure given a fresh context and a value that holds ctx
//
// The struct the closure is given holds ctx (#192).
func goodClosureFreshCtxAndDepsGoroutine(ctx context.Context) {
	go func(_ context.Context, d *ctxDeps) {
		_ = d.ctx
	}(context.Background(), &ctxDeps{ctx: ctx})
}

// [GOOD]: Factory given a fresh context and a value that holds ctx
//
// The struct the factory is given holds ctx (#192).
func goodFactoryFreshCtxAndDepsGoroutine(ctx context.Context) {
	go makeWorkerWithDeps(context.Background(), &ctxDeps{ctx: ctx})()
}

// [GOOD]: Factory given a fresh context and a value that holds ctx
//
// The struct the factory is given holds ctx (#192).
func goodFactoryFreshCtxAndDepsErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(makeErrWorkerWithDeps(context.Background(), &ctxDeps{ctx: ctx}))
	_ = g.Wait()
}

// [GOOD]: Factory given a fresh context and a value that holds ctx
//
// The struct the factory is given holds ctx (#192).
func goodFactoryFreshCtxAndDepsWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(makeWorkerWithDeps(context.TODO(), &ctxDeps{ctx: ctx}))
	wg.Wait()
}

// [GOOD]: Factory given a fresh context and a value that holds ctx
//
// The struct the factory is given holds ctx (#192).
func goodFactoryFreshCtxAndDepsSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, makeErrWorkerWithDeps(context.Background(), &ctxDeps{ctx: ctx}))
	_ = g.Wait()
}

// [GOOD]: Decorator given a fresh context and a method value
//
// The bound receiver of the method value holds ctx, as for g.Go(s.run)
// (#148, #192).
func goodDecoratorFreshCtxAndMethodGoroutine(ctx context.Context) {
	s := &ctxSvc{ctx: ctx}
	go logPanic(context.Background(), s.runNoErr)()
}

// [GOOD]: Decorator given a fresh context and a method value
//
// The bound receiver of the method value holds ctx, as for g.Go(s.run)
// (#148, #192).
func goodDecoratorFreshCtxAndMethodErrgroup(ctx context.Context) {
	s := &ctxSvc{ctx: ctx}
	g := new(errgroup.Group)
	g.Go(logPanicErr(context.Background(), s.run))
	_ = g.Wait()
}

// [GOOD]: Decorator given a fresh context and a method value
//
// The bound receiver of the method value holds ctx, as for g.Go(s.run)
// (#148, #192).
func goodDecoratorFreshCtxAndMethodWaitGroup(ctx context.Context) {
	s := &ctxSvc{ctx: ctx}
	var wg sync.WaitGroup
	wg.Go(logPanic(context.TODO(), s.runNoErr))
	wg.Wait()
}

// [GOOD]: Decorator given a fresh context and a method value
//
// The bound receiver of the method value holds ctx, as for g.Go(s.run)
// (#148, #192).
func goodDecoratorFreshCtxAndMethodSpawner(ctx context.Context) {
	s := &ctxSvc{ctx: ctx}
	g := new(errgroup.Group)
	runWithGroup(g, logPanicErr(context.Background(), s.run))
	_ = g.Wait()
}

// [BAD]: Named function given a fresh context and a basic value
//
// A string holds no context, so the function gets only the fresh context.
func badNamedFuncFreshCtxAndStringGoroutine(ctx context.Context) {
	go runWithName(context.Background(), "worker") // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Factory given a fresh context and a basic value
//
// An int holds no context, so the factory gets only the fresh context.
func badFactoryFreshCtxAndIntErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(makeErrWorkerWithID(context.Background(), 1)) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Decorator given a fresh context and a named function
//
// A named function captures nothing, so the decorator gets only the fresh
// context.
func badDecoratorFreshCtxAndNamedFuncWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(logPanic(context.TODO(), noCtxWork)) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Factory given a fresh context and values of basic types
//
// A slice of strings and a struct of ints hold no context.
func badFactoryFreshCtxAndBasicPartsSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, makeErrWorkerWithParts(context.Background(), []string{"a"}, struct{ n, m int }{})) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

func runWithDeps(_ context.Context, d *ctxDeps) {
	_ = d.ctx
}

func runWithName(ctx context.Context, name string) {
	_, _ = ctx, name
}

func makeWorkerWithDeps(_ context.Context, d *ctxDeps) func() {
	return func() {
		_ = d.ctx
	}
}

func makeErrWorkerWithDeps(_ context.Context, d *ctxDeps) func() error {
	return func() error {
		_ = d.ctx
		return nil
	}
}

func makeErrWorkerWithID(ctx context.Context, id int) func() error {
	return func() error {
		_, _ = ctx, id
		return nil
	}
}

func makeErrWorkerWithParts(ctx context.Context, names []string, p struct{ n, m int }) func() error {
	return func() error {
		_, _, _ = ctx, names, p
		return nil
	}
}

// logPanic runs f, logging a panic to logCtx.
func logPanic(logCtx context.Context, f func()) func() {
	return func() {
		defer func() { _, _ = logCtx, recover() }()
		f()
	}
}

// logPanicErr runs f, logging a panic to logCtx.
func logPanicErr(logCtx context.Context, f func() error) func() error {
	return func() error {
		defer func() { _, _ = logCtx, recover() }()
		return f()
	}
}

func noCtxWork() {
	fmt.Println("no ctx")
}

// ===== Factory given a value that implements context.Context or holds one =====

// idleWrapperWorker makes a worker that does not use w.
func idleWrapperWorker(w ctxWrapper) func() error {
	return func() error { return nil }
}

// idleWrapperTask makes a task that does not use w.
func idleWrapperTask(w ctxWrapper) func() {
	return func() {}
}

// idleServiceWorker makes a worker that does not use s.
func idleServiceWorker(s *ctxService) func() {
	return func() {}
}

// [GOOD]: Factory given a value that implements context.Context
//
// The value counts as a context, so the factory passes as one given ctx
// does (#210).
func goodFactoryContextImplGoroutine(ctx context.Context, w ctxWrapper) {
	go idleWrapperTask(w)()
}

// [GOOD]: Factory given a value that implements context.Context
//
// The value counts as a context, so the factory passes as one given ctx
// does (#210).
func goodFactoryContextImplErrgroup(ctx context.Context, w ctxWrapper) {
	g := new(errgroup.Group)
	g.Go(idleWrapperWorker(w))
	_ = g.Wait()
}

// [GOOD]: Factory given a struct that holds a context
//
// The struct counts as a context, as it does captured and given to a call
// (#210).
func goodFactoryHolderWaitGroup(ctx context.Context, s *ctxService) {
	var wg sync.WaitGroup
	wg.Go(idleServiceWorker(s))
	wg.Wait()
}

// [GOOD]: Factory given a value that implements context.Context
//
// The value counts as a context, so the factory passes as one given ctx
// does (#210).
func goodFactoryContextImplSpawner(ctx context.Context, w ctxWrapper) {
	g := new(errgroup.Group)
	runWithGroup(g, idleWrapperWorker(w))
	_ = g.Wait()
}

// [BAD]: Factory given a struct that holds no context
//
// A struct with no context field is not a context.
func badFactoryPlainHolderErrgroup(ctx context.Context, p plainHolder) {
	g := new(errgroup.Group)
	g.Go(idlePlainWorker(p)) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// idlePlainWorker makes a worker that does not use p.
func idlePlainWorker(p plainHolder) func() error {
	return func() error { return nil }
}

// ===== Named function given a fresh context and a value that implements context.Context =====

// counterCtx is a context.Context of its own, with no field.
type counterCtx int

func (counterCtx) Deadline() (time.Time, bool) { return time.Time{}, false }
func (counterCtx) Done() <-chan struct{}       { return nil }
func (counterCtx) Err() error                  { return nil }
func (counterCtx) Value(any) any               { return nil }

func runWithCounter(_ context.Context, c counterCtx) { _ = c }

// [GOOD]: Named function given a fresh context and a value that implements context.Context
//
// The value may be the context the function uses, so it is not taken as
// given only a fresh context (#210).
func goodNamedFuncFreshCtxAndContextImplGoroutine(ctx context.Context, c counterCtx) {
	go runWithCounter(context.Background(), c)
}
