// Package goroutine contains test fixtures for the goroutine context propagation checker.
// This file covers adversarial patterns - unusual code that tests analyzer limits:
// 2+ level nesting, go fn()(), IIFE, interface methods, LIMITATION cases.
// See basic.go for daily patterns and advanced.go for real-world complex patterns.
package goroutine

import (
	"context"
	"fmt"
)

// ===== 2-LEVEL GOROUTINE NESTING =====

// [BAD]: Nested goroutine - outer uses ctx, inner doesn't
//
// Outer goroutine uses context but inner does not.
func badNestedInner(ctx context.Context) {
	go func() {
		_ = ctx // outer goroutine uses ctx, but inner doesn't
		go func() { // want `goroutine does not propagate context "ctx"`
			fmt.Println("inner")
		}()
	}()
}

// [BAD]: 3-level nesting
//
// 3-level nesting - only first uses ctx
func badNestedDeep(ctx context.Context) {
	go func() {
		_ = ctx
		go func() { // want `goroutine does not propagate context "ctx"`
			go func() { // want `goroutine does not propagate context "ctx"`
				fmt.Println("deep")
			}()
		}()
	}()
}

// [GOOD]: Nested goroutines
//
// Nested goroutines - all use ctx
func goodNestedAllUseCtx(ctx context.Context) {
	go func() {
		_ = ctx
		go func() {
			_ = ctx
			go func() {
				_ = ctx
			}()
		}()
	}()
}

// [BAD]: 4-level nesting
//
// 4-level nesting - last level missing ctx
func badDeeplyNestedGoroutines(ctx context.Context) {
	go func() {
		_ = ctx
		go func() {
			_ = ctx
			go func() {
				_ = ctx
				go func() { // want `goroutine does not propagate context "ctx"`
					fmt.Println("level 4")
				}()
			}()
		}()
	}()
}

// ===== go fn()() HIGHER-ORDER PATTERNS =====

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

// [BAD]: go fn()() - double higher-order
//
// go fn()() - higher-order without ctx
func badGoHigherOrder(ctx context.Context) {
	go makeWorker()() // want `goroutine does not propagate context "ctx"`
}

// [GOOD]: go fn()() - double higher-order
//
// go fn(ctx)() - higher-order with ctx
func goodGoHigherOrder(ctx context.Context) {
	go makeWorkerWithCtx(ctx)()
}

// [BAD]: go fn()()() - triple higher-order
//
// Three levels of function invocation without context propagation.
func badGoHigherOrderTriple(ctx context.Context) {
	makeMaker := func() func() func() {
		return func() func() {
			return func() {
				fmt.Println("triple")
			}
		}
	}
	go makeMaker()()() // want `goroutine does not propagate context "ctx"`
}

// [GOOD]: go fn()()() - triple higher-order
//
// Three levels of function invocation with context propagation.
func goodGoHigherOrderTriple(ctx context.Context) {
	makeMaker := func(c context.Context) func() func() {
		return func() func() {
			return func() {
				_ = c
			}
		}
	}
	go makeMaker(ctx)()()
}

// [BAD]: Arbitrary depth go fn()()...()
//
// Arbitrary depth go fn()()()...() without ctx
func badGoInfiniteChain(ctx context.Context) {
	f := func() func() func() func() {
		return func() func() func() {
			return func() func() {
				return func() {
					fmt.Println("deep chain")
				}
			}
		}
	}
	go f()()()() // want `goroutine does not propagate context "ctx"`
}

// [GOOD]: Arbitrary depth go fn()()...()
//
// Arbitrary depth go fn(ctx)()()...() with ctx
func goodGoInfiniteChain(ctx context.Context) {
	f := func(c context.Context) func() func() func() {
		return func() func() func() {
			return func() func() {
				return func() {
					_ = c
				}
			}
		}
	}
	go f(ctx)()()()
}

// ===== IIFE PATTERNS =====

// [BAD]: IIFE inside goroutine without ctx
//
// IIFE inside goroutine body does not propagate context outward.
func badIIFEInsideGoroutine(ctx context.Context) {
	go func() { // want `goroutine does not propagate context "ctx"`
		func() {
			fmt.Println("iife")
		}()
	}()
}

// [BAD]: Ctx only in IIFE nested closure (LIMITATION)
//
// Known analyzer limitation: this pattern cannot be detected statically.
func badIIFEUsesCtxInNestedFunc(ctx context.Context) {
	// ctx used only in nested IIFE, not by goroutine's direct body
	go func() { // want `goroutine does not propagate context "ctx"`
		func() {
			_ = ctx.Done()
		}()
	}()
}

