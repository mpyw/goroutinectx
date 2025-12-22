// Package conc contains test fixtures for the conc context propagation checker.
// This file covers basic/daily patterns - simple good/bad cases, shadowing, ignore directives.
// See advanced.go for real-world complex patterns and evil.go for adversarial tests.
package conc

import (
	"context"
	"fmt"

	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/pool"
)

// ===== SHOULD REPORT =====

// [BAD]: Literal without ctx
//
// Literal without ctx - basic bad case
//
// See also:
//   errgroup: badErrgroupGo
//   goroutine: badGoroutineNoCapture
//   waitgroup: badWaitGroupGo
func badConcWaitGroupGo(ctx context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
	})
	wg.Wait()
}

// [BAD]: pool.Pool.Go closure context usage
//
// pool.Pool.Go closure does not use context.
func badPoolGo(ctx context.Context) {
	p := pool.New()
	p.Go(func() { // want `pool.Pool.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
	})
	p.Wait()
}

// [BAD]: pool.ErrorPool.Go closure context usage
//
// Literal without ctx - basic bad case
func badErrorPoolGo(ctx context.Context) {
	p := pool.New().WithErrors()
	p.Go(func() error { // want `pool.ErrorPool.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
		return nil
	})
	_ = p.Wait()
}

// [BAD]: Multiple Go calls without ctx
//
// Multiple goroutine closures all fail to use the available context.
//
// See also:
//   errgroup: badErrgroupGoMultiple
//   waitgroup: badWaitGroupGoMultiple
func badConcWaitGroupGoMultiple(ctx context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
	})
	wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
	})
	wg.Wait()
}

// ===== SHOULD NOT REPORT =====

// [GOOD]: Literal with ctx - basic good case
//
// Closure directly references the context variable from enclosing scope.
//
// See also:
//   errgroup: goodErrgroupGoWithCtx
//   goroutine: goodGoroutineCapturesCtx
//   waitgroup: goodWaitGroupGoWithCtx
func goodConcWaitGroupGoWithCtx(ctx context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() {
		_ = ctx.Done()
	})
	wg.Wait()
}

// [GOOD]: pool.Pool.Go closure context usage
//
// Closure directly references the context variable from enclosing scope.
func goodPoolGoWithCtx(ctx context.Context) {
	p := pool.New()
	p.Go(func() {
		_ = ctx.Done()
	})
	p.Wait()
}

// [GOOD]: pool.ErrorPool.Go closure context usage
//
// Closure directly references the context variable from enclosing scope.
func goodErrorPoolGoWithCtx(ctx context.Context) {
	p := pool.New().WithErrors()
	p.Go(func() error {
		_ = ctx.Done()
		return nil
	})
	_ = p.Wait()
}

// [GOOD]: Literal with ctx - via function call
//
// Context is passed to helper function inside closure.
//
// See also:
//   errgroup: goodErrgroupGoCallsWithCtx
//   goroutine: goodGoroutineUsesCtxInCall
//   waitgroup: goodWaitGroupGoCallsWithCtx
func goodConcWaitGroupGoCallsWithCtx(ctx context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() {
		doWork(ctx)
	})
	wg.Wait()
}

// [GOOD]: No ctx param
//
// No ctx param - not checked
//
// See also:
//   errgroup: goodNoContextParam
//   goroutine: goodNoContextParam
//   waitgroup: goodNoContextParam
func goodNoContextParam() {
	wg := conc.WaitGroup{}
	wg.Go(func() {
	})
	wg.Wait()
}

// ===== SHADOWING TESTS =====

// [BAD]: Shadow with non-ctx type - string
//
// Shadow with non-ctx type (string)
//
// See also:
//   errgroup: badShadowingNonContext
//   goroutine: badShadowingNonContext
//   waitgroup: badShadowingNonContext
func badShadowingNonContext(ctx context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
		ctx := "not a context"
		_ = ctx
	})
	wg.Wait()
}

// [GOOD]: Uses ctx before shadow
//
// Uses ctx before shadow - valid usage
//
// See also:
//   errgroup: goodUsesCtxBeforeShadowing
//   goroutine: goodUsesCtxBeforeShadowing
//   waitgroup: goodUsesCtxBeforeShadowing
func goodUsesCtxBeforeShadowing(ctx context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() {
		_ = ctx.Done() // use ctx before shadowing
		ctx := "shadow"
		_ = ctx
	})
	wg.Wait()
}

