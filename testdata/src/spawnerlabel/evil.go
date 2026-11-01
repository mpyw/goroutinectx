// Package spawnerlabel tests the spawnerlabel checker - evil edge cases.
package spawnerlabel

import (
	"context"
	"fmt"
	"slices"

	"golang.org/x/sync/errgroup"

	gotask "github.com/siketyan/gotask/v2"
)

// ===== DEEP NESTING =====

// [BAD]: Forwarding spawn call deep in nested blocks
func evilDeepNesting(task func() error) { // want `function "evilDeepNesting" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	if true {
		for i := 0; i < 1; i++ {
			switch i {
			case 0:
				g.Go(task)
			}
		}
	}
	_ = g.Wait()
}

// [GOOD]: Spawn of its own literal deep in nested blocks (#377)
func goodDeepNestingLiteral() {
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

// [BAD]: Forwarding spawn in a range-over-func loop body
func evilRangeFuncSpawn(xs []int, task func() error) { // want `function "evilRangeFuncSpawn" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	for range slices.Values(xs) {
		g.Go(task)
	}
	_ = g.Wait()
}

// [GOOD]: Spawn of its own literal in a range-over-func loop body (#377)
func goodRangeFuncSpawnLiteral(xs []int) {
	g := new(errgroup.Group)
	for range slices.Values(xs) {
		g.Go(func() error {
			return nil
		})
	}
	_ = g.Wait()
}

// ===== CONDITIONAL PATHS =====

// [BAD]: Forwarding spawn only in one branch
func evilConditionalSpawn(cond bool, task func() error) { // want `function "evilConditionalSpawn" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	if cond {
		g.Go(task)
	}
	_ = g.Wait()
}

// [BAD]: Forwarding spawn only in else branch
func evilElseOnlySpawn(cond bool, task func() error) { // want `function "evilElseOnlySpawn" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	if cond {
		fmt.Println("no spawn")
	} else {
		g.Go(task)
	}
	_ = g.Wait()
}

// [BAD]: Phi of its param and a literal
//
// The value spawned may be the param.
func evilParamOrLiteral(cond bool, task func() error) { // want `function "evilParamOrLiteral" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	f := func() error { return nil }
	if cond {
		f = task
	}
	g.Go(f)
	_ = g.Wait()
}

// ===== DEFER =====

// [BAD]: Forwarding spawn in defer IIFE - SSA detects spawn in nested closure
func badDeferSpawnNestedScope(task func() error) { // want `function "badDeferSpawnNestedScope" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	defer func() {
		g.Go(task)
	}()
	_ = g.Wait()
}

// ===== MULTIPLE SPAWN METHODS =====

// [BAD]: Multiple spawn methods - the second forwards its param
func evilMultipleSpawnMethods(task func() error) { // want `function "evilMultipleSpawnMethods" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.TryGo with func argument\)`
	g := new(errgroup.Group)
	g.Go(func() error {
		return nil
	})
	g.TryGo(task)
}

// ===== CHAIN OF SPAWNER CALLS =====

