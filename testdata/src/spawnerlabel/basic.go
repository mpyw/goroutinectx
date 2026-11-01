// Package spawnerlabel tests the spawnerlabel checker.
package spawnerlabel

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	gotask "github.com/siketyan/gotask/v2"
)

// ===== MISSING LABEL - SHOULD REPORT =====

// [BAD]: Missing label - passes its func param to errgroup.Group.Go
func missingLabelErrgroup(task func() error) { // want `function "missingLabelErrgroup" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	g.Go(task)
	_ = g.Wait()
}

// [GOOD]: Passes its own literal to errgroup.Group.Go (#377)
func goodErrgroupLiteral() {
	g := new(errgroup.Group)
	g.Go(func() error {
		fmt.Println("work")
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Missing label - passes its func param to errgroup.Group.TryGo
func missingLabelErrgroupTryGo(task func() error) { // want `function "missingLabelErrgroupTryGo" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.TryGo with func argument\)`
	g := new(errgroup.Group)
	g.TryGo(task)
	_ = g.Wait()
}

// [GOOD]: Passes its own literal to errgroup.Group.TryGo (#377)
func goodErrgroupTryGoLiteral() {
	g := new(errgroup.Group)
	g.TryGo(func() error {
		fmt.Println("work")
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Missing label - passes its func param to gotask.DoAllFnsSettled
func missingLabelGotaskDoAll(ctx context.Context, fn func(context.Context) error) { // want `function "missingLabelGotaskDoAll" should have //goroutinectx:spawner directive \(calls gotask\.DoAllFnsSettled with func argument\)`
	gotask.DoAllFnsSettled(ctx, fn)
}

// [GOOD]: Passes its own literal to gotask.DoAllFnsSettled (#377)
func goodGotaskDoAllLiteral(ctx context.Context) {
	gotask.DoAllFnsSettled(ctx,
		func(ctx context.Context) error {
			_ = ctx
			return nil
		},
	)
}

// [BAD]: Missing label - runs a task made of its func param with DoAsync
func missingLabelGotaskDoAsync(ctx context.Context, fn func(context.Context) error) { // want `function "missingLabelGotaskDoAsync" should have //goroutinectx:spawner directive \(calls gotask\.Task\.DoAsync with func argument\)`
	task := gotask.NewTask(fn)
	ch := make(chan error)
	task.DoAsync(ctx, ch)
}

// [GOOD]: Runs a task made of its own literal with DoAsync (#377)
func goodGotaskDoAsyncLiteral(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		_ = ctx
		return nil
	})
	ch := make(chan error)
	task.DoAsync(ctx, ch)
}

// [BAD]: Missing label - passes its func param to a marked spawner
func missingLabelIndirectSpawner(fn func() error) { // want `function "missingLabelIndirectSpawner" should have //goroutinectx:spawner directive \(calls runWithGroup with func argument\)`
	g := new(errgroup.Group)
	runWithGroup(g, fn)
	_ = g.Wait()
}

// [GOOD]: Passes its own literal to a marked spawner (#377)
func goodIndirectSpawnerLiteral() {
	g := new(errgroup.Group)
	runWithGroup(g, func() error {
		fmt.Println("work")
		return nil
	})
	_ = g.Wait()
}

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

// [BAD]: Unnecessary label - spawns only its own literal (#377)
//
//goroutinectx:spawner
func unnecessaryLabelOwnLiteral() { // want `function "unnecessaryLabelOwnLiteral" has unnecessary //goroutinectx:spawner directive`
	g := new(errgroup.Group)
	g.Go(func() error {
		fmt.Println("work")
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Unnecessary label - has a func param, but never uses it
//
//goroutinectx:spawner
func unnecessaryLabelUnusedParam(fn func()) { // want `function "unnecessaryLabelUnusedParam" has unnecessary //goroutinectx:spawner directive`
	_ = fn
}

// ===== SHOULD NOT REPORT =====

// [GOOD]: Properly labeled function that forwards its func param
//
//goroutinectx:spawner
func goodProperlyLabeled(task func() error) {
	g := new(errgroup.Group)
	g.Go(task)
	_ = g.Wait()
}

// [GOOD]: Function without spawn calls, no label, no func params
func goodNoLabelNoSpawn() {
	fmt.Println("just a regular function")
}

// [GOOD]: Function with func param that it stores, has label
//
//goroutinectx:spawner
func goodFuncParamWithLabel(fn func()) {
	// It may run later, where the checker cannot see - label is kept
	pending = append(pending, fn)
}

// pending holds the funcs goodFuncParamWithLabel stores.
var pending []func()

// [GOOD]: Ignore directive on missing label
//
//goroutinectx:ignore
func goodIgnoreDirective(task func() error) {
	g := new(errgroup.Group)
	g.Go(task)
	_ = g.Wait()
}

// [GOOD]: Function without body (interface method simulation)
// Go doesn't have abstract methods, but external funcs have no body

// [GOOD]: Spawner inside nested closure - only outermost function matters
func goodNestedSpawnInClosure(task func() error) {
	fn := func() {
		g := new(errgroup.Group)
		g.Go(task)
		_ = g.Wait()
	}
	_ = fn // Not actually calling, just assigning
}

// ===== EDGE CASES =====

// [GOOD]: Method on type - should be treated same as function
type SpawnerType struct{}

// [GOOD]: Method with spawner label
//
// Method with spawner label that forwards its func param
//
//goroutinectx:spawner
func (s *SpawnerType) SpawnWork(task func() error) {
	g := new(errgroup.Group)
	g.Go(task)
	_ = g.Wait()
}

// [BAD]: Method with spawner label
//
// Method missing spawner label
func (s *SpawnerType) MissingLabelMethod(task func() error) { // want `function "MissingLabelMethod" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	g.Go(task)
	_ = g.Wait()
}

// [GOOD]: Method that spawns its own literal (#377)
func (s *SpawnerType) goodMethodLiteral() {
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
