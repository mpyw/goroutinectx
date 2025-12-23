// Package errgroup contains test fixtures for the errgroup context propagation checker.
// This file covers adversarial patterns - tests analyzer limits: higher-order functions,
// non-literal function arguments, interface methods.
// See basic.go for daily patterns and advanced.go for real-world complex patterns.
package errgroup

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

// ===== HIGHER-ORDER FUNCTION PATTERNS =====

//vt:helper
func makeWorker() func() error {
	return func() error {
		fmt.Println("worker")
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

// [BAD]: Variable func
//
// Function stored in variable does not capture context.
//
// See also:
//   waitgroup: badVariableFunc
func badVariableFunc(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error {
		fmt.Println("no ctx")
		return nil
	}
	g.Go(fn) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [GOOD]: Variable func
//
// Function stored in variable captures and uses context.
//
// See also:
//   waitgroup: goodVariableFuncWithCtx
func goodVariableFuncWithCtx(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error {
		_ = ctx
		return nil
	}
	g.Go(fn) // OK - fn uses ctx
	_ = g.Wait()
}

// [BAD]: Higher-order func
//
// Function returned by factory does not use context.
//
// See also:
//   waitgroup: badHigherOrderFunc
func badHigherOrderFunc(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(makeWorker()) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [GOOD]: Higher-order func
//
// Factory function is called with context, enabling propagation.
//
// See also:
//   waitgroup: goodHigherOrderFuncWithCtx
func goodHigherOrderFuncWithCtx(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(makeWorkerWithCtx(ctx)) // OK - makeWorkerWithCtx captures ctx
	_ = g.Wait()
}

// ===== STRUCT FIELD / SLICE / MAP TRACKING =====
// These patterns CAN be tracked when defined in the same function.

type taskHolderWithCtx struct {
	task func() error
}

// [GOOD]: Struct field with ctx
//
// Function stored in struct field captures context.
//
// See also:
//   waitgroup: goodStructFieldWithCtx
func goodStructFieldWithCtx(ctx context.Context) {
	g := new(errgroup.Group)
	holder := taskHolderWithCtx{
		task: func() error {
			_ = ctx // Uses ctx
			return nil
		},
	}
	g.Go(holder.task) // OK - now tracked
	_ = g.Wait()
}

// [GOOD]: Slice index with ctx
//
// Function in slice element captures context.
//
// See also:
//   waitgroup: goodSliceIndexWithCtx
func goodSliceIndexWithCtx(ctx context.Context) {
	g := new(errgroup.Group)
	tasks := []func() error{
		func() error {
			_ = ctx // Uses ctx
			return nil
		},
	}
	g.Go(tasks[0]) // OK - now tracked
	_ = g.Wait()
}

// [GOOD]: Map key with ctx
//
// Function in map value captures context.
//
// See also:
//   waitgroup: goodMapKeyWithCtx
func goodMapKeyWithCtx(ctx context.Context) {
	g := new(errgroup.Group)
	tasks := map[string]func() error{
		"key": func() error {
			_ = ctx // Uses ctx
			return nil
		},
	}
	g.Go(tasks["key"]) // OK - now tracked
	_ = g.Wait()
}

// ===== INTERFACE METHOD PATTERNS =====
// ctx passed as argument to interface method IS detected by the analyzer.

// When ctx is passed as argument, analyzer detects ctx usage.
type WorkerFactory interface {
	CreateWorker(ctx context.Context) func() error
}

type myFactory struct{}

//vt:helper
func (f *myFactory) CreateWorker(ctx context.Context) func() error {
	return func() error {
		_ = ctx // Implementation captures ctx
		return nil
	}
}

// [GOOD]: Interface method with argument
//
// Context is passed as argument to interface method.
//
// See also:
//   waitgroup: goodInterfaceMethodWithCtxArg
func goodInterfaceMethodWithCtxArg(ctx context.Context, factory WorkerFactory) {
	g := new(errgroup.Group)
	// ctx IS passed as argument to CreateWorker - analyzer detects ctx usage
	g.Go(factory.CreateWorker(ctx)) // OK - ctx passed as argument
	_ = g.Wait()
}

type WorkerFactoryNoCtx interface {
	CreateWorker() func() error
}

// [NOTCHECKED]: Interface method with argument
//
// Interface method call without context argument - not traced.
//
// See also:
//   waitgroup: badInterfaceMethodWithoutCtxArgNotTraced
func badInterfaceMethodWithoutCtxArgNotTraced(ctx context.Context, factory WorkerFactoryNoCtx) {
	g := new(errgroup.Group)
	// ctx NOT passed to CreateWorker - should fail but can't trace
	g.Go(factory.CreateWorker()) // No error - can't trace interface methods
	_ = g.Wait()
}

// ===== TRACING LIMITATIONS =====
// These patterns cannot be tracked statically.
// Due to "zero false positives" policy, these are NOT reported.

//goroutinectx:spawner //vt:helper
func runWithGroup(g *errgroup.Group, fn func() error) {
	g.Go(fn)
}

// [GOOD]: Function with ctx passed through spawner
//
// Function with ctx passed through spawner - should pass
//
// See also:
//   waitgroup: goodFuncPassedThroughSpawner
func goodFuncPassedThroughSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error {
		_ = ctx // fn uses ctx
		return nil
	}
	runWithGroup(g, fn) // OK - fn uses ctx, and runWithGroup is marked as spawner
	_ = g.Wait()
}

// [BAD]: Function without ctx passed through spawner
//
// Function without ctx passed through spawner - should report
//
// See also:
//   waitgroup: badFuncPassedThroughSpawner
func badFuncPassedThroughSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error {
		fmt.Println("no ctx")
		return nil
	}
	runWithGroup(g, fn) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// [LIMITATION]: Function from channel - ctx captured but not traced
//
// Function received from channel cannot be traced statically.
//
// See also:
//   waitgroup: goodFuncFromChannelNotTraced
func goodFuncFromChannelNotTraced(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan func() error, 1)
	ch <- func() error {
		_ = ctx // The func DOES capture ctx
		return nil
	}
	fn := <-ch
	// Can't trace through channel receive, assume OK
	g.Go(fn) // No error - zero false positives policy
	_ = g.Wait()
}

type taskHolder struct {
	task func() error
}

// [BAD]: Function from struct field without ctx
//
// Function stored in struct field does not capture context.
//
// See also:
//   waitgroup: badStructFieldWithoutCtx
func badStructFieldWithoutCtx(ctx context.Context) {
	g := new(errgroup.Group)
	holder := taskHolder{
		task: func() error {
			fmt.Println("no ctx")
			return nil
		},
	}
	g.Go(holder.task) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Function from map without ctx
//
// Function from map without ctx - NOW TRACKED
//
// See also:
//   waitgroup: badMapValueWithoutCtx
func badMapValueWithoutCtx(ctx context.Context) {
	g := new(errgroup.Group)
	tasks := map[string]func() error{
		"task1": func() error { return nil },
	}
	g.Go(tasks["task1"]) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Function from slice without ctx
//
// Function from slice without ctx - NOW TRACKED
//
// See also:
//   waitgroup: badSliceValueWithoutCtx
func badSliceValueWithoutCtx(ctx context.Context) {
	g := new(errgroup.Group)
	tasks := []func() error{
		func() error { return nil },
	}
	g.Go(tasks[0]) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [LIMITATION]: Function through interface{} - ctx captured but not traced
//
// Context captured but lost through interface type assertion.
//
// See also:
//   waitgroup: goodFuncThroughInterfaceNotTraced
func goodFuncThroughInterfaceNotTraced(ctx context.Context) {
	g := new(errgroup.Group)

	var i interface{} = func() error {
		_ = ctx // fn DOES capture ctx
		return nil
	}

	// Type assert to get func back
	fn := i.(func() error)
	// Can't trace through interface{} assertion, assume OK
	g.Go(fn) // No error - zero false positives policy
	_ = g.Wait()
}

// [NOTCHECKED]: Function through interface without ctx
//
// Function through interface{} type assertion without context - not traced.
//
// See also:
//   waitgroup: badFuncThroughInterfaceWithoutCtxNotTraced
func badFuncThroughInterfaceWithoutCtxNotTraced(ctx context.Context) {
	g := new(errgroup.Group)

	var i interface{} = func() error {
		fmt.Println("no ctx") // fn does NOT use ctx
		return nil
	}

	fn := i.(func() error)
	// Can't trace through interface{} assertion
	g.Go(fn) // No error - can't trace interface assertion
	_ = g.Wait()
}

// ===== MULTIPLE CONTEXT EVIL PATTERNS =====

// [GOOD]: Three contexts - uses middle one
//
// Using the middle of multiple context parameters is valid.
//
// See also:
//   goroutine: goodUsesMiddleOfThreeContexts
//   waitgroup: goodUsesMiddleOfThreeContexts
func goodUsesMiddleOfThreeContexts(ctx1, ctx2, ctx3 context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx2 // uses middle context
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Three contexts - uses last one
//
// Using the last of multiple context parameters is valid.
//
// See also:
//   goroutine: goodUsesLastOfThreeContexts
//   waitgroup: goodUsesLastOfThreeContexts
func goodUsesLastOfThreeContexts(ctx1, ctx2, ctx3 context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx3 // uses last context
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Multiple ctx in separate param groups
//
// Context in separate parameter group is detected and used.
//
// See also:
//   goroutine: goodMultipleCtxSeparateGroups
//   waitgroup: goodMultipleCtxSeparateGroups
func goodMultipleCtxSeparateGroups(a int, ctx1 context.Context, b string, ctx2 context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx2 // uses second ctx from different param group
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Multiple ctx in separate param groups - none used
//
// Context in separate parameter group is not used.
//
// See also:
//   goroutine: badMultipleCtxSeparateGroups
//   waitgroup: badMultipleCtxSeparateGroups
func badMultipleCtxSeparateGroups(a int, ctx1 context.Context, b string, ctx2 context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx1"`
		fmt.Println(a, b) // uses other params but not ctx
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Both contexts used
//
// When multiple contexts exist, using any one satisfies the check.
//
// See also:
//   goroutine: goodUsesBothContexts
//   waitgroup: goodUsesBothContexts
func goodUsesBothContexts(ctx1, ctx2 context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx1
		_ = ctx2
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Higher-order with multiple ctx - factory receives ctx1
//
// Factory function receives first context parameter.
//
// See also:
//   goroutine: goodHigherOrderMultipleCtx
//   waitgroup: goodHigherOrderMultipleCtx
func goodHigherOrderMultipleCtx(ctx1, ctx2 context.Context) {
	g := new(errgroup.Group)
	g.Go(makeWorkerWithCtx(ctx1)) // factory uses ctx1
	_ = g.Wait()
}

// [GOOD]: Higher-order with multiple ctx - factory receives ctx2
//
// Factory function receives second context parameter.
//
// See also:
//   goroutine: goodHigherOrderMultipleCtxSecond
//   waitgroup: goodHigherOrderMultipleCtxSecond
func goodHigherOrderMultipleCtxSecond(ctx1, ctx2 context.Context) {
	g := new(errgroup.Group)
	g.Go(makeWorkerWithCtx(ctx2)) // factory uses ctx2
	_ = g.Wait()
}

// ===== ADVANCED NESTED PATTERNS (SHADOWING, ARGUMENT PASSING) =====

// [BAD]: Shadowing - inner ctx shadows outer
//
// Inner function's context parameter shadows the outer one.
//
// See also:
//   waitgroup: evilShadowingInnerHasCtx
func evilShadowingInnerHasCtx(outerCtx context.Context) {
	innerFunc := func(ctx context.Context) {
		g := new(errgroup.Group)
		g.Go(func() error {
			_ = ctx // uses inner ctx
			return nil
		})
		_ = g.Wait()
	}
	innerFunc(outerCtx)
}

// [BAD]: Shadowing - inner ignores ctx
//
// Inner function ignores the shadowed context.
//
// See also:
//   waitgroup: evilShadowingInnerIgnoresCtx
func evilShadowingInnerIgnoresCtx(outerCtx context.Context) {
	innerFunc := func(ctx context.Context) {
		g := new(errgroup.Group)
		g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
			return nil
		})
		_ = g.Wait()
	}
	innerFunc(outerCtx)
}

// [BAD]: Two levels of shadowing
//
// Context is shadowed through two levels of function nesting.
//
// See also:
//   waitgroup: evilShadowingTwoLevels
func evilShadowingTwoLevels(ctx1 context.Context) {
	func(ctx2 context.Context) {
		func(ctx3 context.Context) {
			g := new(errgroup.Group)
			g.Go(func() error {
				_ = ctx3 // uses ctx3
				return nil
			})
			_ = g.Wait()
		}(ctx2)
	}(ctx1)
}

// [BAD]: Two levels of shadowing
//
// Context is shadowed through two levels of function nesting.
//
// See also:
//   waitgroup: evilShadowingTwoLevelsBad
func evilShadowingTwoLevelsBad(ctx1 context.Context) {
	func(ctx2 context.Context) {
		func(ctx3 context.Context) {
			g := new(errgroup.Group)
			g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx3"`
				return nil
			})
			_ = g.Wait()
		}(ctx2)
	}(ctx1)
}

// ===== MIDDLE LAYER INTRODUCES CTX (OUTER HAS NONE) =====

// [GOOD]: Middle layer introduces ctx - goroutine uses it
//
// Middle layer introduces context that inner goroutine uses.
//
// See also:
//   goroutine: goodMiddleLayerIntroducesCtxUsed
//   waitgroup: evilMiddleLayerIntroducesCtx
func evilMiddleLayerIntroducesCtx() {
	func(ctx context.Context) {
		g := new(errgroup.Group)
		g.Go(func() error {
			_ = ctx
			return nil
		})
		func() {
			g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
				return nil
			})
		}()
		_ = g.Wait()
	}(context.Background())
}

// [GOOD]: Middle layer introduces ctx
//
// Middle layer introduces context that inner goroutine uses.
//
// See also:
//   waitgroup: evilMiddleLayerIntroducesCtxGood
func evilMiddleLayerIntroducesCtxGood() {
	func(ctx context.Context) {
		g := new(errgroup.Group)
		func() {
			g.Go(func() error {
				_ = ctx
				return nil
			})
		}()
		_ = g.Wait()
	}(context.Background())
}

// ===== INTERLEAVED LAYERS (ctx -> no ctx -> ctx shadowing) =====

// [BAD]: Interleaved layers
//
// Nested function layers where goroutine ignores available context.
//
// See also:
//   waitgroup: evilInterleavedLayers
func evilInterleavedLayers(outerCtx context.Context) {
	func() {
		func(middleCtx context.Context) {
			g := new(errgroup.Group)
			func() {
				g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "middleCtx"`
					return nil
				})
			}()
			_ = g.Wait()
		}(outerCtx)
	}()
}

// [GOOD]: Interleaved layers
//
// Nested function layers with context shadowing handled correctly.
//
// See also:
//   waitgroup: evilInterleavedLayersGood
func evilInterleavedLayersGood(outerCtx context.Context) {
	func() {
		func(middleCtx context.Context) {
			g := new(errgroup.Group)
			func() {
				g.Go(func() error {
					_ = middleCtx
					return nil
				})
			}()
			_ = g.Wait()
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
//   goroutine: goodHigherOrderReturnsVariableWithCtx
//   waitgroup: goodHigherOrderReturnsVariableWithCtx
func goodHigherOrderReturnsVariableWithCtx(ctx context.Context) {
	g := new(errgroup.Group)
	makeWorker := func() func() error {
		worker := func() error {
			_ = ctx // worker uses ctx
			return nil
		}
		return worker // Returns variable, not literal
	}
	g.Go(makeWorker())
	_ = g.Wait()
}

// [BAD]: Higher-order returns variable - without ctx
//
// Factory function returns a variable that does not capture context.
//
// See also:
//   goroutine: badHigherOrderReturnsVariableWithoutCtx
//   waitgroup: badHigherOrderReturnsVariableWithoutCtx
func badHigherOrderReturnsVariableWithoutCtx(ctx context.Context) {
	g := new(errgroup.Group)
	makeWorker := func() func() error {
		worker := func() error {
			fmt.Println("no ctx")
			return nil
		}
		return worker // Returns variable, not literal
	}
	g.Go(makeWorker()) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [GOOD]: Higher-order returns reassigned variable - with ctx
//
// Factory function returns a reassigned variable that captures context.
//
// See also:
//   goroutine: goodHigherOrderReturnsReassignedVariableWithCtx
//   waitgroup: goodHigherOrderReturnsReassignedVariableWithCtx
func goodHigherOrderReturnsReassignedVariableWithCtx(ctx context.Context) {
	g := new(errgroup.Group)
	makeWorker := func() func() error {
		worker := func() error {
			fmt.Println("first assignment")
			return nil
		}
		worker = func() error {
			_ = ctx // Last assignment uses ctx
			return nil
		}
		return worker
	}
	g.Go(makeWorker())
	_ = g.Wait()
}
