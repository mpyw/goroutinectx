// Package goroutine contains test fixtures for the goroutine context propagation checker.
// The cases it shares with other checkers are in testdata/src/patterns.
// This file covers basic/daily patterns - single goroutine, shadowing, ignore directives.
// See advanced.go for real-world complex patterns and evil.go for adversarial tests.
package goroutine

import (
	"context"
	"fmt"
)

// ===== SHOULD REPORT =====

// [BAD]: Literal without ctx - variant
//
// Closure variation that does not capture context.
func badGoroutineIgnoresCtx(ctx context.Context) {
	go func() { // want `goroutine does not propagate context "ctx"`
		x := 1
		_ = x
	}()
}

// ===== SHOULD NOT REPORT =====

// [GOOD]: Literal with derived ctx
//
// Closure uses a context derived from errgroup.WithContext.
func goodGoroutineWithDerivedCtx(ctx context.Context) {
	go func() {
		ctx2, cancel := context.WithCancel(ctx)
		defer cancel()
		_ = ctx2
	}()
}

// [GOOD]: Literal with ctx in select
//
// Goroutine with select statement properly uses context.
func goodGoroutineSelectOnCtx(ctx context.Context) {
	go func() {
		select {
		case <-ctx.Done():
			return
		default:
		}
	}()
}

// ===== SHADOWING TESTS =====

// [BAD]: Shadow with non-ctx type - channel
//
// Shadow with non-ctx type (channel)
func badShadowingWithDifferentType(ctx context.Context) {
	go func() { // want `goroutine does not propagate context "ctx"`
		ctx := make(chan int) // shadows with channel
		close(ctx)
	}()
}

// [BAD]: Shadow with non-ctx type - function
//
// Shadow with non-ctx type (function)
func badShadowingWithFunction(ctx context.Context) {
	go func() { // want `goroutine does not propagate context "ctx"`
		ctx := func() {} // shadows with function
		ctx()
	}()
}

// [BAD]: Shadow in nested block
//
// Context is shadowed within a nested block scope.
func badShadowingInNestedBlock(ctx context.Context) {
	go func() { // want `goroutine does not propagate context "ctx"`
		if true {
			ctx := "shadowed in block"
			_ = ctx
		}
	}()
}

// ===== CONTEXT FROM LOCAL VARIABLE =====

// [NOTCHECKED]: No ctx param (local var)
//
// No ctx param (local var) - not checked
func notCheckedLocalContextVariable() {
	// No context parameter, so not checked
	ctx := context.Background()
	go func() {
		fmt.Println("local context not checked")
	}()
	_ = ctx
}

// ===== CONTEXT PASSED AS ARGUMENT =====

// [GOOD]: Ctx passed as argument to goroutine
//
// Context is passed as argument when spawning the goroutine.
func goodGoroutinePassesCtxAsArg(ctx context.Context) {
	go func(c context.Context) {
		_ = c.Done() // uses its own param
	}(ctx)
}

// ===== DIRECT FUNCTION CALL =====

// [GOOD]: Direct function call
//
// Direct function call - not a func literal
func goodDirectFunctionCall(ctx context.Context) {
	go doSomething(ctx) // not a func literal
}

func doSomething(ctx context.Context) {
	_ = ctx
}

// ===== IGNORE DIRECTIVES =====

// [GOOD]: Ignore directive - with reason
//
// The //goroutinectx:ignore directive suppresses the warning.
func goodIgnoredWithReason(ctx context.Context) {
	go func() { //goroutinectx:ignore // intentionally fire-and-forget
		fmt.Println("background task")
	}()
}

// [GOOD]: Ignore directive - checker-specific (goroutine)
//
// The //goroutinectx:ignore directive can specify which checker to ignore.
func goodIgnoredCheckerSpecific(ctx context.Context) {
	go func() { //goroutinectx:ignore goroutine
		fmt.Println("background task")
	}()
}

// [GOOD]: Ignore directive - checker-specific with comment
//
// Checker-specific ignore can also have a comment.
func goodIgnoredCheckerSpecificWithComment(ctx context.Context) {
	go func() { //goroutinectx:ignore goroutine // fire-and-forget
		fmt.Println("background task")
	}()
}

// [BAD]: Ignore directive - unused checker-specific
//
// Specifying an unrelated checker doesn't suppress warnings from other checkers.
func badIgnoredWrongChecker(ctx context.Context) {
	//goroutinectx:ignore errgroup // want `unused goroutinectx:ignore directive for checker\(s\): errgroup`
	go func() { // want `goroutine does not propagate context "ctx"`
		fmt.Println("background task")
	}()
}

// [BAD]: Ignore directive - completely unused
//
// An ignore directive that doesn't suppress any warning is reported as unused.
func badUnusedIgnore(ctx context.Context) {
	//goroutinectx:ignore // want `unused goroutinectx:ignore directive`
	go func() {
		_ = ctx // Context is used, no warning generated
	}()
}
