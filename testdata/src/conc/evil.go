// Package conc contains test fixtures for the conc context propagation checker.
// This file covers adversarial patterns - tests analyzer limits: higher-order functions,
// non-literal function arguments, interface methods.
// See basic.go for daily patterns and advanced.go for real-world complex patterns.
package conc

import (
	"context"
	"fmt"

	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/pool"
)

// ===== HIGHER-ORDER FUNCTION PATTERNS =====

//vt:helper
func makeWorker() func() {
	return func() {
		fmt.Println("worker")
	}
}

//vt:helper
func makeWorkerWithCtx(ctx context.Context) func() {
	return func() {
		_ = ctx
	}
}

// [BAD]: Variable func
//
// Function stored in variable does not capture context.
//
// See also:
//   errgroup: badVariableFunc
//   waitgroup: badVariableFunc
func badVariableFunc(ctx context.Context) {
	wg := conc.WaitGroup{}
	fn := func() {
		fmt.Println("no ctx")
	}
	wg.Go(fn) // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [GOOD]: Variable func
//
// Function stored in variable captures and uses context.
//
// See also:
//   errgroup: goodVariableFuncWithCtx
//   waitgroup: goodVariableFuncWithCtx
func goodVariableFuncWithCtx(ctx context.Context) {
	wg := conc.WaitGroup{}
	fn := func() {
		_ = ctx
	}
	wg.Go(fn) // OK - fn uses ctx
	wg.Wait()
}

// [BAD]: Higher-order func
//
// Function returned by factory does not use context.
//
// See also:
//   errgroup: badHigherOrderFunc
//   waitgroup: badHigherOrderFunc
func badHigherOrderFunc(ctx context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(makeWorker()) // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [GOOD]: Higher-order func
//
// Factory function is called with context, enabling propagation.
//
// See also:
//   errgroup: goodHigherOrderFuncWithCtx
//   waitgroup: goodHigherOrderFuncWithCtx
func goodHigherOrderFuncWithCtx(ctx context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(makeWorkerWithCtx(ctx)) // OK - makeWorkerWithCtx captures ctx
	wg.Wait()
}

// ===== STRUCT FIELD / SLICE / MAP TRACKING =====
// These patterns CAN be tracked when defined in the same function.

type taskHolder struct {
	task func()
}

type taskHolderWithCtx struct {
	task func()
}

// [GOOD]: Struct field with ctx
//
// Function stored in struct field captures context.
//
// See also:
//   errgroup: goodStructFieldWithCtx
//   waitgroup: goodStructFieldWithCtx
func goodStructFieldWithCtx(ctx context.Context) {
	wg := conc.WaitGroup{}
	holder := taskHolderWithCtx{
		task: func() {
			_ = ctx // Uses ctx
		},
	}
	wg.Go(holder.task) // OK - now tracked
	wg.Wait()
}

// [GOOD]: Slice index with ctx
//
// Function in slice element captures context.
//
// See also:
//   errgroup: goodSliceIndexWithCtx
//   waitgroup: goodSliceIndexWithCtx
func goodSliceIndexWithCtx(ctx context.Context) {
	wg := conc.WaitGroup{}
	tasks := []func(){
		func() {
			_ = ctx // Uses ctx
		},
	}
	wg.Go(tasks[0]) // OK - now tracked
	wg.Wait()
}

// [GOOD]: Map key with ctx
//
// Function in map value captures context.
//
// See also:
//   errgroup: goodMapKeyWithCtx
//   waitgroup: goodMapKeyWithCtx
func goodMapKeyWithCtx(ctx context.Context) {
	wg := conc.WaitGroup{}
	tasks := map[string]func(){
		"key": func() {
			_ = ctx // Uses ctx
		},
	}
	wg.Go(tasks["key"]) // OK - now tracked
	wg.Wait()
}

// ===== INTERFACE METHOD PATTERNS =====
// ctx passed as argument to interface method IS detected by the analyzer.

// When ctx is passed as argument, analyzer detects ctx usage.
type WorkerFactory interface {
	CreateWorker(ctx context.Context) func()
}

type myFactory struct{}

//vt:helper
func (f *myFactory) CreateWorker(ctx context.Context) func() {
	return func() {
		_ = ctx // Implementation captures ctx
	}
}

// [GOOD]: Interface method with argument
//
// Context is passed as argument to interface method.
//
// See also:
//   errgroup: goodInterfaceMethodWithCtxArg
//   waitgroup: goodInterfaceMethodWithCtxArg
func goodInterfaceMethodWithCtxArg(ctx context.Context, factory WorkerFactory) {
	wg := conc.WaitGroup{}
	// ctx IS passed as argument to CreateWorker - analyzer detects ctx usage
	wg.Go(factory.CreateWorker(ctx)) // OK - ctx passed as argument
	wg.Wait()
}

type WorkerFactoryNoCtx interface {
	CreateWorker() func()
}

// [BAD]: Interface method with argument
//
// Interface method call without context argument.
//
// See also:
//   errgroup: badInterfaceMethodWithoutCtxArg
//   waitgroup: badInterfaceMethodWithoutCtxArg
func badInterfaceMethodWithoutCtxArg(ctx context.Context, factory WorkerFactoryNoCtx) {
	wg := conc.WaitGroup{}
	// ctx NOT passed to CreateWorker - expected to fail
	wg.Go(factory.CreateWorker()) // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// ===== REMAINING LIMITATIONS =====
// These patterns cannot be tracked statically.

// [LIMITATION]: Function from channel - ctx captured but not traced
//
// Function received from channel cannot be traced statically.
//
// See also:
//   errgroup: limitationFuncFromChannel
//   waitgroup: limitationFuncFromChannel
func limitationFuncFromChannel(ctx context.Context) {
	wg := conc.WaitGroup{}
	ch := make(chan func(), 1)
	ch <- func() {
		_ = ctx // The func DOES capture ctx
	}
	fn := <-ch
	// LIMITATION: fn captures ctx, but analyzer can't trace through channel receive
	wg.Go(fn) // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Function from struct field without ctx
//
// Function stored in struct field does not capture context.
//
// See also:
//   errgroup: badStructFieldWithoutCtx
//   waitgroup: badStructFieldWithoutCtx
func badStructFieldWithoutCtx(ctx context.Context) {
	wg := conc.WaitGroup{}
	holder := taskHolder{
		task: func() {
			fmt.Println("no ctx")
		},
	}
	wg.Go(holder.task) // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Function from map without ctx
//
// Function from map without ctx - NOW TRACKED
//
// See also:
//   errgroup: badMapValueWithoutCtx
//   waitgroup: badMapValueWithoutCtx
func badMapValueWithoutCtx(ctx context.Context) {
	wg := conc.WaitGroup{}
	tasks := map[string]func(){
		"task1": func() {},
	}
	wg.Go(tasks["task1"]) // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Function from slice without ctx
//
// Function from slice without ctx - NOW TRACKED
//
// See also:
//   errgroup: badSliceValueWithoutCtx
//   waitgroup: badSliceValueWithoutCtx
func badSliceValueWithoutCtx(ctx context.Context) {
	wg := conc.WaitGroup{}
	tasks := []func(){
		func() {},
	}
	wg.Go(tasks[0]) // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [LIMITATION]: Function through interface{} - ctx captured but not traced
//
// Context captured but lost through interface type assertion.
//
// See also:
//   errgroup: limitationFuncThroughInterfaceWithCtx
//   waitgroup: limitationFuncThroughInterfaceWithCtx
func limitationFuncThroughInterfaceWithCtx(ctx context.Context) {
	wg := conc.WaitGroup{}

	var i interface{} = func() {
		_ = ctx // fn DOES capture ctx
	}

	// Type assert to get func back
	fn := i.(func())
	// LIMITATION: fn captures ctx, but analyzer can't trace through interface{} assertion
	wg.Go(fn) // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Function through interface without ctx
//
// Interface method call without context argument.
//
// See also:
//   errgroup: badFuncThroughInterfaceWithoutCtx
//   waitgroup: badFuncThroughInterfaceWithoutCtx
func badFuncThroughInterfaceWithoutCtx(ctx context.Context) {
	wg := conc.WaitGroup{}

	var i interface{} = func() {
		fmt.Println("no ctx") // fn does NOT use ctx
	}

	fn := i.(func())
	wg.Go(fn) // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// ===== MULTIPLE CONTEXT EVIL PATTERNS =====

// [GOOD]: Three contexts - uses middle one
//
// Using the middle of multiple context parameters is valid.
//
// See also:
//   goroutine: goodUsesMiddleOfThreeContexts
//   errgroup: goodUsesMiddleOfThreeContexts
//   waitgroup: goodUsesMiddleOfThreeContexts
func goodUsesMiddleOfThreeContexts(ctx1, ctx2, ctx3 context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() {
		_ = ctx2 // uses middle context
	})
	wg.Wait()
}

// [GOOD]: Three contexts - uses last one
//
// Using the last of multiple context parameters is valid.
//
// See also:
//   goroutine: goodUsesLastOfThreeContexts
//   errgroup: goodUsesLastOfThreeContexts
//   waitgroup: goodUsesLastOfThreeContexts
func goodUsesLastOfThreeContexts(ctx1, ctx2, ctx3 context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() {
		_ = ctx3 // uses last context
	})
	wg.Wait()
}

// [GOOD]: Multiple ctx in separate param groups
//
// Context in separate parameter group is detected and used.
//
// See also:
//   goroutine: goodMultipleCtxSeparateGroups
//   errgroup: goodMultipleCtxSeparateGroups
//   waitgroup: goodMultipleCtxSeparateGroups
func goodMultipleCtxSeparateGroups(a int, ctx1 context.Context, b string, ctx2 context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() {
		_ = ctx2 // uses second ctx from different param group
	})
	wg.Wait()
}

// [BAD]: Multiple ctx in separate param groups - none used
//
// Context in separate parameter group is not used.
//
// See also:
//   goroutine: badMultipleCtxSeparateGroups
//   errgroup: badMultipleCtxSeparateGroups
//   waitgroup: badMultipleCtxSeparateGroups
func badMultipleCtxSeparateGroups(a int, ctx1 context.Context, b string, ctx2 context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx1"`
		fmt.Println(a, b) // uses other params but not ctx
	})
	wg.Wait()
}

// [GOOD]: Both contexts used
//
// When multiple contexts exist, using any one satisfies the check.
//
// See also:
//   goroutine: goodUsesBothContexts
//   errgroup: goodUsesBothContexts
//   waitgroup: goodUsesBothContexts
func goodUsesBothContexts(ctx1, ctx2 context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() {
		_ = ctx1
		_ = ctx2
	})
	wg.Wait()
}

// [GOOD]: Higher-order with multiple ctx - factory receives ctx1
//
// Factory function receives first context parameter.
//
// See also:
//   goroutine: goodHigherOrderMultipleCtx
//   errgroup: goodHigherOrderMultipleCtx
//   waitgroup: goodHigherOrderMultipleCtx
func goodHigherOrderMultipleCtx(ctx1, ctx2 context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(makeWorkerWithCtx(ctx1)) // factory uses ctx1
	wg.Wait()
}

// [GOOD]: Higher-order with multiple ctx - factory receives ctx2
//
// Factory function receives second context parameter.
//
// See also:
//   goroutine: goodHigherOrderMultipleCtxSecond
//   errgroup: goodHigherOrderMultipleCtxSecond
//   waitgroup: goodHigherOrderMultipleCtxSecond
func goodHigherOrderMultipleCtxSecond(ctx1, ctx2 context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(makeWorkerWithCtx(ctx2)) // factory uses ctx2
	wg.Wait()
}

// ===== HIGHER-ORDER WITH VARIABLE RETURN =====

// [GOOD]: Higher-order returns variable - with ctx
//
// Factory function returns a variable that captures context.
//
// See also:
//   goroutine: goodHigherOrderReturnsVariableWithCtx
//   errgroup: goodHigherOrderReturnsVariableWithCtx
//   waitgroup: goodHigherOrderReturnsVariableWithCtx
func goodHigherOrderReturnsVariableWithCtx(ctx context.Context) {
	wg := conc.WaitGroup{}
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
//
// See also:
//   goroutine: badHigherOrderReturnsVariableWithoutCtx
//   errgroup: badHigherOrderReturnsVariableWithoutCtx
//   waitgroup: badHigherOrderReturnsVariableWithoutCtx
func badHigherOrderReturnsVariableWithoutCtx(ctx context.Context) {
	wg := conc.WaitGroup{}
	makeWorker := func() func() {
		worker := func() {
			fmt.Println("no ctx")
		}
		return worker // Returns variable, not literal
	}
	wg.Go(makeWorker()) // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// ===== POOL-SPECIFIC EVIL PATTERNS =====

// [BAD]: pool.Pool variable func without ctx
//
// pool.Pool with variable function without context.
func badPoolVariableFunc(ctx context.Context) {
	p := pool.New()
	fn := func() {
		fmt.Println("no ctx")
	}
	p.Go(fn) // want `pool.Pool.Go\(\) closure should use context "ctx"`
	p.Wait()
}

// [GOOD]: pool.Pool variable func with ctx
//
// pool.Pool with variable function that captures context.
func goodPoolVariableFuncWithCtx(ctx context.Context) {
	p := pool.New()
	fn := func() {
		_ = ctx
	}
	p.Go(fn) // OK - fn uses ctx
	p.Wait()
}
