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
func badCtxAsSecondParam(logger interface{}, ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		_ = logger
		return nil
	})
	_ = g.Wait()
}

func doWork(ctx context.Context) error {
	_ = ctx
	return nil
}
