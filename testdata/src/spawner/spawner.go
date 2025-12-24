// Package spawner tests the //goroutinectx:spawner directive.
package spawner

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ===== SPAWNER FUNCTIONS =====

//goroutinectx:spawner //vt:helper
func runWithGroup(g *errgroup.Group, fn func() error) {
	g.Go(fn)
}

//goroutinectx:spawner //vt:helper
func runWithWaitGroup(wg *sync.WaitGroup, fn func()) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		fn()
	}()
}

//goroutinectx:spawner //vt:helper
func runMultipleFuncs(fn1, fn2 func()) {
	go fn1()
	go fn2()
}

// ===== SHOULD REPORT =====

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

// [BAD]: Multiple func args - both context usage
//
// Multiple function arguments, none use context.
func badMultipleFuncs(ctx context.Context) {
	runMultipleFuncs(
		func() { fmt.Println("no ctx 1") }, // want `runMultipleFuncs\(\) func argument should use context "ctx"`
		func() { fmt.Println("no ctx 2") }, // want `runMultipleFuncs\(\) func argument should use context "ctx"`
	)
}

// [BAD]: Multiple func args - first bad
//
// First function argument does not use context.
func badFirstFuncOnly(ctx context.Context) {
	runMultipleFuncs(
		func() { fmt.Println("no ctx") }, // want `runMultipleFuncs\(\) func argument should use context "ctx"`
		func() { _ = ctx },
	)
}

// [BAD]: Multiple func args - second bad
//
// Second function argument does not use context.
func badSecondFuncOnly(ctx context.Context) {
	runMultipleFuncs(
		func() { _ = ctx },
		func() { fmt.Println("no ctx") }, // want `runMultipleFuncs\(\) func argument should use context "ctx"`
	)
}

// ===== SHOULD NOT REPORT =====

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

// [GOOD]: Multiple func args - both context usage
//
// Multiple function arguments, all use context.
func goodMultipleFuncs(ctx context.Context) {
	runMultipleFuncs(
		func() { _ = ctx },
		func() { _ = ctx },
	) // OK
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

// [GOOD]: Func has own ctx param
//
// Function declares own context parameter, outer context not required.
func goodFuncHasOwnCtx(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func(innerCtx context.Context) error {
		_ = innerCtx
		return nil
	}
	// Note: runWithGroup expects func() error, not func(context.Context) error
	// This pattern is valid when the function declares its own context
	_ = fn
	_ = g
}

// ===== FACTORY FUNCTION PATTERNS =====

//vt:helper
func makeWorker() func() error {
	return func() error {
		fmt.Println("no ctx")
		return nil
	}
}

//vt:helper
func makeWorkerWithCtx(ctx context.Context) func() error {
	return func() error {
		_ = ctx
		return nil
	}
}

// [GOOD]: Factory function with context
//
// Factory function that takes context and returns func using it.
func goodFactoryFunctionWithCtx(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, makeWorkerWithCtx(ctx)) // OK - factory takes ctx and returns func using it
	_ = g.Wait()
}

// [BAD]: Factory function with context
//
// Factory function returns func not using context.
func badFactoryFunctionNoCtx(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, makeWorker()) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== NON-SPAWNER FUNCTIONS (should not be checked) =====

//vt:helper
func normalHelper(g *errgroup.Group, fn func() error) {
	g.Go(fn)
}

// [GOOD]: Call to non-spawner function
//
// Call to non-spawner function - not checked
func goodNonSpawnerFunction(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error {
		fmt.Println("no ctx")
		return nil
	}
	normalHelper(g, fn) // OK - normalHelper is not marked as spawner
	_ = g.Wait()
}
