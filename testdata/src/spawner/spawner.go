// Package spawner tests the //goroutinectx:spawner directive. The cases it
// shares with other checkers are in testdata/src/patterns.
package spawner

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

// ===== SPAWNER FUNCTIONS =====

//goroutinectx:spawner
func runMultipleFuncs(fn1, fn2 func()) {
	go fn1()
	go fn2()
}

// ===== SHOULD REPORT =====

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

// [GOOD]: Multiple func args - both context usage
//
// Multiple function arguments, all use context.
func goodMultipleFuncs(ctx context.Context) {
	runMultipleFuncs(
		func() { _ = ctx },
		func() { _ = ctx },
	) // OK
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

// ===== NON-SPAWNER FUNCTIONS (should not be checked) =====

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
