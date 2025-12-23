// Package spawnerlabel tests the spawnerlabel checker - evil edge cases.
package spawnerlabel

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	gotask "github.com/siketyan/gotask/v2"
)

// ===== DEEP NESTING =====

// [BAD]: Spawn call deep in nested blocks
func evilDeepNesting() { // want `function "evilDeepNesting" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	if true {
		for i := 0; i < 1; i++ {
			switch i {
			case 0:
				g.Go(func() error {
					return nil
				})
			}
		}
	}
	_ = g.Wait()
}

// ===== CONDITIONAL PATHS =====

// [BAD]: Spawn only in one branch
func evilConditionalSpawn(cond bool) { // want `function "evilConditionalSpawn" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	if cond {
		g.Go(func() error {
			return nil
		})
	}
	_ = g.Wait()
}

// [BAD]: Spawn only in else branch
func evilElseOnlySpawn(cond bool) { // want `function "evilElseOnlySpawn" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	if cond {
		fmt.Println("no spawn")
	} else {
		g.Go(func() error {
			return nil
		})
	}
	_ = g.Wait()
}

// ===== DEFER =====

// [BAD]: Spawn in defer IIFE - SSA detects spawn in nested closure
func badDeferSpawnNestedScope() { // want `function "badDeferSpawnNestedScope" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	defer func() {
		g.Go(func() error {
			return nil
		})
	}()
	_ = g.Wait()
}

// ===== MULTIPLE SPAWN METHODS =====

// [BAD]: Multiple spawn methods - missing label
func evilMultipleSpawnMethods() { // want `function "evilMultipleSpawnMethods" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	g.Go(func() error {
		return nil
	})
	g.TryGo(func() error {
		return nil
	})
}

// ===== CHAIN OF SPAWNER CALLS =====

// [BAD]: Calls two spawner-marked functions
func evilChainedSpawners() { // want `function "evilChainedSpawners" should have //goroutinectx:spawner directive \(calls runWithGroup with func argument\)`
	g := new(errgroup.Group)
	runWithGroup(g, func() error {
		return nil
	})
	runWithGroup(g, func() error {
		return nil
	})
	_ = g.Wait()
}

// ===== GOTASK VARIANTS =====

// [GOOD]: gotask.DoAll with Task - Task is not a func argument
func goodGotaskDoAllWithTask(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) gotask.Result[int] {
		_ = ctx
		return gotask.Result[int]{Value: 42}
	})
	gotask.DoAll(ctx, task)
}

// [GOOD]: gotask.DoRace with Task - Task is not a func argument
func goodGotaskDoRaceWithTask(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) int {
		_ = ctx
		return 42
	})
	gotask.DoRace(ctx, task)
}

// [BAD]: gotask.DoAllFns missing label - takes func arguments directly
func evilGotaskDoAllFns(ctx context.Context) { // want `function "evilGotaskDoAllFns" should have //goroutinectx:spawner directive \(calls gotask\.DoAllFns with func argument\)`
	gotask.DoAllFns(ctx,
		func(ctx context.Context) gotask.Result[int] {
			_ = ctx
			return gotask.Result[int]{Value: 42}
		},
	)
}

// ===== NESTED CLOSURE =====

// [BAD]: Spawn in nested function literal - SSA detects spawn
func badNestedFuncLitSpawn() { // want `function "badNestedFuncLitSpawn" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	outer := func() {
		g := new(errgroup.Group)
		g.Go(func() error {
			return nil
		})
		_ = g.Wait()
	}
	outer()
}

// Interface method calls - can't determine spawn statically
type Runner interface {
	Run(fn func())
}

// [LIMITATION]: Interface method call spawn not detected
//
// Interface method calls - can't determine spawn statically
func limitationInterfaceCall(r Runner) {
	r.Run(func() {
		fmt.Println("work")
	})
}

// [LIMITATION]: Higher-order return spawn not detected
//
// Higher-order return - can't trace through function return
func limitationHigherOrderReturn() {
	fn := getSpawnerFunc()
	fn()
}

//vt:helper
func getSpawnerFunc() func() {
	return func() {
		g := new(errgroup.Group)
		g.Go(func() error {
			return nil
		})
		_ = g.Wait()
	}
}

// [LIMITATION]: Channel receive spawn not detected
//
// Channel receive - can't trace func from channel
func limitationChannelReceive(ch chan func()) {
	fn := <-ch
	_ = fn
}

// ===== UNNECESSARY LABEL EDGE CASES =====

// [GOOD]: Has func param but no spawn - label is justified (wrapper pattern)
//
//goroutinectx:spawner
func goodFuncParamNoSpawn(callback func() error) error {
	return callback()
}

// [BAD]: Has label, no spawn, no func param
//
//goroutinectx:spawner
func (r *receiverType) unnecessaryLabelOnMethod() { // want `function "unnecessaryLabelOnMethod" has unnecessary //goroutinectx:spawner directive`
	fmt.Println("no spawn")
}

