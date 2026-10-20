// Package spawnerlabel tests the spawnerlabel checker.
package spawnerlabel

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	gotask "github.com/siketyan/gotask/v2"
)

// ===== MISSING LABEL - SHOULD REPORT =====

// [BAD]: Missing label - calls errgroup.Group.Go with func arg
func missingLabelErrgroup() { // want `function "missingLabelErrgroup" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	g.Go(func() error {
		fmt.Println("work")
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Missing label - calls errgroup.Group.TryGo with func arg
func missingLabelErrgroupTryGo() { // want `function "missingLabelErrgroupTryGo" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.TryGo with func argument\)`
	g := new(errgroup.Group)
	g.TryGo(func() error {
		fmt.Println("work")
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Missing label - calls gotask.DoAllFnsSettled with func arg
func missingLabelGotaskDoAll(ctx context.Context) { // want `function "missingLabelGotaskDoAll" should have //goroutinectx:spawner directive \(calls gotask\.DoAllFnsSettled with func argument\)`
	gotask.DoAllFnsSettled(ctx,
		func(ctx context.Context) error {
			_ = ctx
			return nil
		},
	)
}

// [BAD]: Missing label - calls gotask.Task.DoAsync
func missingLabelGotaskDoAsync(ctx context.Context) { // want `function "missingLabelGotaskDoAsync" should have //goroutinectx:spawner directive \(calls gotask\.Task\.DoAsync with func argument\)`
	task := gotask.NewTask(func(ctx context.Context) error {
		_ = ctx
		return nil
	})
	ch := make(chan error)
	task.DoAsync(ctx, ch)
}

// [BAD]: Missing label - indirect spawner call (errgroup wrapper)
func missingLabelIndirectSpawner() { // want `function "missingLabelIndirectSpawner" should have //goroutinectx:spawner directive \(calls runWithGroup with func argument\)`
	g := new(errgroup.Group)
	runWithGroup(g, func() error {
		fmt.Println("work")
		return nil
	})
	_ = g.Wait()
}

//vt:helper
//goroutinectx:spawner
func runWithGroup(g *errgroup.Group, fn func() error) {
	g.Go(fn)
}

// ===== UNNECESSARY LABEL - SHOULD REPORT =====

// [BAD]: Unnecessary label - no spawn calls and no func params
//
//goroutinectx:spawner
func unnecessaryLabelNoSpawn() { // want `function "unnecessaryLabelNoSpawn" has unnecessary //goroutinectx:spawner directive`
	fmt.Println("just a regular function")
}

// [BAD]: Unnecessary label - has spawn call but without func arg
//
//goroutinectx:spawner
func unnecessaryLabelNoFuncArg() { // want `function "unnecessaryLabelNoFuncArg" has unnecessary //goroutinectx:spawner directive`
	// This doesn't actually compile - errgroup.Go needs a func argument
	// But for demonstration, showing that just calling methods isn't enough
	g := new(errgroup.Group)
	_ = g.Wait() // No Go() call
}

// ===== SHOULD NOT REPORT =====

// [GOOD]: Properly labeled function with spawn call
//
//goroutinectx:spawner
func goodProperlyLabeled() {
	g := new(errgroup.Group)
	g.Go(func() error {
		fmt.Println("work")
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Function without spawn calls, no label, no func params
func goodNoLabelNoSpawn() {
	fmt.Println("just a regular function")
}

// [GOOD]: Function with func param, no spawn call, has label
//
//goroutinectx:spawner
func goodFuncParamWithLabel(fn func()) {
	// This could spawn in a subtype/mock/etc - label is justified
	_ = fn
}

// [GOOD]: Ignore directive on missing label
//
//goroutinectx:ignore
func goodIgnoreDirective() {
	g := new(errgroup.Group)
	g.Go(func() error {
		fmt.Println("work")
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Function without body (interface method simulation)
// Go doesn't have abstract methods, but external funcs have no body

// [GOOD]: Spawner inside nested closure - only outermost function matters
func goodNestedSpawnInClosure() {
	fn := func() {
		g := new(errgroup.Group)
		g.Go(func() error {
			return nil
		})
		_ = g.Wait()
	}
	_ = fn // Not actually calling, just assigning
}

// ===== EDGE CASES =====

// [GOOD]: Method on type - should be treated same as function
type SpawnerType struct{}

// [GOOD]: Method with spawner label
//
// Method with spawner label and spawn call
//
//goroutinectx:spawner
func (s *SpawnerType) SpawnWork() { // want SpawnWork:"spawner"
	g := new(errgroup.Group)
	g.Go(func() error {
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Method with spawner label
//
// Method missing spawner label
func (s *SpawnerType) MissingLabelMethod() { // want `function "MissingLabelMethod" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	g.Go(func() error {
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Function with variadic func params
//
//goroutinectx:spawner
func goodVariadicFuncParams(fns ...func()) {
	for _, fn := range fns {
		go fn()
	}
}
