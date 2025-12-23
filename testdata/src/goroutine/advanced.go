// Package goroutine contains test fixtures for the goroutine context propagation checker.
// This file covers advanced patterns - real-world complex patterns that are not daily
// but commonly seen in production code: defer, loops, channels, WaitGroup, method calls.
// See basic.go for daily patterns and evil.go for adversarial tests.
package goroutine

import (
	"context"
	"fmt"
	"sync"
)

// ===== DEFER PATTERNS =====

// [BAD]: Defer without ctx
//
// Closure with defer statement does not use context.
func badGoroutineWithDefer(ctx context.Context) {
	go func() { // want `goroutine does not propagate context "ctx"`
		defer fmt.Println("deferred")
		fmt.Println("body")
	}()
}

// [GOOD]: Ctx in deferred nested closure - SSA correctly detects
//
// SSA FreeVars propagation correctly detects context captured in nested closures.
//
// See also:
//   errgroup: goodDeferNestedClosure
//   waitgroup: goodDeferNestedClosure
func goodDeferNestedClosure(ctx context.Context) {
	go func() { // SSA correctly detects ctx capture
		defer func() {
			_ = ctx.Done() // ctx in deferred closure - SSA captures this
		}()
	}()
}

// [BAD]: Defer with recovery, no ctx
//
// Closure with defer statement does not use context.
func badGoroutineWithRecovery(ctx context.Context) {
	go func() { // want `goroutine does not propagate context "ctx"`
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("recovered:", r)
			}
		}()
		panic("test")
	}()
}

// [GOOD]: Ctx in recovery closure - SSA correctly detects
//
// SSA FreeVars propagation correctly detects context captured in nested closures.
func goodGoroutineUsesCtxInRecoveryClosure(ctx context.Context) {
	go func() { // SSA correctly detects ctx capture
		defer func() {
			if r := recover(); r != nil {
				_ = ctx // ctx in recovery closure - SSA captures this
			}
		}()
		panic("test")
	}()
}

// ===== GOROUTINE IN LOOP =====

// [BAD]: Go in for loop without ctx
//
// Goroutines spawned in loop iterations do not use context.
//
// See also:
//   errgroup: badLoopGo
//   waitgroup: badLoopGo
func badGoroutinesInLoop(ctx context.Context) {
	for i := 0; i < 3; i++ {
		go func() { // want `goroutine does not propagate context "ctx"`
			fmt.Println("loop iteration")
		}()
	}
}

// [GOOD]: Goroutine in for loop with ctx
//
// Goroutines in loop properly capture and use context.
func goodGoroutinesInLoopWithCtx(ctx context.Context) {
	for i := 0; i < 3; i++ {
		go func() {
			_ = ctx
		}()
	}
}

// [BAD]: Go in range loop without ctx
//
// Goroutines spawned in loop iterations do not use context.
//
// See also:
//   errgroup: badRangeLoopGo
//   waitgroup: badRangeLoopGo
func badGoroutinesInRangeLoop(ctx context.Context) {
	items := []int{1, 2, 3}
	for _, item := range items {
		go func() { // want `goroutine does not propagate context "ctx"`
			fmt.Println(item)
		}()
	}
}

// ===== CONDITIONAL GOROUTINE =====

// [BAD]: Conditional Go without ctx
//
// Conditional branches spawn goroutines without using context.
//
// See also:
//   errgroup: badConditionalGo
//   waitgroup: badConditionalGo
func badConditionalGoroutine(ctx context.Context, flag bool) {
	if flag {
		go func() { // want `goroutine does not propagate context "ctx"`
			fmt.Println("if branch")
		}()
	} else {
		go func() { // want `goroutine does not propagate context "ctx"`
			fmt.Println("else branch")
		}()
	}
}

// [GOOD]: Conditional goroutine with ctx
//
// All conditional branches properly use context in goroutines.
func goodConditionalGoroutine(ctx context.Context, flag bool) {
	if flag {
		go func() {
			_ = ctx
		}()
	} else {
		go func() {
			_ = ctx
		}()
	}
}