// [GOOD]: IIFE with ctx in goroutine body
//
// IIFE in goroutine body properly uses context.
func goodIIFEWithCtxInGoroutineBody(ctx context.Context) {
	go func() {
		_ = ctx // ctx used directly
		func() {
			fmt.Println("iife")
		}()
	}()
}

// ===== INTERFACE METHOD PATTERNS =====

type Runner interface {
	Run()
}

type myRunner struct{}

//vt:helper
func (r *myRunner) Run() {
	fmt.Println("running")
}

// [BAD]: Interface method
//
// Interface method call without context argument.
func badGoroutineCallsInterfaceMethod(ctx context.Context, r Runner) {
	go func() { // want `goroutine does not propagate context "ctx"`
		r.Run()
	}()
}

type CtxRunner interface {
	RunWithCtx(ctx context.Context)
}

type myCtxRunner struct{}

//vt:helper
func (r *myCtxRunner) RunWithCtx(ctx context.Context) {
	_ = ctx
}

// [GOOD]: Interface method
//
// Context is passed as argument to interface method.
func goodGoroutineCallsInterfaceMethodWithCtx(ctx context.Context, r CtxRunner) {
	go func() {
		r.RunWithCtx(ctx)
	}()
}

// ===== TYPE ASSERTION IN GOROUTINE =====

// [BAD]: Type assertion without ctx
//
// Function retrieved via type assertion without context.
func badGoroutineWithTypeAssertion(ctx context.Context) {
	var x interface{} = "hello"
	go func() { // want `goroutine does not propagate context "ctx"`
		if s, ok := x.(string); ok {
			fmt.Println(s)
		}
	}()
}

// ===== GOROUTINE IN EXPRESSION =====

// [BAD]: Goroutine in expression (not immediately invoked)
//
// Goroutine spawned in expression context without invocation.
func badGoroutineInExpression(ctx context.Context) {
	_ = func() {
		go func() { // want `goroutine does not propagate context "ctx"`
			fmt.Println("in expression")
		}()
	}
}

// ===== MULTIPLE GOROUTINES PARALLEL =====

// [BAD]: Multiple parallel goroutines
//
// Multiple parallel goroutines - mixed ctx usage
func badMultipleGoroutinesParallel(ctx context.Context) {
	go func() { // want `goroutine does not propagate context "ctx"`
		fmt.Println("first")
	}()

	go func() { // want `goroutine does not propagate context "ctx"`
		fmt.Println("second")
	}()

	go func() {
		_ = ctx
	}()
}

// ===== NESTED CLOSURE GOROUTINE PATTERNS =====
// These test goroutines inside nested closures - analyzer CAN trace ctx through FreeVar chains.

// [GOOD]: Goroutine in nested closure
//
// Goroutine in nested closure properly captures context.
func goodNestedClosureWithCtx(ctx context.Context) {
	wrapper := func() {
		go func() {
			_ = ctx // ctx IS used via FreeVar chain - analyzer detects it
		}()
	}
	wrapper()
}

// [BAD]: Goroutine in nested closure
//
// Goroutine in nested closure does not use context.
func badNestedClosureWithoutCtx(ctx context.Context) {
	wrapper := func() {
		go func() { // want `goroutine does not propagate context "ctx"`
			fmt.Println("no ctx") // ctx NOT used
		}()
	}
	wrapper()
}

// [GOOD]: Goroutine in deferred function
//
// Closure with defer statement properly uses context.
func goodDeferredGoroutineWithCtx(ctx context.Context) {
	defer func() {
		go func() {
			_ = ctx // ctx IS used - analyzer detects it
		}()
	}()
}

// [BAD]: Goroutine in deferred function
//
// Closure with defer statement does not use context.
func badDeferredGoroutineWithoutCtx(ctx context.Context) {
	defer func() {
		go func() { // want `goroutine does not propagate context "ctx"`
			fmt.Println("no ctx") // ctx NOT used
		}()
	}()
}

// [GOOD]: Goroutine in init expression
//
// Goroutine spawned in variable initialization uses context.
func goodGoroutineInInitWithCtx(ctx context.Context) {
	_ = func() func() {
		go func() {
			_ = ctx // ctx IS used - analyzer detects it
		}()
		return nil
	}()
}

// [BAD]: Goroutine in init expression
//
// Goroutine spawned in variable initialization ignores context.
func badGoroutineInInitWithoutCtx(ctx context.Context) {
	_ = func() func() {
		go func() { // want `goroutine does not propagate context "ctx"`
			fmt.Println("no ctx") // ctx NOT used
		}()
		return nil
	}()
}

// ===== MULTIPLE CONTEXT EVIL PATTERNS =====

// [GOOD]: Three contexts - uses middle one
//
// Using the middle of multiple context parameters is valid.
//
// See also:
//   errgroup: goodUsesMiddleOfThreeContexts
//   waitgroup: goodUsesMiddleOfThreeContexts
func goodUsesMiddleOfThreeContexts(ctx1, ctx2, ctx3 context.Context) {
	go func() {
		_ = ctx2 // uses middle context
	}()
}