type receiverType struct{}

// [GOOD]: Has label and spawns via variable func
//
//goroutinectx:spawner
func goodSpawnViaVariable() {
	g := new(errgroup.Group)
	fn := func() error {
		return nil
	}
	g.Go(fn)
	_ = g.Wait()
}

// ===== MORE EDGE CASES =====

// [BAD]: Spawn in switch statement
func spawnInSwitch(n int) { // want `function "spawnInSwitch" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	switch n {
	case 1:
		g.Go(func() error {
			return nil
		})
	case 2:
		fmt.Println("no spawn")
	default:
		g.Go(func() error {
			return nil
		})
	}
	_ = g.Wait()
}

// [BAD]: Spawn in select statement
func spawnInSelect(ch chan int) { // want `function "spawnInSelect" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	select {
	case <-ch:
		g.Go(func() error {
			return nil
		})
	default:
		fmt.Println("default")
	}
	_ = g.Wait()
}

// [BAD]: Spawn with early return before
func spawnAfterEarlyReturn(cond bool) { // want `function "spawnAfterEarlyReturn" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	if cond {
		return
	}
	g.Go(func() error {
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Multiple different spawn methods in one function
func multipleSpawnTypes() { // want `function "multipleSpawnTypes" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	g.Go(func() error {
		return nil
	})
	g.TryGo(func() error {
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Struct with func field - field doesn't count as func param
type TaskHolder struct {
	Task func() error
}

// [BAD]: Struct with func field - field does not count as func param
//
//goroutinectx:spawner
func unnecessaryWithStructField() { // want `function "unnecessaryWithStructField" has unnecessary //goroutinectx:spawner directive`
	h := TaskHolder{Task: func() error { return nil }}
	_ = h.Task()
}

type Executor struct{}

// [GOOD]: Method with func param - justified label
//
//goroutinectx:spawner
func (e *Executor) Execute(task func()) {
	go task()
}

// [BAD]: Method calls another spawner method
func (e *Executor) Wrapper() { // want `function "Wrapper" should have //goroutinectx:spawner directive \(calls Execute with func argument\)`
	e.Execute(func() {
		fmt.Println("work")
	})
}

// [GOOD]: Function param is func returning func - justified label
//
//goroutinectx:spawner
func funcReturningFunc(factory func() func()) {
	fn := factory()
	go fn()
}

// [BAD]: Spawn in range loop
func spawnInRangeLoop(items []int) { // want `function "spawnInRangeLoop" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	for _, item := range items {
		item := item
		g.Go(func() error {
			_ = item
			return nil
		})
	}
	_ = g.Wait()
}

// [GOOD]: Only creates group but doesn't spawn
func onlyCreateGroup() {
	g := new(errgroup.Group)
	_ = g.Wait()
}

// [BAD]: IIFE spawn - SSA detects spawn in IIFE
func badIIFEWithSpawn() { // want `function "badIIFEWithSpawn" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	func() {
		g.Go(func() error {
			return nil
		})
	}()
	_ = g.Wait()
}

// [BAD]: Spawn in type assertion branch
func spawnInTypeAssertion(v interface{}) { // want `function "spawnInTypeAssertion" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	if _, ok := v.(int); ok {
		g.Go(func() error {
			return nil
		})
	}
	_ = g.Wait()
}

// [BAD]: Very deep nesting - should still be detected
func deepNesting() { // want `function "deepNesting" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	if true {
		if true {
			if true {
				for i := 0; i < 1; i++ {
					g.Go(func() error {
						return nil
					})
				}
			}
		}
	}
	_ = g.Wait()
}

// [LIMITATION]: Unreachable spawn after panic - SSA eliminates unreachable code
//
// SSA optimizes away code after panic()
func limitationPanicBeforeSpawn() {
	g := new(errgroup.Group)
	panic("oops")
	g.Go(func() error {
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Closed over spawn call in returned func
func closedOverSpawn() func() {
	return func() {
		g := new(errgroup.Group)
		g.Go(func() error {
			return nil
		})
		_ = g.Wait()
	}
}

// [BAD]: Named return with spawn
func namedReturnWithSpawn() (err error) { // want `function "namedReturnWithSpawn" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	g.Go(func() error {
		return nil
	})
	err = g.Wait()
	return
}

// [BAD]: IIFE containing spawn call
//
// IIFE that spawns a goroutine - SSA traces into IIFE.
func badIIFEContainingSpawn() { // want `function "badIIFEContainingSpawn" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	func() {
		g.Go(func() error {
			return nil
		})
	}()
	_ = g.Wait()
}

// [BAD]: Nested IIFE containing spawn call
//
// Nested IIFE that spawns a goroutine.
func badNestedIIFEContainingSpawn() { // want `function "badNestedIIFEContainingSpawn" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	func() {
		func() {
			g.Go(func() error {
				return nil
			})
		}()
	}()
	_ = g.Wait()
}