// ===== CHANNEL OPERATIONS =====

// [BAD]: Channel send without ctx
//
// Goroutine using channels does not propagate context.
func badGoroutineWithChannelSend(ctx context.Context) {
	ch := make(chan int)
	go func() { // want `goroutine does not propagate context "ctx"`
		ch <- 42
	}()
	<-ch
}

// [GOOD]: Channel with select on ctx
//
// Channel with select on ctx.Done()
func goodGoroutineWithChannelAndCtx(ctx context.Context) {
	ch := make(chan int)
	go func() {
		select {
		case ch <- 42:
		case <-ctx.Done():
			return
		}
	}()
	<-ch
}

// [BAD]: Channel result pattern
//
// Goroutine using channels does not propagate context.
func badGoroutineReturnsViaChannel(ctx context.Context) {
	result := make(chan int)
	go func() { // want `goroutine does not propagate context "ctx"`
		result <- compute()
	}()
	<-result
}

// [GOOD]: Channel result pattern
//
// Goroutine using channels properly captures context.
func goodGoroutineReturnsWithCtx(ctx context.Context) {
	result := make(chan int)
	go func() {
		select {
		case result <- compute():
		case <-ctx.Done():
		}
	}()
	<-result
}

//vt:helper
func compute() int { return 42 }

// ===== SELECT PATTERNS =====

// [BAD]: Select statement
//
// Select without ctx.Done() case
func badGoroutineWithMultiCaseSelect(ctx context.Context) {
	ch1 := make(chan int)
	ch2 := make(chan int)
	go func() { // want `goroutine does not propagate context "ctx"`
		select {
		case <-ch1:
			fmt.Println("ch1")
		case <-ch2:
			fmt.Println("ch2")
		}
	}()
}

// [GOOD]: Select statement
//
// Select with ctx.Done() case
func goodGoroutineWithCtxInSelect(ctx context.Context) {
	ch1 := make(chan int)
	go func() {
		select {
		case <-ch1:
			fmt.Println("ch1")
		case <-ctx.Done():
			return
		}
	}()
}

// ===== WAITGROUP PATTERN =====

// [BAD]: Waitgroup pattern
//
// Traditional WaitGroup Add/Done pattern without context usage.
func badGoroutineWithWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // want `goroutine does not propagate context "ctx"`
		defer wg.Done()
		fmt.Println("work")
	}()
	wg.Wait()
}

// [GOOD]: Waitgroup pattern
//
// Traditional Add/Done pattern is not checked by the waitgroup checker.
func goodGoroutineWithWaitGroupAndCtx(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-ctx.Done():
			return
		default:
			fmt.Println("work")
		}
	}()
	wg.Wait()
}

// ===== METHOD CALLS =====

type worker struct {
	name string
}

//vt:helper
func (w *worker) run() {
	fmt.Println("running:", w.name)
}

//vt:helper
func (w *worker) runWithCtx(ctx context.Context) {
	_ = ctx
	fmt.Println("running:", w.name)
}

// [BAD]: Method call
//
// Method called in goroutine does not receive context.
func badGoroutineCallsMethodWithoutCtx(ctx context.Context) {
	w := &worker{name: "test"}
	go func() { // want `goroutine does not propagate context "ctx"`
		w.run()
	}()
}

// [GOOD]: Method call
//
// Method called in goroutine properly receives context.
func goodGoroutineCallsMethodWithCtx(ctx context.Context) {
	w := &worker{name: "test"}
	go func() {
		w.runWithCtx(ctx)
	}()
}

// ===== MULTIPLE VARIABLE CAPTURE =====

// [BAD]: Captures other vars but not ctx
//
// Closure captures other variables but not context.
func badGoroutineCapturesOtherButNotCtx(ctx context.Context) {
	x := 42
	y := "hello"
	go func() { // want `goroutine does not propagate context "ctx"`
		fmt.Println(x, y) // captures x, y but NOT ctx
	}()
}