// [GOOD]: Three contexts - uses last one
//
// Using the last of multiple context parameters is valid.
//
// See also:
//   errgroup: goodUsesLastOfThreeContexts
//   waitgroup: goodUsesLastOfThreeContexts
func goodUsesLastOfThreeContexts(ctx1, ctx2, ctx3 context.Context) {
	go func() {
		_ = ctx3 // uses last context
	}()
}

// [GOOD]: Multiple ctx in separate param groups
//
// Context in separate parameter group is detected and used.
//
// See also:
//   errgroup: goodMultipleCtxSeparateGroups
//   waitgroup: goodMultipleCtxSeparateGroups
func goodMultipleCtxSeparateGroups(a int, ctx1 context.Context, b string, ctx2 context.Context) {
	go func() {
		_ = ctx2 // uses second ctx from different param group
	}()
}

// [BAD]: Multiple ctx in separate param groups - none used
//
// Context in separate parameter group is not used.
//
// See also:
//   errgroup: badMultipleCtxSeparateGroups
//   waitgroup: badMultipleCtxSeparateGroups
func badMultipleCtxSeparateGroups(a int, ctx1 context.Context, b string, ctx2 context.Context) {
	go func() { // want `goroutine does not propagate context "ctx1"`
		fmt.Println(a, b) // uses other params but not ctx
	}()
}

// [GOOD]: Both contexts used
//
// When multiple contexts exist, using any one satisfies the check.
//
// See also:
//   errgroup: goodUsesBothContexts
//   waitgroup: goodUsesBothContexts
func goodUsesBothContexts(ctx1, ctx2 context.Context) {
	go func() {
		_ = ctx1
		_ = ctx2
	}()
}

// [GOOD]: Nested goroutine - outer uses ctx1, inner uses ctx2
//
// Nested goroutines using different context parameters.
func goodNestedDifferentContexts(ctx1, ctx2 context.Context) {
	go func() {
		_ = ctx1 // outer uses ctx1
		go func() {
			_ = ctx2 // inner uses ctx2 - still valid!
		}()
	}()
}

// [BAD]: Nested goroutine - outer uses ctx2, inner uses neither
//
// Inner goroutine ignores all available contexts.
func badNestedOnlyOuterUsesCtx(ctx1, ctx2 context.Context) {
	go func() {
		_ = ctx2 // outer uses ctx2
		go func() { // want `goroutine does not propagate context "ctx1"`
			fmt.Println("inner uses neither")
		}()
	}()
}

// [GOOD]: Higher-order with multiple ctx - factory receives ctx1
//
// Factory function receives first context parameter.
//
// See also:
//   errgroup: goodHigherOrderMultipleCtx
//   waitgroup: goodHigherOrderMultipleCtx
func goodHigherOrderMultipleCtx(ctx1, ctx2 context.Context) {
	go makeWorkerWithCtx(ctx1)() // factory uses ctx1
}

// [GOOD]: Higher-order with multiple ctx - factory receives ctx2
//
// Factory function receives second context parameter.
//
// See also:
//   errgroup: goodHigherOrderMultipleCtxSecond
//   waitgroup: goodHigherOrderMultipleCtxSecond
func goodHigherOrderMultipleCtxSecond(ctx1, ctx2 context.Context) {
	go makeWorkerWithCtx(ctx2)() // factory uses ctx2
}

// ===== IIFE AND ARGUMENT-PASSING PATTERNS =====
// These test goroutines inside IIFEs and context passed via arguments

// [BAD]: Goroutine inside IIFE
//
// Goroutine inside IIFE ignores outer context.
func badGoroutineInIIFE(ctx context.Context) {
	func() {
		go func() { // want `goroutine does not propagate context "ctx"`
			fmt.Println("goroutine in IIFE")
		}()
	}()
}

// [GOOD]: Goroutine inside IIFE
//
// Goroutine inside IIFE captures context from outer scope.
func goodGoroutineInIIFE(ctx context.Context) {
	func() {
		go func() {
			_ = ctx.Done()
		}()
	}()
}

// [GOOD]: Context passed via argument - inner shadows outer
//
// Inner function parameter shadows outer context correctly.
func goodArgumentShadowing(outerCtx context.Context) {
	func(ctx context.Context) {
		go func() {
			_ = ctx // uses inner ctx (shadowing)
		}()
	}(outerCtx)
}

// [BAD]: Context passed via argument - inner ignores it
//
// Inner function receives context but goroutine ignores it.
func badArgumentShadowing(outerCtx context.Context) {
	func(ctx context.Context) {
		go func() { // want `goroutine does not propagate context "ctx"`
			fmt.Println("ignores inner ctx")
		}()
	}(outerCtx)
}

