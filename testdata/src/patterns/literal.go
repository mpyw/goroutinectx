// A func literal given directly, with and without ctx.

package patterns

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ===== Literal without ctx =====

// [BAD]: Literal without ctx
//
// Literal without ctx - basic bad case
func badGoroutineNoCapture(ctx context.Context) {
	go func() { // want `goroutine does not propagate context "ctx"`
		fmt.Println("no context")
	}()
}

// [BAD]: Literal without ctx
//
// Literal without ctx - basic bad case
func badErrgroupGo(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Literal without ctx
//
// Literal without ctx - basic bad case
func badWaitGroupGo(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
	})
	wg.Wait()
}

// [BAD]: Inline func literal context usage
//
// Inline function literal does not use context.
func badInlineFuncLiteral(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error { // want `runWithGroup\(\) func argument should use context "ctx"`
		fmt.Println("no ctx")
		return nil
	})
	_ = g.Wait()
}

// ===== Parenthesized literal =====

// [BAD]: Parenthesized literal
//
// Parentheses around the literal are looked through (#118).
func badParenGoroutine(ctx context.Context) {
	go (func() { // want `goroutine does not propagate context "ctx"`
		fmt.Println("no context")
	})()
}

// [BAD]: Parenthesized literal
//
// Parentheses around the literal are looked through (#118).
func badParenErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go((func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
		return nil
	}))
	_ = g.Wait()
}

// [BAD]: Parenthesized literal
//
// Parentheses around the literal are looked through (#118).
func badParenWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go((func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
	}))
	wg.Wait()
}

// [BAD]: Parenthesized literal
//
// Parentheses around the literal are looked through (#118).
func badParenSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, (func() error { // want `runWithGroup\(\) func argument should use context "ctx"`
		fmt.Println("no context")
		return nil
	}))
	_ = g.Wait()
}

// [GOOD]: Parenthesized literal with ctx
//
// A parenthesized literal that uses ctx passes.
func goodParenGoroutine(ctx context.Context) {
	go (func() {
		_ = ctx
	})()
}

// ===== Literal with ctx =====

// [GOOD]: Literal with ctx - basic good case
//
// Closure directly references the context variable from enclosing scope.
func goodGoroutineCapturesCtx(ctx context.Context) {
	go func() {
		_ = ctx.Done()
	}()
}

// [GOOD]: Literal with ctx - basic good case
//
// Closure directly references the context variable from enclosing scope.
func goodErrgroupGoWithCtx(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx.Done()
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Literal with ctx - basic good case
//
// Closure directly references the context variable from enclosing scope.
func goodWaitGroupGoWithCtx(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = ctx.Done()
	})
	wg.Wait()
}

// [GOOD]: Inline func literal context usage
//
// Inline function literal properly uses context.
func goodInlineFuncLiteral(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error {
		_ = ctx
		return nil
	}) // OK
	_ = g.Wait()
}

// ===== Literal with ctx - via function call =====

// [GOOD]: Literal with ctx - via function call
//
// Context is passed to helper function inside closure.
func goodGoroutineUsesCtxInCall(ctx context.Context) {
	go func() {
		doSomething(ctx)
	}()
}

// [GOOD]: Literal with ctx - via function call
//
// Context is passed to helper function inside closure.
func goodErrgroupGoCallsWithCtx(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		return doWork(ctx)
	})
	_ = g.Wait()
}

// [GOOD]: Literal with ctx - via function call
//
// Context is passed to helper function inside closure.
func goodWaitGroupGoCallsWithCtx(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		doSomething(ctx)
	})
	wg.Wait()
}

// ===== No ctx param =====

// [GOOD]: No ctx param
//
// No ctx param - not checked
func goodNoContextParamGoroutine() {
	go func() {
		fmt.Println("hello")
	}()
}

// [GOOD]: No ctx param
//
// No ctx param - not checked
func goodNoContextParamErrgroup() {
	g := new(errgroup.Group)
	g.Go(func() error {
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: No ctx param
//
// No ctx param - not checked
func goodNoContextParamWaitGroup() {
	var wg sync.WaitGroup
	wg.Go(func() {
		fmt.Println("hello")
	})
	wg.Wait()
}

// [GOOD]: No ctx param
//
// No ctx param - not checked
func goodNoCtxParam() {
	g := new(errgroup.Group)
	runWithGroup(g, func() error {
		fmt.Println("no ctx")
		return nil
	}) // OK - no ctx in scope
	_ = g.Wait()
}

// ===== Blank ctx param =====

// [GOOD]: Blank ctx param
//
// A blank ctx param cannot be used, so nothing is checked (#114).
func goodBlankContextParamGoroutine(_ context.Context) {
	go func() {
		fmt.Println("hello")
	}()
}

// [GOOD]: Blank ctx param
//
// A blank ctx param cannot be used, so nothing is checked (#114).
func goodBlankContextParamErrgroup(_ context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Blank ctx param
//
// A blank ctx param cannot be used, so nothing is checked (#114).
func goodBlankContextParamWaitGroup(_ context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		fmt.Println("hello")
	})
	wg.Wait()
}

// [GOOD]: Blank ctx param
//
// A blank ctx param cannot be used, so nothing is checked (#114).
func goodBlankContextParamSpawner(_ context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error {
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Blank ctx param beside a named one
//
// The named ctx param is still checked.
func badBlankAndNamedContextParamGoroutine(_ context.Context, ctx context.Context) {
	go func() { // want `goroutine does not propagate context "ctx"`
		fmt.Println("hello")
	}()
}

// ===== Multiple Go calls without ctx =====

// [BAD]: Multiple Go calls without ctx
//
// Multiple goroutine closures all fail to use the available context.
func badErrgroupGoMultiple(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		return nil
	})
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Multiple Go calls without ctx
//
// Multiple goroutine closures all fail to use the available context.
func badWaitGroupGoMultiple(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	})
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	})
	wg.Wait()
}

func doSomething(ctx context.Context) {
	_ = ctx
}

func doWork(ctx context.Context) error {
	_ = ctx
	return nil
}
