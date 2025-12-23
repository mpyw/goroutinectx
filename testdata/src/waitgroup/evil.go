//go:build go1.25

// Package waitgroup contains test fixtures for the waitgroup context propagation checker.
// This file covers adversarial patterns - tests analyzer limits: higher-order functions,
// non-literal function arguments, interface methods.
// See basic.go for daily patterns and advanced.go for real-world complex patterns.
package waitgroup

import (
	"context"
	"fmt"
	"sync"
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
func badVariableFunc(ctx context.Context) {
	var wg sync.WaitGroup
	fn := func() {
		fmt.Println("no ctx")
	}
	wg.Go(fn) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [GOOD]: Variable func
//
// Function stored in variable captures and uses context.
//
// See also:
//   errgroup: goodVariableFuncWithCtx
func goodVariableFuncWithCtx(ctx context.Context) {
	var wg sync.WaitGroup
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
func badHigherOrderFunc(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(makeWorker()) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [GOOD]: Higher-order func
//
// Factory function is called with context, enabling propagation.
//
// See also:
//   errgroup: goodHigherOrderFuncWithCtx
func goodHigherOrderFuncWithCtx(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(makeWorkerWithCtx(ctx)) // OK - makeWorkerWithCtx captures ctx
	wg.Wait()
}

// ===== STRUCT FIELD / SLICE / MAP TRACKING =====
// These patterns CAN be tracked when defined in the same function.

type taskHolderWithCtx struct {
	task func()
}

// [GOOD]: Struct field with ctx
//
// Function stored in struct field captures context.
//
// See also:
//   errgroup: goodStructFieldWithCtx
func goodStructFieldWithCtx(ctx context.Context) {
	var wg sync.WaitGroup
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
func goodSliceIndexWithCtx(ctx context.Context) {
	var wg sync.WaitGroup
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
func goodMapKeyWithCtx(ctx context.Context) {
	var wg sync.WaitGroup
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
func goodInterfaceMethodWithCtxArg(ctx context.Context, factory WorkerFactory) {
	var wg sync.WaitGroup
	// ctx IS passed as argument to CreateWorker - analyzer detects ctx usage
	wg.Go(factory.CreateWorker(ctx)) // OK - ctx passed as argument
	wg.Wait()
}

type WorkerFactoryNoCtx interface {
	CreateWorker() func()
}

// [NOTCHECKED]: Interface method with argument
//
// Interface method call without context argument - not traced.
//
// See also:
//   errgroup: badInterfaceMethodWithoutCtxArgNotTraced
func badInterfaceMethodWithoutCtxArgNotTraced(ctx context.Context, factory WorkerFactoryNoCtx) {
	var wg sync.WaitGroup
	// ctx NOT passed to CreateWorker - should fail but can't trace
	wg.Go(factory.CreateWorker()) // No error - can't trace interface methods
	wg.Wait()
}

// ===== TRACING LIMITATIONS =====
// These patterns cannot be tracked statically.
// Due to "zero false positives" policy, these are NOT reported.

//goroutinectx:spawner //vt:helper
func runWithWaitGroup(wg *sync.WaitGroup, fn func()) {
	wg.Go(fn)
}

// [GOOD]: Function with ctx passed through spawner
//
// Function with ctx passed through spawner - should pass
//
// See also:
//   errgroup: goodFuncPassedThroughSpawner
func goodFuncPassedThroughSpawner(ctx context.Context) {
	var wg sync.WaitGroup
	fn := func() {
		_ = ctx // fn uses ctx
	}
	runWithWaitGroup(&wg, fn) // OK - fn uses ctx, and runWithWaitGroup is marked as spawner
	wg.Wait()
}

// [BAD]: Function without ctx passed through spawner
//
// Function without ctx passed through spawner - should report
//
// See also:
//   errgroup: badFuncPassedThroughSpawner
func badFuncPassedThroughSpawner(ctx context.Context) {
	var wg sync.WaitGroup
	fn := func() {
		fmt.Println("no ctx")
	}
	runWithWaitGroup(&wg, fn) // want `runWithWaitGroup\(\) func argument should use context "ctx"`
	wg.Wait()
}

// [LIMITATION]: Function from channel - ctx captured but not traced
//
// Function received from channel cannot be traced statically.
//
// See also:
//   errgroup: goodFuncFromChannelNotTraced
func goodFuncFromChannelNotTraced(ctx context.Context) {
	var wg sync.WaitGroup
	ch := make(chan func(), 1)
	ch <- func() {
		_ = ctx // The func DOES capture ctx
	}
	fn := <-ch
	// Can't trace through channel receive, assume OK
	wg.Go(fn) // No error - zero false positives policy
	wg.Wait()
}

type taskHolder struct {
	task func()
}

// [BAD]: Function from struct field without ctx
//
// Function stored in struct field does not capture context.
//
// See also:
//   errgroup: badStructFieldWithoutCtx
func badStructFieldWithoutCtx(ctx context.Context) {
	var wg sync.WaitGroup
	holder := taskHolder{
		task: func() {
			fmt.Println("no ctx")
		},
	}
	wg.Go(holder.task) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Function from map without ctx
//
// Function from map without ctx - NOW TRACKED
//
// See also:
//   errgroup: badMapValueWithoutCtx
func badMapValueWithoutCtx(ctx context.Context) {
	var wg sync.WaitGroup
	tasks := map[string]func(){
		"task1": func() {},
	}
	wg.Go(tasks["task1"]) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Function from slice without ctx
//
// Function from slice without ctx - NOW TRACKED
//
// See also:
//   errgroup: badSliceValueWithoutCtx
func badSliceValueWithoutCtx(ctx context.Context) {
	var wg sync.WaitGroup
	tasks := []func(){
		func() {},
	}
	wg.Go(tasks[0]) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [LIMITATION]: Function through interface{} - ctx captured but not traced
//
// Context captured but lost through interface type assertion.
//
// See also:
//   errgroup: goodFuncThroughInterfaceNotTraced
func goodFuncThroughInterfaceNotTraced(ctx context.Context) {
	var wg sync.WaitGroup

	var i interface{} = func() {
		_ = ctx // fn DOES capture ctx
	}

	// Type assert to get func back
	fn := i.(func())
	// Can't trace through interface{} assertion, assume OK
	wg.Go(fn) // No error - zero false positives policy
	wg.Wait()
}

// [NOTCHECKED]: Function through interface without ctx
//
// Function through interface{} type assertion without context - not traced.
//
// See also:
//   errgroup: badFuncThroughInterfaceWithoutCtxNotTraced
func badFuncThroughInterfaceWithoutCtxNotTraced(ctx context.Context) {
	var wg sync.WaitGroup

	var i interface{} = func() {
		fmt.Println("no ctx") // fn does NOT use ctx
	}

	fn := i.(func())
	// Can't trace through interface{} assertion
	wg.Go(fn) // No error - can't trace interface assertion
	wg.Wait()
}

// ===== MULTIPLE CONTEXT EVIL PATTERNS =====

// [GOOD]: Three contexts - uses middle one
//
// Using the middle of multiple context parameters is valid.
//
// See also:
//   errgroup: goodUsesMiddleOfThreeContexts
//   goroutine: goodUsesMiddleOfThreeContexts
func goodUsesMiddleOfThreeContexts(ctx1, ctx2, ctx3 context.Context) {
	var wg sync.WaitGroup
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
//   errgroup: goodUsesLastOfThreeContexts
//   goroutine: goodUsesLastOfThreeContexts
func goodUsesLastOfThreeContexts(ctx1, ctx2, ctx3 context.Context) {
	var wg sync.WaitGroup
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
//   errgroup: goodMultipleCtxSeparateGroups
//   goroutine: goodMultipleCtxSeparateGroups
func goodMultipleCtxSeparateGroups(a int, ctx1 context.Context, b string, ctx2 context.Context) {
	var wg sync.WaitGroup
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
//   errgroup: badMultipleCtxSeparateGroups
//   goroutine: badMultipleCtxSeparateGroups
func badMultipleCtxSeparateGroups(a int, ctx1 context.Context, b string, ctx2 context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx1"`
		fmt.Println(a, b) // uses other params but not ctx
	})
	wg.Wait()
}

// [GOOD]: Both contexts used
//
// When multiple contexts exist, using any one satisfies the check.
//
// See also:
//   errgroup: goodUsesBothContexts
//   goroutine: goodUsesBothContexts
func goodUsesBothContexts(ctx1, ctx2 context.Context) {
	var wg sync.WaitGroup
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
//   errgroup: goodHigherOrderMultipleCtx
//   goroutine: goodHigherOrderMultipleCtx
func goodHigherOrderMultipleCtx(ctx1, ctx2 context.Context) {
	var wg sync.WaitGroup
	wg.Go(makeWorkerWithCtx(ctx1)) // factory uses ctx1
	wg.Wait()
}

// [GOOD]: Higher-order with multiple ctx - factory receives ctx2
//
// Factory function receives second context parameter.
//
// See also:
//   errgroup: goodHigherOrderMultipleCtxSecond
//   goroutine: goodHigherOrderMultipleCtxSecond
func goodHigherOrderMultipleCtxSecond(ctx1, ctx2 context.Context) {
	var wg sync.WaitGroup
	wg.Go(makeWorkerWithCtx(ctx2)) // factory uses ctx2
	wg.Wait()
}

// ===== ADVANCED NESTED PATTERNS (SHADOWING, ARGUMENT PASSING) =====

// [BAD]: Shadowing - inner ctx shadows outer
//
// Inner function's context parameter shadows the outer one.
//
// See also:
//   errgroup: evilShadowingInnerHasCtx
func evilShadowingInnerHasCtx(outerCtx context.Context) {
	innerFunc := func(ctx context.Context) {
		var wg sync.WaitGroup
		wg.Go(func() {
			_ = ctx // uses inner ctx
		})
		wg.Wait()
	}
	innerFunc(outerCtx)
}

// [BAD]: Shadowing - inner ignores ctx
//
// Inner function ignores the shadowed context.
//
// See also:
//   errgroup: evilShadowingInnerIgnoresCtx
func evilShadowingInnerIgnoresCtx(outerCtx context.Context) {
	innerFunc := func(ctx context.Context) {
		var wg sync.WaitGroup
		wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		})
		wg.Wait()
	}
	innerFunc(outerCtx)
}

// [BAD]: Two levels of shadowing
//
// Context is shadowed through two levels of function nesting.
//
// See also:
//   errgroup: evilShadowingTwoLevels
func evilShadowingTwoLevels(ctx1 context.Context) {
	func(ctx2 context.Context) {
		func(ctx3 context.Context) {
			var wg sync.WaitGroup
			wg.Go(func() {
				_ = ctx3 // uses ctx3
			})
			wg.Wait()
		}(ctx2)
	}(ctx1)
}

// [BAD]: Two levels of shadowing
//
// Context is shadowed through two levels of function nesting.
//
// See also:
//   errgroup: evilShadowingTwoLevelsBad
func evilShadowingTwoLevelsBad(ctx1 context.Context) {
	func(ctx2 context.Context) {
		func(ctx3 context.Context) {
			var wg sync.WaitGroup
			wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx3"`
			})
			wg.Wait()
		}(ctx2)
	}(ctx1)
}

// ===== MIDDLE LAYER INTRODUCES CTX (OUTER HAS NONE) =====

// [GOOD]: Middle layer introduces ctx - goroutine uses it
//
// Middle layer introduces context that inner goroutine uses.
//
// See also:
//   errgroup: evilMiddleLayerIntroducesCtx
//   goroutine: goodMiddleLayerIntroducesCtxUsed
func evilMiddleLayerIntroducesCtx() {
	func(ctx context.Context) {
		var wg sync.WaitGroup
		wg.Go(func() {
			_ = ctx
		})
		func() {
			wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
			})
		}()
		wg.Wait()
	}(context.Background())
}

// [GOOD]: Middle layer introduces ctx
//
// Middle layer introduces context that inner goroutine uses.
//
// See also:
//   errgroup: evilMiddleLayerIntroducesCtxGood
func evilMiddleLayerIntroducesCtxGood() {
	func(ctx context.Context) {
		var wg sync.WaitGroup
		func() {
			wg.Go(func() {
				_ = ctx
			})
		}()
		wg.Wait()
	}(context.Background())
}

// ===== INTERLEAVED LAYERS (ctx -> no ctx -> ctx shadowing) =====

// [BAD]: Interleaved layers
//
// Nested function layers where goroutine ignores available context.
//
// See also:
//   errgroup: evilInterleavedLayers
func evilInterleavedLayers(outerCtx context.Context) {
	func() {
		func(middleCtx context.Context) {
			var wg sync.WaitGroup
			func() {
				wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "middleCtx"`
				})
			}()
			wg.Wait()
		}(outerCtx)
	}()
}

// [GOOD]: Interleaved layers
//
// Nested function layers with context shadowing handled correctly.
//
// See also:
//   errgroup: evilInterleavedLayersGood
func evilInterleavedLayersGood(outerCtx context.Context) {
	func() {
		func(middleCtx context.Context) {
			var wg sync.WaitGroup
			func() {
				wg.Go(func() {
					_ = middleCtx
				})
			}()
			wg.Wait()
		}(outerCtx)
	}()
}

// ===== HIGHER-ORDER WITH VARIABLE RETURN =====
// These patterns test FuncLitReturnUsesContext/ReturnedValueUsesContext with Ident (variable) returns.

// [GOOD]: Higher-order returns variable - with ctx
//
// Factory function returns a variable that captures context.
//
// See also:
//   errgroup: goodHigherOrderReturnsVariableWithCtx
//   goroutine: goodHigherOrderReturnsVariableWithCtx
func goodHigherOrderReturnsVariableWithCtx(ctx context.Context) {
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
//
// See also:
//   errgroup: badHigherOrderReturnsVariableWithoutCtx
//   goroutine: badHigherOrderReturnsVariableWithoutCtx
func badHigherOrderReturnsVariableWithoutCtx(ctx context.Context) {
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
//
// See also:
//   errgroup: goodHigherOrderReturnsReassignedVariableWithCtx
//   goroutine: goodHigherOrderReturnsReassignedVariableWithCtx
func goodHigherOrderReturnsReassignedVariableWithCtx(ctx context.Context) {
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
