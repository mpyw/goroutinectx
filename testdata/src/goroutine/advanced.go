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

func (w *worker) run() {
	fmt.Println("running:", w.name)
}

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
