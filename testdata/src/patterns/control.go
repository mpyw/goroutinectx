// Spawns under if, for, range and defer.

package patterns

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ===== Conditional =====

// [BAD]: Conditional Go without ctx
//
// Conditional branches spawn goroutines without using context.
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

// [BAD]: Conditional Go without ctx
//
// Conditional branches spawn goroutines without using context.
func badConditionalGoErrgroup(ctx context.Context, flag bool) {
	g := new(errgroup.Group)
	if flag {
		g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
			return nil
		})
	} else {
		g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
			return nil
		})
	}
	_ = g.Wait()
}

// [BAD]: Conditional Go without ctx
//
// Conditional branches spawn goroutines without using context.
func badConditionalGoWaitGroup(ctx context.Context, flag bool) {
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

// [GOOD]: Conditional goroutine with ctx
//
// All conditional branches properly use context in goroutines.
func goodConditionalGoErrgroup(ctx context.Context, flag bool) {
	g := new(errgroup.Group)
	if flag {
		g.Go(func() error {
			_ = ctx
			return nil
		})
	} else {
		g.Go(func() error {
			_ = ctx
			return nil
		})
	}
	_ = g.Wait()
}

// [GOOD]: Conditional goroutine with ctx
//
// All conditional branches properly use context in goroutines.
func goodConditionalGoWaitGroup(ctx context.Context, flag bool) {
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

// ===== For loop =====

// [BAD]: Go in for loop without ctx
//
// Goroutines spawned in loop iterations do not use context.
func badGoroutinesInLoop(ctx context.Context) {
	for i := 0; i < 3; i++ {
		go func() { // want `goroutine does not propagate context "ctx"`
			fmt.Println("loop iteration")
		}()
	}
}

// [BAD]: Go in for loop without ctx
//
// Goroutines spawned in loop iterations do not use context.
func badLoopGoErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	for i := 0; i < 3; i++ {
		g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
			return nil
		})
	}
	_ = g.Wait()
}

// [BAD]: Go in for loop without ctx
//
// Goroutines spawned in loop iterations do not use context.
func badLoopGoWaitGroup(ctx context.Context) {
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
func goodGoroutinesInLoopWithCtx(ctx context.Context) {
	for i := 0; i < 3; i++ {
		go func() {
			_ = ctx
		}()
	}
}

// [GOOD]: Goroutine in for loop with ctx
//
// Goroutines in loop properly capture and use context.
func goodLoopGoErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	for i := 0; i < 3; i++ {
		g.Go(func() error {
			_ = ctx
			return nil
		})
	}
	_ = g.Wait()
}

// [GOOD]: Goroutine in for loop with ctx
//
// Goroutines in loop properly capture and use context.
func goodLoopGoWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Go(func() {
			_ = ctx
		})
	}
	wg.Wait()
}

// ===== Range loop =====

// [BAD]: Go in range loop without ctx
//
// Goroutines spawned in loop iterations do not use context.
func badGoroutinesInRangeLoop(ctx context.Context) {
	items := []int{1, 2, 3}
	for _, item := range items {
		go func() { // want `goroutine does not propagate context "ctx"`
			fmt.Println(item)
		}()
	}
}

// [BAD]: Go in range loop without ctx
//
// Goroutines spawned in loop iterations do not use context.
func badRangeLoopGoErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	items := []int{1, 2, 3}
	for _, item := range items {
		g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
			fmt.Println(item)
			return nil
		})
	}
	_ = g.Wait()
}

// [BAD]: Go in range loop without ctx
//
// Goroutines spawned in loop iterations do not use context.
func badRangeLoopGoWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	items := []int{1, 2, 3}
	for _, item := range items {
		wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
			fmt.Println(item)
		})
	}
	wg.Wait()
}

// ===== Defer =====

// [BAD]: Closure with defer but no ctx
//
// Closure with defer statement does not use context.
func badDeferInClosureErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		defer fmt.Println("deferred")
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Closure with defer but no ctx
//
// Closure with defer statement does not use context.
func badDeferInClosureWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		defer fmt.Println("deferred")
	})
	wg.Wait()
}

// [GOOD]: Literal with ctx in select - with defer
//
// Closure with ctx and defer
func goodDeferWithCtxDirectErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx // use ctx directly
		defer fmt.Println("cleanup")
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Literal with derived ctx - with defer
//
// Closure with ctx and defer
func goodDeferWithCtxDirectWaitGroup(ctx context.Context) {
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
func goodDeferNestedClosureGoroutine(ctx context.Context) {
	go func() { // SSA correctly detects ctx capture
		defer func() {
			_ = ctx.Done() // ctx in deferred closure - SSA captures this
		}()
	}()
}

// [GOOD]: Ctx in deferred nested closure - SSA correctly detects
//
// SSA FreeVars propagation correctly detects context captured in nested closures.
func goodDeferNestedClosureErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { // SSA correctly detects ctx capture
		// ctx in nested closure - SSA captures this
		defer func() { _ = ctx }()
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Ctx in deferred nested closure - SSA correctly detects
//
// SSA FreeVars propagation correctly detects context captured in nested closures.
func goodDeferNestedClosureWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // SSA correctly detects ctx capture
		// ctx in nested closure - SSA captures this
		defer func() { _ = ctx }()
	})
	wg.Wait()
}