// [GOOD]: Captures ctx among other vars
//
// Closure captures multiple variables including context.
func goodGoroutineCapturesCtxAmongOthers(ctx context.Context) {
	x := 42
	y := "hello"
	go func() {
		fmt.Println(x, y)
		_ = ctx
	}()
}

// ===== CONTROL FLOW =====

// [BAD]: Loop inside goroutine
//
// Loop inside goroutine body does not check context.
func badGoroutineWithLoop(ctx context.Context) {
	go func() { // want `goroutine does not propagate context "ctx"`
		for i := 0; i < 10; i++ {
			fmt.Println(i)
		}
	}()
}

// [GOOD]: Loop inside goroutine
//
// Loop inside goroutine body uses context for cancellation.
func goodGoroutineUsesCtxInLoop(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// work
			}
		}
	}()
}

// [BAD]: Switch inside goroutine
//
// Goroutine with switch statement does not use context.
func badGoroutineWithSwitch(ctx context.Context) {
	go func() { // want `goroutine does not propagate context "ctx"`
		switch x := 1; x {
		case 1:
			fmt.Println("one")
		default:
			fmt.Println("other")
		}
	}()
}

// [GOOD]: Switch inside goroutine
//
// Goroutine with switch statement properly uses context.
func goodGoroutineUsesCtxInSwitch(ctx context.Context) {
	go func() {
		switch {
		case ctx.Err() != nil:
			return
		default:
			// continue
		}
	}()
}

// ===== INNER FUNCTION PATTERNS =====

// [GOOD]: Inner func has own ctx param
//
// Inner function declares its own context parameter and uses it.
//
// See also:
//   errgroup: goodNestedInnerHasOwnCtx
//   waitgroup: goodNestedInnerHasOwnCtx
func goodShadowingInnerCtxParam(outerCtx context.Context) {
	go func(ctx context.Context) {
		_ = ctx.Done() // uses inner ctx - OK
	}(outerCtx)
}

// ===== INDEX EXPRESSION PATTERNS =====

// [GOOD]: Index expression captures ctx
//
// Function in slice captures context.
func goodIndexExprCapturesCtx(ctx context.Context) {
	handlers := []func(){
		func() { _ = ctx },
	}
	go handlers[0]()
}

// [BAD]: Index expression captures ctx
//
// Function in slice does not capture context.
func badIndexExprMissingCtx(ctx context.Context) {
	handlers := []func(){
		func() { fmt.Println("no ctx") },
	}
	go handlers[0]() // want `goroutine does not propagate context "ctx"`
}

// ===== MAP INDEX EXPRESSION PATTERNS =====

// [GOOD]: Map index expression captures ctx
//
// Function in map with string key captures context.
func goodMapIndexCapturesCtx(ctx context.Context) {
	handlers := map[string]func(){
		"work": func() { _ = ctx },
	}
	go handlers["work"]()
}

// [BAD]: Map index expression captures ctx
//
// Function in map with string key does not capture context.
func badMapIndexMissingCtx(ctx context.Context) {
	handlers := map[string]func(){
		"work": func() { fmt.Println("no ctx") },
	}
	go handlers["work"]() // want `goroutine does not propagate context "ctx"`
}

// ===== STRUCT FIELD SELECTOR PATTERNS =====

// [GOOD]: Struct field selector captures ctx
//
// Function in struct field captures context.
func goodStructFieldCapturesCtx(ctx context.Context) {
	s := struct{ handler func() }{
		handler: func() { _ = ctx },
	}
	go s.handler()
}

// [BAD]: Struct field selector captures ctx
//
// Function in struct field does not capture context.
func badStructFieldMissingCtx(ctx context.Context) {
	s := struct{ handler func() }{
		handler: func() { fmt.Println("no ctx") },
	}
	go s.handler() // want `goroutine does not propagate context "ctx"`
}