// [GOOD]: Two levels of argument passing
//
// Context passed through two levels of function calls.
func goodTwoLevelArguments(ctx1 context.Context) {
	func(ctx2 context.Context) {
		func(ctx3 context.Context) {
			go func() {
				_ = ctx3 // uses innermost ctx
			}()
		}(ctx2)
	}(ctx1)
}

// [BAD]: Two levels
//
// Two levels - innermost doesn't use
func badTwoLevelArguments(ctx1 context.Context) {
	func(ctx2 context.Context) {
		func(ctx3 context.Context) {
			go func() { // want `goroutine does not propagate context "ctx3"`
				fmt.Println("ignores ctx3")
			}()
		}(ctx2)
	}(ctx1)
}

// [GOOD]: Middle layer introduces ctx - goroutine uses it
//
// Middle layer introduces context that inner goroutine uses.
//
// See also:
//   errgroup: evilMiddleLayerIntroducesCtx
//   waitgroup: evilMiddleLayerIntroducesCtx
func goodMiddleLayerIntroducesCtxUsed() {
	func(ctx context.Context) {
		go func() {
			_ = ctx
		}()
	}(context.Background())
}

// [BAD]: Middle layer introduces ctx - goroutine ignores
//
// Middle layer has context but inner goroutine ignores it.
func badMiddleLayerIntroducesCtxIgnored() {
	func(ctx context.Context) {
		go func() { // want `goroutine does not propagate context "ctx"`
			fmt.Println("ignores middle ctx")
		}()
	}(context.Background())
}

// [GOOD]: Interleaved layers - ctx->no ctx->ctx shadowing
//
// Interleaved layers - ctx -> no ctx -> ctx (shadowing) -> goroutine uses it
func goodInterleavedLayersUsed(outerCtx context.Context) {
	func() {
		func(middleCtx context.Context) {
			go func() {
				_ = middleCtx // uses shadowing ctx
			}()
		}(outerCtx)
	}()
}

// [BAD]: Interleaved layers - goroutine ignores shadowing ctx
//
// Nested function layers where goroutine ignores available context.
func badInterleavedLayersIgnored(outerCtx context.Context) {
	func() {
		func(middleCtx context.Context) {
			go func() { // want `goroutine does not propagate context "middleCtx"`
				fmt.Println("ignores middleCtx")
			}()
		}(outerCtx)
	}()
}

// ===== HIGHER-ORDER WITH VARIABLE RETURN =====
// These patterns test ReturnedValueUsesContext with Ident (variable) returns.

// [GOOD]: Higher-order returns variable - with ctx
//
// Factory function returns a variable that captures context.
//
// See also:
//   errgroup: goodHigherOrderReturnsVariableWithCtx
//   waitgroup: goodHigherOrderReturnsVariableWithCtx
func goodHigherOrderReturnsVariableWithCtx(ctx context.Context) {
	makeWorker := func() func() {
		worker := func() {
			_ = ctx // worker uses ctx
		}
		return worker // Returns variable, not literal
	}
	go makeWorker()()
}

// [BAD]: Higher-order returns variable - without ctx
//
// Factory function returns a variable that does not capture context.
//
// See also:
//   errgroup: badHigherOrderReturnsVariableWithoutCtx
//   waitgroup: badHigherOrderReturnsVariableWithoutCtx
func badHigherOrderReturnsVariableWithoutCtx(ctx context.Context) {
	makeWorker := func() func() {
		worker := func() {
			fmt.Println("no ctx")
		}
		return worker // Returns variable, not literal
	}
	go makeWorker()() // want `goroutine does not propagate context "ctx"`
}

// [GOOD]: Higher-order returns reassigned variable - with ctx
//
// Factory function returns a reassigned variable that captures context.
//
// See also:
//   errgroup: goodHigherOrderReturnsReassignedVariableWithCtx
//   waitgroup: goodHigherOrderReturnsReassignedVariableWithCtx
func goodHigherOrderReturnsReassignedVariableWithCtx(ctx context.Context) {
	makeWorker := func() func() {
		worker := func() {
			fmt.Println("first assignment")
		}
		worker = func() {
			_ = ctx // Last assignment uses ctx
		}
		return worker
	}
	go makeWorker()()
}

// [GOOD]: Triple higher-order returns variable - with ctx
//
// Triple higher-order with variable returns at each level.
func goodTripleHigherOrderVariableWithCtx(ctx context.Context) {
	makeMaker := func() func() func() {
		maker := func() func() {
			worker := func() {
				_ = ctx
			}
			return worker
		}
		return maker
	}
	go makeMaker()()()
}