// [BAD]: Calls two spawner-marked functions, one with its param
func evilChainedSpawners(task func() error) { // want `function "evilChainedSpawners" should have //goroutinectx:spawner directive \(calls runWithGroup with func argument\)`
	g := new(errgroup.Group)
	runWithGroup(g, func() error {
		return nil
	})
	runWithGroup(g, task)
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

// [BAD]: gotask.DoAllFns missing label - forwards its func param
func evilGotaskDoAllFns(ctx context.Context, fn func(context.Context) gotask.Result[int]) { // want `function "evilGotaskDoAllFns" should have //goroutinectx:spawner directive \(calls gotask\.DoAllFns with func argument\)`
	gotask.DoAllFns(ctx, fn)
}

// [GOOD]: gotask.DoAllFns with its own literal (#377)
func goodGotaskDoAllFnsLiteral(ctx context.Context) {
	gotask.DoAllFns(ctx,
		func(ctx context.Context) gotask.Result[int] {
			_ = ctx
			return gotask.Result[int]{Value: 42}
		},
	)
}

// ===== NESTED CLOSURE =====

// [BAD]: Forwarding spawn in nested function literal - SSA detects spawn
func badNestedFuncLitSpawn(task func() error) { // want `function "badNestedFuncLitSpawn" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	outer := func() {
		g := new(errgroup.Group)
		g.Go(task)
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
func limitationInterfaceCall(r Runner, fn func()) {
	r.Run(fn)
}

// [GOOD]: Higher-order return spawn of its own literal (#137, #377)
//
// Calling what a function returns runs the closure it returns, and the
// closure spawns its own literal, not a func param of this function.
func goodHigherOrderReturn() {
	fn := getSpawnerFunc()
	fn()
}

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

// [BAD]: Has func param but only calls it in place - the label is not needed
// (#377)
//
//goroutinectx:spawner
func unnecessaryFuncParamNoSpawn(callback func() error) error { // want `function "unnecessaryFuncParamNoSpawn" has unnecessary //goroutinectx:spawner directive`
	return callback()
}

// [GOOD]: Has func param and passes it on to a function it cannot see into
//
//goroutinectx:spawner
func goodFuncParamPassedOn(r Runner, callback func()) {
	r.Run(callback)
}

// [BAD]: Has label, no spawn, no func param
//
//goroutinectx:spawner
func (r *receiverType) unnecessaryLabelOnMethod() { // want `function "unnecessaryLabelOnMethod" has unnecessary //goroutinectx:spawner directive`
	fmt.Println("no spawn")
}

type receiverType struct{}

// [BAD]: Has label and spawns only its own variable func (#377)
//
//goroutinectx:spawner
func unnecessarySpawnViaVariable() { // want `function "unnecessarySpawnViaVariable" has unnecessary //goroutinectx:spawner directive`
	g := new(errgroup.Group)
	fn := func() error {
		return nil
	}
	g.Go(fn)
	_ = g.Wait()
}

// [GOOD]: Has label and spawns its param through a variable
//
//goroutinectx:spawner
func goodSpawnViaVariable(task func() error) {
	g := new(errgroup.Group)
	fn := task
	g.Go(fn)
	_ = g.Wait()
}

// ===== MORE EDGE CASES =====

// [BAD]: Forwarding spawn in switch statement
func spawnInSwitch(n int, task func() error) { // want `function "spawnInSwitch" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	switch n {
	case 1:
		g.Go(func() error {
			return nil
		})
	case 2:
		fmt.Println("no spawn")
	default:
		g.Go(task)
	}
	_ = g.Wait()
}

// [BAD]: Forwarding spawn in select statement
func spawnInSelect(ch chan int, task func() error) { // want `function "spawnInSelect" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	select {
	case <-ch:
		g.Go(task)
	default:
		fmt.Println("default")
	}
	_ = g.Wait()
}

// [BAD]: Forwarding spawn with early return before
func spawnAfterEarlyReturn(cond bool, task func() error) { // want `function "spawnAfterEarlyReturn" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	if cond {
		return
	}
	g.Go(task)
	_ = g.Wait()
}

// [BAD]: Multiple different spawn methods in one function
func multipleSpawnTypes(task func() error) { // want `function "multipleSpawnTypes" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	g.Go(task)
	g.TryGo(task)
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

// [BAD]: Method forwards its param to another spawner method
func (e *Executor) Wrapper(task func()) { // want `function "Wrapper" should have //goroutinectx:spawner directive \(calls Execute with func argument\)`
	e.Execute(task)
}

// [GOOD]: Method gives another spawner method its own literal (#377)
func (e *Executor) goodWrapperLiteral() {
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

// [BAD]: Forwarding spawn in range loop
func spawnInRangeLoop(items []int, task func() error) { // want `function "spawnInRangeLoop" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	for _, item := range items {
		item := item
		g.Go(func() error {
			_ = item
			return task()
		})
	}
	_ = g.Wait()
}

// [GOOD]: Spawn of its own literal in range loop (#377)
func goodSpawnInRangeLoopLiteral(items []int) {
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

// [BAD]: Forwarding IIFE spawn - SSA detects spawn in IIFE
func badIIFEWithSpawn(task func() error) { // want `function "badIIFEWithSpawn" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	func() {
		g.Go(task)
	}()
	_ = g.Wait()
}

// [GOOD]: IIFE spawn of its own literal (#377)
func goodIIFEWithSpawnLiteral() {
	g := new(errgroup.Group)
	func() {
		g.Go(func() error {
			return nil
		})
	}()
	_ = g.Wait()
}

// [BAD]: Forwarding spawn in type assertion branch
func spawnInTypeAssertion(v interface{}, task func() error) { // want `function "spawnInTypeAssertion" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	if _, ok := v.(int); ok {
		g.Go(task)
	}
	_ = g.Wait()
}