// ===== IGNORE DIRECTIVES =====

// [GOOD]: Ignore directive - same line
//
// The //goroutinectx:ignore directive suppresses the warning.
//
// See also:
//   errgroup: goodIgnoredSameLine
//   goroutine: goodIgnoredSameLine
//   waitgroup: goodIgnoredSameLine
func goodIgnoredSameLine(ctx context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() { //goroutinectx:ignore
	})
	wg.Wait()
}

// [GOOD]: Ignore directive - previous line
//
// The //goroutinectx:ignore directive suppresses the warning.
//
// See also:
//   errgroup: goodIgnoredPreviousLine
//   goroutine: goodIgnoredPreviousLine
//   waitgroup: goodIgnoredPreviousLine
func goodIgnoredPreviousLine(ctx context.Context) {
	wg := conc.WaitGroup{}
	//goroutinectx:ignore
	wg.Go(func() {
	})
	wg.Wait()
}

// ===== MULTIPLE CONTEXT PARAMETERS =====

// [BAD]: Multiple ctx params - reports first
//
// Multiple context parameters available but none are used.
//
// See also:
//   errgroup: twoContextParams
//   goroutine: twoContextParams
//   waitgroup: twoContextParams
func twoContextParams(ctx1, ctx2 context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx1"`
	})
	wg.Wait()
}

// [GOOD]: Multiple ctx params - uses first
//
// One of the available context parameters is properly used.
//
// See also:
//   errgroup: goodUsesOneOfTwoContexts
//   goroutine: goodUsesOneOfTwoContexts
//   waitgroup: goodUsesOneOfTwoContexts
func goodUsesOneOfTwoContexts(ctx1, ctx2 context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() {
		_ = ctx1
	})
	wg.Wait()
}

// [GOOD]: Multiple ctx params - uses second
//
// One of the available context parameters is properly used.
//
// See also:
//   errgroup: goodUsesSecondOfTwoContexts
//   goroutine: goodUsesSecondOfTwoContexts
//   waitgroup: goodUsesSecondOfTwoContexts
func goodUsesSecondOfTwoContexts(ctx1, ctx2 context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() {
		_ = ctx2 // uses second context - should NOT report
	})
	wg.Wait()
}

// [GOOD]: Context as non-first param
//
// Context is detected and used even when not the first parameter.
//
// See also:
//   errgroup: goodCtxAsSecondParam
//   goroutine: goodCtxAsSecondParam
//   waitgroup: goodCtxAsSecondParam
func goodCtxAsSecondParam(logger interface{}, ctx context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() {
		_ = ctx // ctx is second param but still detected
	})
	wg.Wait()
}

// [BAD]: Context as non-first param without use
//
// Context parameter exists but is not used in the closure.
//
// See also:
//   errgroup: badCtxAsSecondParam
//   goroutine: badCtxAsSecondParam
//   waitgroup: badCtxAsSecondParam
func badCtxAsSecondParam(logger interface{}, ctx context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
		_ = logger
	})
	wg.Wait()
}

// ===== CONTEXT POOL SPECIAL CASE =====

// [GOOD]: pool.ContextPool.Go callback context usage
//
// ContextPool.Go provides context to the callback - special case.
func goodContextPoolGo(ctx context.Context) {
	p := pool.New().WithContext(ctx)
	p.Go(func(ctx context.Context) error {
		// ctx is provided by ContextPool - no need to capture outer ctx
		_ = ctx.Done()
		return nil
	})
	_ = p.Wait()
}

// [BAD]: pool.ContextPool.Go callback context usage
//
// ContextPool.Go provides context but callback doesn't use it.
func badContextPoolGoIgnoresCtx(ctx context.Context) {
	p := pool.New().WithContext(ctx)
	p.Go(func(ctx context.Context) error { // want `pool.ContextPool.Go\(\) closure should use context "ctx"`
		fmt.Println("ignores ctx")
		return nil
	})
	_ = p.Wait()
}

//vt:helper
func doWork(ctx context.Context) {
	_ = ctx
}
