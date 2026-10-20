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

// ===== UNTRACEABLE PATTERNS (FUNCTION PARAMETERS) =====

// [LIMITATION]: Function from parameter - cannot trace
//
// Function passed as parameter cannot be traced statically.
// We assume it's OK (zero false positives policy).
func limitationFuncFromParameter(ctx context.Context, fn func() error) {
	g := new(errgroup.Group)
	// fn comes from parameter - can't trace its body
	runWithGroup(g, fn) // No error - can't trace parameter function
	_ = g.Wait()
}

// [LIMITATION]: Function from variadic parameter - cannot trace
//
// Functions passed as variadic parameter cannot be traced statically.
func limitationFuncFromVariadicParameter(ctx context.Context, fns ...func()) {
	var wg sync.WaitGroup
	for _, fn := range fns {
		runWithWaitGroup(&wg, fn) // No error - can't trace parameter function
	}
	wg.Wait()
}

// ===== IIFE FACTORY PATTERNS =====

// [BAD]: IIFE factory with parentheses
//
// The parenthesized factory is resolved through SSA like any other.
func badIIFEFactoryWithParentheses(ctx context.Context) {
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
func goodInlineFactoryVariableWithCtx(ctx context.Context) {
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
func badInlineFactoryVariableWithoutCtx(ctx context.Context) {
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
