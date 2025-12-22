//go:build go1.25

// Package waitgroup contains test fixtures for the waitgroup context propagation checker.
// This file covers advanced patterns - real-world complex patterns: nested functions,
// conditionals, loops. See basic.go for daily patterns and evil.go for adversarial tests.
package waitgroup

import (
	"context"
	"fmt"
	"sync"
)

// ===== NESTED FUNCTIONS =====

// [BAD]: Go call inside inner named func without ctx
//
// Go call in nested function does not use the outer context.
//
// See also:
//   errgroup: badNestedInnerFunc
func badNestedInnerFunc(ctx context.Context) {
	var wg sync.WaitGroup
	innerFunc := func() {
		wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
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
func badNestedClosure(ctx context.Context) {
	var wg sync.WaitGroup
	func() {
		wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
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
func badNestedDeep(ctx context.Context) {
	var wg sync.WaitGroup
	func() {
		func() {
			wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
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
func goodNestedWithCtx(ctx context.Context) {
	var wg sync.WaitGroup
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
//   errgroup: goodNestedInnerHasOwnCtx
//   goroutine: goodShadowingInnerCtxParam
func goodNestedInnerHasOwnCtx(outerCtx context.Context) {
	innerFunc := func(ctx context.Context) {
		var wg sync.WaitGroup
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
//   errgroup: badConditionalGo
//   goroutine: badConditionalGoroutine
func badConditionalGo(ctx context.Context, flag bool) {
	var wg sync.WaitGroup
	if flag {
		wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		})
	} else {
		wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		})
	}
	wg.Wait()
}

// [GOOD]: Conditional goroutine with ctx
//
// All conditional branches properly use context in goroutines.
func goodConditionalGo(ctx context.Context, flag bool) {
	var wg sync.WaitGroup
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
//   errgroup: badLoopGo
//   goroutine: badGoroutinesInLoop
func badLoopGo(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		})
	}
	wg.Wait()
}

// [GOOD]: Goroutine in for loop with ctx
//
// Goroutines in loop properly capture and use context.
func goodLoopGo(ctx context.Context) {
	var wg sync.WaitGroup
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
//   errgroup: badRangeLoopGo
//   goroutine: badGoroutinesInRangeLoop
func badRangeLoopGo(ctx context.Context) {
	var wg sync.WaitGroup
	items := []int{1, 2, 3}
	for _, item := range items {
		wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
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
func badDeferInClosure(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		defer fmt.Println("deferred")
	})
	wg.Wait()
}

// [GOOD]: Literal with derived ctx - with defer
//
// Closure with ctx and defer
func goodDeferWithCtxDirect(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = ctx // use ctx directly
		defer fmt.Println("cleanup")
	})
	wg.Wait()
}

// [GOOD]: Ctx in deferred nested closure - SSA correctly detects
//
// SSA FreeVars propagation correctly detects context captured in nested closures.
//
// See also:
//   errgroup: goodDeferNestedClosure
//   goroutine: goodDeferNestedClosure
func goodDeferNestedClosure(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // SSA correctly detects ctx capture
		// ctx in nested closure - SSA captures this
		defer func() { _ = ctx }()
	})
	wg.Wait()
}
