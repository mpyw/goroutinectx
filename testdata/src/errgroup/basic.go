// Package errgroup contains test fixtures for the errgroup context propagation checker.
// This file covers basic/daily patterns - simple good/bad cases, shadowing, ignore directives.
// See advanced.go for real-world complex patterns and evil.go for adversarial tests.
package errgroup

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

// ===== SHOULD REPORT =====

// [BAD]: Literal without ctx
//
// Literal without ctx - basic bad case
//
// See also:
//   goroutine: badGoroutineNoCapture
//   waitgroup: badWaitGroupGo
func badErrgroupGo(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Literal without ctx - TryGo
//
// TryGo without ctx
func badErrgroupTryGo(ctx context.Context) {
	g := new(errgroup.Group)
	g.TryGo(func() error { // want `errgroup.Group.TryGo\(\) closure should use context "ctx"`
		fmt.Println("no context")
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Multiple Go calls without ctx
//
// Multiple goroutine closures all fail to use the available context.
//
// See also:
//   waitgroup: badWaitGroupGoMultiple
func badErrgroupGoMultiple(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		return nil
	})
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		return nil
	})
	_ = g.Wait()
}

// ===== SHOULD NOT REPORT =====

// [GOOD]: Literal with ctx - basic good case
//
// Closure directly references the context variable from enclosing scope.
//
// See also:
//   goroutine: goodGoroutineCapturesCtx
//   waitgroup: goodWaitGroupGoWithCtx
func goodErrgroupGoWithCtx(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx.Done()
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Literal with ctx - via function call
//
// Context is passed to helper function inside closure.
//
// See also:
//   goroutine: goodGoroutineUsesCtxInCall
//   waitgroup: goodWaitGroupGoCallsWithCtx
func goodErrgroupGoCallsWithCtx(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		return doWork(ctx)
	})
	_ = g.Wait()
}

// [GOOD]: Literal with derived ctx - errgroup.WithContext
//
// errgroup.WithContext pattern
func goodErrgroupWithContext(ctx context.Context) {
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		_ = ctx.Done()
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Derived ctx in for loop with function variable
//
// Real-world production pattern: derived ctx used in closure inside for loop
func goodErrgroupWithContextForLoop(ctx context.Context) {
	eg, ctx := errgroup.WithContext(ctx)
	for _, f := range []func(context.Context) error{doWork, doWork} {
		eg.Go(func() error {
			return f(ctx) // uses derived ctx
		})
	}
	_ = eg.Wait()
}

// [GOOD]: No ctx param
//
// No ctx param - not checked
//
// See also:
//   goroutine: goodNoContextParam
//   waitgroup: goodNoContextParam
func goodNoContextParam() {
	g := new(errgroup.Group)
	g.Go(func() error {
		return nil
	})
	_ = g.Wait()
}

// ===== SHADOWING TESTS =====

// [BAD]: Shadow with non-ctx type - string
//
// Shadow with non-ctx type (string)
//
// See also:
//   goroutine: badShadowingNonContext
//   waitgroup: badShadowingNonContext
func badShadowingNonContext(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		ctx := "not a context"
		_ = ctx
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Uses ctx before shadow
//
// Uses ctx before shadow - valid usage
//
// See also:
//   goroutine: goodUsesCtxBeforeShadowing
//   waitgroup: goodUsesCtxBeforeShadowing
func goodUsesCtxBeforeShadowing(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx.Done() // use ctx before shadowing
		ctx := "shadow"
		_ = ctx
		return nil
	})
	_ = g.Wait()
}

// ===== IGNORE DIRECTIVES =====

// [GOOD]: Ignore directive - same line
//
// The //goroutinectx:ignore directive suppresses the warning.
//
// See also:
//   goroutine: goodIgnoredSameLine
//   waitgroup: goodIgnoredSameLine
func goodIgnoredSameLine(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { //goroutinectx:ignore
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Ignore directive - previous line
//
// The //goroutinectx:ignore directive suppresses the warning.
//
// See also:
//   goroutine: goodIgnoredPreviousLine
//   waitgroup: goodIgnoredPreviousLine
func goodIgnoredPreviousLine(ctx context.Context) {
	g := new(errgroup.Group)
	//goroutinectx:ignore
	g.Go(func() error {
		return nil
	})
	_ = g.Wait()
}

// ===== MULTIPLE CONTEXT PARAMETERS =====

// [BAD]: Multiple ctx params - reports first
//
// Multiple context parameters available but none are used.
//
// See also:
//   goroutine: twoContextParams
//   waitgroup: twoContextParams
func twoContextParams(ctx1, ctx2 context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx1"`
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Multiple ctx params - uses first
//
// One of the available context parameters is properly used.
//
// See also:
//   goroutine: goodUsesOneOfTwoContexts
//   waitgroup: goodUsesOneOfTwoContexts
func goodUsesOneOfTwoContexts(ctx1, ctx2 context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx1
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Multiple ctx params - uses second
//
// One of the available context parameters is properly used.
//
// See also:
//   goroutine: goodUsesSecondOfTwoContexts
//   waitgroup: goodUsesSecondOfTwoContexts
func goodUsesSecondOfTwoContexts(ctx1, ctx2 context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx2 // uses second context - should NOT report
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Context as non-first param
//
// Context is detected and used even when not the first parameter.
//
// See also:
//   goroutine: goodCtxAsSecondParam
//   waitgroup: goodCtxAsSecondParam
func goodCtxAsSecondParam(logger interface{}, ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx // ctx is second param but still detected
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Context as non-first param without use
//
// Context parameter exists but is not used in the closure.
//
// See also:
//   goroutine: badCtxAsSecondParam
//   waitgroup: badCtxAsSecondParam
func badCtxAsSecondParam(logger interface{}, ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		_ = logger
		return nil
	})
	_ = g.Wait()
}

//vt:helper
func doWork(ctx context.Context) error {
	_ = ctx
	return nil
}