// [BAD]: Very deep nesting - should still be detected
func deepNesting(task func() error) { // want `function "deepNesting" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	if true {
		if true {
			if true {
				for i := 0; i < 1; i++ {
					g.Go(task)
				}
			}
		}
	}
	_ = g.Wait()
}

// [LIMITATION]: Unreachable spawn after panic - SSA eliminates unreachable code
//
// SSA optimizes away code after panic()
func limitationPanicBeforeSpawn(task func() error) {
	g := new(errgroup.Group)
	panic("oops")
	g.Go(task)
	_ = g.Wait()
}

// [GOOD]: Closed over spawn call in returned func
func closedOverSpawn(task func() error) func() {
	return func() {
		g := new(errgroup.Group)
		g.Go(task)
		_ = g.Wait()
	}
}

// [BAD]: Named return with forwarding spawn
func namedReturnWithSpawn(task func() error) (err error) { // want `function "namedReturnWithSpawn" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	g.Go(task)
	err = g.Wait()
	return
}

// [BAD]: IIFE containing forwarding spawn call
//
// IIFE that spawns a goroutine - SSA traces into IIFE.
func badIIFEContainingSpawn(task func() error) { // want `function "badIIFEContainingSpawn" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	func() {
		g.Go(func() error {
			return task()
		})
	}()
	_ = g.Wait()
}

// [BAD]: Nested IIFE containing forwarding spawn call
//
// Nested IIFE that spawns a goroutine.
func badNestedIIFEContainingSpawn(task func() error) { // want `function "badNestedIIFEContainingSpawn" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	func() {
		func() {
			g.Go(task)
		}()
	}()
	_ = g.Wait()
}

// ===== CLOSURES MADE BUT NOT RUN (#137) =====

// [GOOD]: Forwarding closure, only made
//
// The closure is never run here, so the function spawns nothing.
func goodSpawningClosureOnlyMade(task func() error) {
	fn := func() {
		g := new(errgroup.Group)
		g.Go(task)
		_ = g.Wait()
	}
	_ = fn
}

// [GOOD]: Factory whose spawning result is never run
//
// Calling the factory only makes the closure.
func goodFactoryResultNotRun() {
	w := makeSpawningWorker(1)
	_ = w
}

// [GOOD]: Factory whose spawning result is run, spawning a literal (#377)
//
// The closure spawns its own literal, not a func param of this function.
func goodFactoryResultRun() {
	makeSpawningWorker(1)()
}

// [LIMITATION]: Factory given the func param, whose result is run
//
// The closure spawns what the factory was given, but its free variable is
// bound to the factory's own param, which is not followed to the call.
func limitationFactoryGivenParamRun(task func() error) {
	makeForwardingWorker(task)()
}

// [BAD]: Forwarding closure run in a goroutine
//
// The goroutine runs a closure that captures the param.
func badSpawnInGoroutine(task func() error) { // want `function "badSpawnInGoroutine" should have //goroutinectx:spawner directive \(runs func argument in go statement\)`
	go func() {
		g := new(errgroup.Group)
		g.Go(task)
		_ = g.Wait()
	}()
}

// [GOOD]: Spawning closure of its own literal run in a goroutine (#377)
func goodSpawnInGoroutineLiteral(n int) {
	go func() {
		g := new(errgroup.Group)
		g.Go(func() error { _ = n; return nil })
		_ = g.Wait()
	}()
}

// makeSpawningWorker returns a closure that spawns. It makes the closure but
// does not run it, so it needs no label.
func makeSpawningWorker(n int) func() {
	return func() {
		g := new(errgroup.Group)
		g.Go(func() error { _ = n; return nil })
		_ = g.Wait()
	}
}

