//go:build go1.25

// Package waitgroup contains test fixtures for the waitgroup context propagation checker.
// This file covers basic/daily patterns - simple good/bad cases, shadowing, ignore directives.
// Note: sync.WaitGroup.Go() was added in Go 1.25.
// See advanced.go for real-world complex patterns and evil.go for adversarial tests.
package waitgroup

import (
	"context"
	"fmt"
	"sync"
)

// ===== SHOULD REPORT =====

// [BAD]: Literal without ctx
//
// Literal without ctx - basic bad case
//
// See also:
//   goroutine: badGoroutineNoCapture
//   errgroup: badErrgroupGo
func badWaitGroupGo(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
	})
	wg.Wait()
}

// [BAD]: Literal without ctx - pointer receiver
//
// Pointer receiver variant
func badWaitGroupGoPtr(ctx context.Context) {
	wg := new(sync.WaitGroup)
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
	})
	wg.Wait()
}

// [BAD]: Multiple Go calls without ctx
//
// Multiple goroutine closures all fail to use the available context.
//
// See also:
//   errgroup: badErrgroupGoMultiple
func badWaitGroupGoMultiple(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	})
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
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
func goodWaitGroupGoWithCtx(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = ctx.Done()
	})
	wg.Wait()
}

// [GOOD]: Literal with ctx - via function call
//
// Context is passed to helper function inside closure.
//
// See also:
//   goroutine: goodGoroutineUsesCtxInCall
//   errgroup: goodErrgroupGoCallsWithCtx
func goodWaitGroupGoCallsWithCtx(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		doSomething(ctx)
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
func goodNoContextParam() {
	var wg sync.WaitGroup
	wg.Go(func() {
		fmt.Println("hello")
	})
	wg.Wait()
}

// [GOOD]: Traditional pattern (Add/Done)
//
// Traditional pattern (Add/Done) - not checked by waitgroup checker
func goodTraditionalPattern(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = ctx.Done()
	}()
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
func badShadowingNonContext(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
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
func goodUsesCtxBeforeShadowing(ctx context.Context) {
	var wg sync.WaitGroup
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
func goodIgnoredSameLine(ctx context.Context) {
	var wg sync.WaitGroup
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
func goodIgnoredPreviousLine(ctx context.Context) {
	var wg sync.WaitGroup
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
func twoContextParams(ctx1, ctx2 context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx1"`
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
func goodUsesOneOfTwoContexts(ctx1, ctx2 context.Context) {
	var wg sync.WaitGroup
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
func goodUsesSecondOfTwoContexts(ctx1, ctx2 context.Context) {
	var wg sync.WaitGroup
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
func goodCtxAsSecondParam(logger interface{}, ctx context.Context) {
	var wg sync.WaitGroup
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
func badCtxAsSecondParam(logger interface{}, ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		_ = logger
	})
	wg.Wait()
}

//vt:helper
func doSomething(ctx context.Context) {
	_ = ctx
}
