// Package conc contains test fixtures for the conc context propagation checker.
// This file covers advanced patterns - real-world complex patterns: nested functions,
// conditionals, loops. See basic.go for daily patterns and evil.go for adversarial tests.
package conc

import (
	"context"
	"fmt"

	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/pool"
)

// ===== NESTED FUNCTIONS =====

// [BAD]: Go call inside inner named func without ctx
//
// Go call in nested function does not use the outer context.
//
// See also:
//   errgroup: badNestedInnerFunc
//   waitgroup: badNestedInnerFunc
func badNestedInnerFunc(ctx context.Context) {
	wg := conc.WaitGroup{}
	innerFunc := func() {
		wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
		})
	}
	innerFunc()
	wg.Wait()
}

// [BAD]: Go call inside IIFE without ctx
//
// Go call inside immediately invoked function expression without context.
//
// See also:
//   errgroup: badNestedClosure
//   waitgroup: badNestedClosure
func badNestedClosure(ctx context.Context) {
	wg := conc.WaitGroup{}
	func() {
		wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
		})
	}()
	wg.Wait()
}

// [BAD]: Go call inside deeply nested IIFE without ctx
//
// Go call inside immediately invoked function expression without context.
//
// See also:
//   errgroup: badNestedDeep
//   waitgroup: badNestedDeep
func badNestedDeep(ctx context.Context) {
	wg := conc.WaitGroup{}
	func() {
		func() {
			wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
			})
		}()
	}()
	wg.Wait()
}

// [GOOD]: Go call inside inner func with ctx
//
// Nested function properly captures and uses the outer context.
//
// See also:
//   errgroup: goodNestedWithCtx
//   waitgroup: goodNestedWithCtx
func goodNestedWithCtx(ctx context.Context) {
	wg := conc.WaitGroup{}
	innerFunc := func() {
		wg.Go(func() {
			_ = ctx
		})
	}
	innerFunc()
	wg.Wait()
}

// [GOOD]: Inner func has own ctx param
//
// Inner function declares its own context parameter and uses it.
//
// See also:
//   goroutine: goodShadowingInnerCtxParam
//   errgroup: goodNestedInnerHasOwnCtx
//   waitgroup: goodNestedInnerHasOwnCtx
func goodNestedInnerHasOwnCtx(outerCtx context.Context) {
	innerFunc := func(ctx context.Context) {
		wg := conc.WaitGroup{}
		wg.Go(func() {
			_ = ctx // uses inner ctx
		})
		wg.Wait()
	}
	innerFunc(outerCtx)
}

// ===== CONDITIONAL PATTERNS =====

// [BAD]: Conditional Go without ctx
//
// Conditional branches spawn goroutines without using context.
//
// See also:
//   goroutine: badConditionalGoroutine
//   errgroup: badConditionalGo
//   waitgroup: badConditionalGo
func badConditionalGo(ctx context.Context, flag bool) {
	wg := conc.WaitGroup{}
	if flag {
		wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
		})
	} else {
		wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
		})
	}
	wg.Wait()
}

// [GOOD]: Conditional goroutine with ctx
//
// All conditional branches properly use context in goroutines.
func goodConditionalGo(ctx context.Context, flag bool) {
	wg := conc.WaitGroup{}
	if flag {
		wg.Go(func() {
			_ = ctx
		})
	} else {
		wg.Go(func() {
			_ = ctx
		})
	}
	wg.Wait()
}

// ===== LOOP PATTERNS =====

// [BAD]: Go in for loop without ctx
//
// Goroutines spawned in loop iterations do not use context.
//
// See also:
//   goroutine: badGoroutinesInLoop
//   errgroup: badLoopGo
//   waitgroup: badLoopGo
func badLoopGo(ctx context.Context) {
	wg := conc.WaitGroup{}
	for i := 0; i < 3; i++ {
		wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
		})
	}
	wg.Wait()
}

// [GOOD]: Goroutine in for loop with ctx
//
// Goroutines in loop properly capture and use context.
func goodLoopGo(ctx context.Context) {
	wg := conc.WaitGroup{}
	for i := 0; i < 3; i++ {
		wg.Go(func() {
			_ = ctx
		})
	}
	wg.Wait()
}

// [BAD]: Go in range loop without ctx
//
// Goroutines spawned in loop iterations do not use context.
//
// See also:
//   goroutine: badGoroutinesInRangeLoop
//   errgroup: badRangeLoopGo
//   waitgroup: badRangeLoopGo
func badRangeLoopGo(ctx context.Context) {
	wg := conc.WaitGroup{}
	items := []int{1, 2, 3}
	for _, item := range items {
		wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
			fmt.Println(item)
		})
	}
	wg.Wait()
}

// ===== DEFER PATTERNS =====

// [BAD]: Closure with defer but no ctx
//
// Closure with defer statement does not use context.
//
// See also:
//   errgroup: badDeferInClosure
//   waitgroup: badDeferInClosure
func badDeferInClosure(ctx context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
		defer fmt.Println("deferred")
	})
	wg.Wait()
}

// [GOOD]: Literal with ctx in select - with defer
//
// Closure with ctx and defer
func goodDeferWithCtxDirect(ctx context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() {
		_ = ctx // use ctx directly
		defer fmt.Println("cleanup")
	})
	wg.Wait()
}

// [LIMITATION]: Ctx in deferred nested closure not detected
//
// Context used only in deferred nested closure is not detected.
//
// See also:
//   goroutine: limitationDeferNestedClosure
//   errgroup: limitationDeferNestedClosure
//   waitgroup: limitationDeferNestedClosure
func limitationDeferNestedClosure(ctx context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
		// ctx is only in nested closure - not detected
		defer func() { _ = ctx }()
	})
	wg.Wait()
}

// ===== POOL SPECIFIC PATTERNS =====

// [BAD]: pool.Pool in for loop without ctx
//
// pool.Pool goroutines in loop without context.
func badPoolLoopGo(ctx context.Context) {
	p := pool.New()
	for i := 0; i < 3; i++ {
		p.Go(func() { // want `pool.Pool.Go\(\) closure should use context "ctx"`
		})
	}
	p.Wait()
}

// [GOOD]: pool.Pool in for loop with ctx
//
// pool.Pool goroutines in loop with context.
func goodPoolLoopGo(ctx context.Context) {
	p := pool.New()
	for i := 0; i < 3; i++ {
		p.Go(func() {
			_ = ctx
		})
	}
	p.Wait()
}