// makeForwardingWorker returns a closure that spawns task. It makes the
// closure but does not run it, so it needs no label.
func makeForwardingWorker(task func() error) func() {
	return func() {
		g := new(errgroup.Group)
		g.Go(task)
		_ = g.Wait()
	}
}

// [GOOD]: Generic factory whose spawning result is run (#303, #377)
//
// The closure the generic factory returns is followed through the
// instantiation to its origin. It spawns its own literal.
func goodGenericFactoryResultRun() {
	makeSpawningWorkerG[int](1)()
}

// [GOOD]: Generic factory whose spawning result is held, then run (#377)
func goodGenericFactoryResultHeldRun() {
	w := makeSpawningWorkerG[string]("x")
	w()
}

// [GOOD]: Generic factory whose spawning result is never run
//
// Calling the factory only makes the closure.
func goodGenericFactoryResultNotRun() {
	w := makeSpawningWorkerG[int](1)
	_ = w
}

// makeSpawningWorkerG is makeSpawningWorker, generic.
func makeSpawningWorkerG[T any](v T) func() {
	return func() {
		g := new(errgroup.Group)
		g.Go(func() error { _ = v; return nil })
		_ = g.Wait()
	}
}

// ===== CLOSURES RUN THROUGH A PHI (#306) =====

// phiFlag picks a branch the analyzer cannot predict.
var phiFlag bool

// [BAD]: Forwarding closure called through a Phi
//
// Each value the Phi joins is the closure, so the call runs the spawn, as
// calling it directly does.
func badSpawningClosurePhiCall(task func() error) { // want `function "badSpawningClosurePhiCall" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	f := func() {
		g := new(errgroup.Group)
		g.Go(task)
		_ = g.Wait()
	}
	h := f
	if phiFlag {
		h = f
	}
	h()
}

// [BAD]: Forwarding closure deferred through a Phi
//
// The deferred call runs the spawn.
func badSpawningClosurePhiDefer(task func() error) { // want `function "badSpawningClosurePhiDefer" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	f := func() {
		g := new(errgroup.Group)
		g.Go(task)
		_ = g.Wait()
	}
	h := f
	if phiFlag {
		h = f
	}
	defer h()
}

// [BAD]: Forwarding closure run in a goroutine through a Phi
//
// The goroutine runs a closure that captures the param.
func badSpawningClosurePhiGo(task func() error) { // want `function "badSpawningClosurePhiGo" should have //goroutinectx:spawner directive \(runs func argument in go statement\)`
	f := func() {
		g := new(errgroup.Group)
		g.Go(task)
		_ = g.Wait()
	}
	h := f
	if phiFlag {
		h = f
	}
	go h()
}

// [BAD]: One of two closures joined by a Phi forwards
//
// The call may run the closure that spawns the param.
func badOneSpawningClosurePhiCall(n int, task func() error) { // want `function "badOneSpawningClosurePhiCall" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	h := func() { fmt.Println(n) }
	if phiFlag {
		h = func() {
			g := new(errgroup.Group)
			g.Go(task)
			_ = g.Wait()
		}
	}
	h()
}

// [GOOD]: Closures joined by a Phi that spawn only their own literals (#377)
func goodLiteralSpawningClosurePhiCall(n int) {
	h := func() { fmt.Println(n) }
	if phiFlag {
		h = func() {
			g := new(errgroup.Group)
			g.Go(func() error { _ = n; return nil })
			_ = g.Wait()
		}
	}
	h()
}

// [GOOD]: Closures joined by a Phi that do not spawn
//
// Neither closure the call may run spawns.
func goodPlainClosurePhiCall(n int) {
	h := func() { fmt.Println(n) }
	if phiFlag {
		h = func() { fmt.Println(n + 1) }
	}
	h()
}

// [GOOD]: Forwarding closure through a Phi, only made
//
// The closure is never run here, so the function spawns nothing.
func goodSpawningClosurePhiOnlyMade(task func() error) {
	f := func() {
		g := new(errgroup.Group)
		g.Go(task)
		_ = g.Wait()
	}
	h := f
	if phiFlag {
		h = f
	}
	_ = h
}
