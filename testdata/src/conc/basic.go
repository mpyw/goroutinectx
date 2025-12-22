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

// [BAD]: Literal without ctx - conc.WaitGroup
//
// Literal without ctx - basic bad case
//
// See also:
//   goroutine: badGoroutineNoCapture
//   errgroup: badErrgroupGo
//   waitgroup: badWaitGroupGo
func badConcWaitGroupGo(ctx context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
	})
	wg.Wait()
}

// [BAD]: Literal without ctx - pool.Pool
//
// Literal without ctx - basic bad case
//
// See also:
//   goroutine: badGoroutineNoCapture
//   errgroup: badErrgroupGo
//   waitgroup: badWaitGroupGo
func badPoolGo(ctx context.Context) {
	p := pool.New()
	p.Go(func() { // want `pool.Pool.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
	})
	p.Wait()
}

// [BAD]: Literal without ctx - pool.ErrorPool
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
//   goroutine: goodGoroutineCapturesCtx
//   errgroup: goodErrgroupGoWithCtx
//   waitgroup: goodWaitGroupGoWithCtx
func goodConcWaitGroupGoWithCtx(ctx context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() {
		_ = ctx.Done()
	})
	wg.Wait()
}

// [GOOD]: Literal with ctx - pool.Pool
//
// Closure directly references the context variable from enclosing scope.
func goodPoolGoWithCtx(ctx context.Context) {
	p := pool.New()
	p.Go(func() {
		_ = ctx.Done()
	})
	p.Wait()
}

// [GOOD]: Literal with ctx - pool.ErrorPool
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
//   goroutine: goodGoroutineUsesCtxInCall
//   errgroup: goodErrgroupGoCallsWithCtx
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
//   goroutine: goodNoContextParam
//   errgroup: goodNoContextParam
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
//   goroutine: badShadowingNonContext
//   errgroup: badShadowingNonContext
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
//   goroutine: goodUsesCtxBeforeShadowing
//   errgroup: goodUsesCtxBeforeShadowing
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
//   goroutine: goodIgnoredSameLine
//   errgroup: goodIgnoredSameLine
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
//   goroutine: goodIgnoredPreviousLine
//   errgroup: goodIgnoredPreviousLine
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
//   goroutine: twoContextParams
//   errgroup: twoContextParams
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
//   goroutine: goodUsesOneOfTwoContexts
//   errgroup: goodUsesOneOfTwoContexts
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
//   goroutine: goodUsesSecondOfTwoContexts
//   errgroup: goodUsesSecondOfTwoContexts
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
//   goroutine: goodCtxAsSecondParam
//   errgroup: goodCtxAsSecondParam
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
//   goroutine: badCtxAsSecondParam
//   errgroup: badCtxAsSecondParam
//   waitgroup: badCtxAsSecondParam
func badCtxAsSecondParam(logger interface{}, ctx context.Context) {
	wg := conc.WaitGroup{}
	wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
		_ = logger
	})
	wg.Wait()
}

// ===== CONTEXT POOL SPECIAL CASE =====

// [GOOD]: pool.ContextPool.Go receives ctx via callback parameter
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

// [BAD]: pool.ContextPool.Go callback ignores provided ctx
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
