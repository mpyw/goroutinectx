// Functions with more than one context parameter.

package patterns

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ===== Context as non-first param =====

// [GOOD]: Context as non-first param
//
// Context is detected and used even when not the first parameter.
func goodCtxAsSecondParamGoroutine(logger interface{}, ctx context.Context) {
	go func() {
		_ = ctx // ctx is second param but still detected
	}()
}

// [GOOD]: Context as non-first param
//
// Context is detected and used even when not the first parameter.
func goodCtxAsSecondParamErrgroup(logger interface{}, ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx // ctx is second param but still detected
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Context as non-first param
//
// Context is detected and used even when not the first parameter.
func goodCtxAsSecondParamWaitGroup(logger interface{}, ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = ctx // ctx is second param but still detected
	})
	wg.Wait()
}

// [BAD]: Context as non-first param without use
//
// Context parameter exists but is not used in the closure.
func badCtxAsSecondParamGoroutine(logger interface{}, ctx context.Context) {
	go func() { // want `goroutine does not propagate context "ctx"`
		_ = logger
	}()
}

// [BAD]: Context as non-first param without use
//
// Context parameter exists but is not used in the closure.
func badCtxAsSecondParamErrgroup(logger interface{}, ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		_ = logger
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Context as non-first param without use
//
// Context parameter exists but is not used in the closure.
func badCtxAsSecondParamWaitGroup(logger interface{}, ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		_ = logger
	})
	wg.Wait()
}

// ===== Two ctx params =====

// [BAD]: Multiple ctx params - reports first
//
// Multiple context parameters available but none are used.
func twoContextParamsGoroutine(ctx1, ctx2 context.Context) {
	go func() { // want `goroutine does not propagate context "ctx1"`
		fmt.Println("ignoring both contexts")
	}()
}

// [BAD]: Multiple ctx params - reports first
//
// Multiple context parameters available but none are used.
func twoContextParamsErrgroup(ctx1, ctx2 context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx1"`
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Multiple ctx params - reports first
//
// Multiple context parameters available but none are used.
func twoContextParamsWaitGroup(ctx1, ctx2 context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx1"`
	})
	wg.Wait()
}

// [GOOD]: Multiple ctx params - uses first
//
// One of the available context parameters is properly used.
func goodUsesOneOfTwoContextsGoroutine(ctx1, ctx2 context.Context) {
	go func() {
		_ = ctx1 // uses first context
	}()
}

// [GOOD]: Multiple ctx params - uses first
//
// One of the available context parameters is properly used.
func goodUsesOneOfTwoContextsErrgroup(ctx1, ctx2 context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx1
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Multiple ctx params - uses first
//
// One of the available context parameters is properly used.
func goodUsesOneOfTwoContextsWaitGroup(ctx1, ctx2 context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = ctx1
	})
	wg.Wait()
}

// [GOOD]: Multiple ctx params - uses second
//
// One of the available context parameters is properly used.
func goodUsesSecondOfTwoContextsGoroutine(ctx1, ctx2 context.Context) {
	go func() {
		_ = ctx2 // uses second context - should NOT report
	}()
}

// [GOOD]: Multiple ctx params - uses second
//
// One of the available context parameters is properly used.
func goodUsesSecondOfTwoContextsErrgroup(ctx1, ctx2 context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx2 // uses second context - should NOT report
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Multiple ctx params - uses second
//
// One of the available context parameters is properly used.
func goodUsesSecondOfTwoContextsWaitGroup(ctx1, ctx2 context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = ctx2 // uses second context - should NOT report
	})
	wg.Wait()
}

// [GOOD]: Both contexts used
//
// When multiple contexts exist, using any one satisfies the check.
func goodUsesBothContextsGoroutine(ctx1, ctx2 context.Context) {
	go func() {
		_ = ctx1
		_ = ctx2
	}()
}

// [GOOD]: Both contexts used
//
// When multiple contexts exist, using any one satisfies the check.
func goodUsesBothContextsErrgroup(ctx1, ctx2 context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx1
		_ = ctx2
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Both contexts used
//
// When multiple contexts exist, using any one satisfies the check.
func goodUsesBothContextsWaitGroup(ctx1, ctx2 context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = ctx1
		_ = ctx2
	})
	wg.Wait()
}

// ===== Multiple ctx in separate param groups =====

// [GOOD]: Multiple ctx in separate param groups
//
// Context in separate parameter group is detected and used.
func goodMultipleCtxSeparateGroupsGoroutine(a int, ctx1 context.Context, b string, ctx2 context.Context) {
	go func() {
		_ = ctx2 // uses second ctx from different param group
	}()
}

// [GOOD]: Multiple ctx in separate param groups
//
// Context in separate parameter group is detected and used.
func goodMultipleCtxSeparateGroupsErrgroup(a int, ctx1 context.Context, b string, ctx2 context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx2 // uses second ctx from different param group
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Multiple ctx in separate param groups
//
// Context in separate parameter group is detected and used.
func goodMultipleCtxSeparateGroupsWaitGroup(a int, ctx1 context.Context, b string, ctx2 context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = ctx2 // uses second ctx from different param group
	})
	wg.Wait()
}

// [BAD]: Multiple ctx in separate param groups - none used
//
// Context in separate parameter group is not used.
func badMultipleCtxSeparateGroupsGoroutine(a int, ctx1 context.Context, b string, ctx2 context.Context) {
	go func() { // want `goroutine does not propagate context "ctx1"`
		fmt.Println(a, b) // uses other params but not ctx
	}()
}

// [BAD]: Multiple ctx in separate param groups - none used
//
// Context in separate parameter group is not used.
func badMultipleCtxSeparateGroupsErrgroup(a int, ctx1 context.Context, b string, ctx2 context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx1"`
		fmt.Println(a, b) // uses other params but not ctx
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Multiple ctx in separate param groups - none used
//
// Context in separate parameter group is not used.
func badMultipleCtxSeparateGroupsWaitGroup(a int, ctx1 context.Context, b string, ctx2 context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx1"`
		fmt.Println(a, b) // uses other params but not ctx
	})
	wg.Wait()
}

// ===== Three ctx params =====

// [GOOD]: Three contexts - uses middle one
//
// Using the middle of multiple context parameters is valid.
func goodUsesMiddleOfThreeContextsGoroutine(ctx1, ctx2, ctx3 context.Context) {
	go func() {
		_ = ctx2 // uses middle context
	}()
}

// [GOOD]: Three contexts - uses middle one
//
// Using the middle of multiple context parameters is valid.
func goodUsesMiddleOfThreeContextsErrgroup(ctx1, ctx2, ctx3 context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx2 // uses middle context
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Three contexts - uses middle one
//
// Using the middle of multiple context parameters is valid.
func goodUsesMiddleOfThreeContextsWaitGroup(ctx1, ctx2, ctx3 context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = ctx2 // uses middle context
	})
	wg.Wait()
}

// [GOOD]: Three contexts - uses last one
//
// Using the last of multiple context parameters is valid.
func goodUsesLastOfThreeContextsGoroutine(ctx1, ctx2, ctx3 context.Context) {
	go func() {
		_ = ctx3 // uses last context
	}()
}

// [GOOD]: Three contexts - uses last one
//
// Using the last of multiple context parameters is valid.
func goodUsesLastOfThreeContextsErrgroup(ctx1, ctx2, ctx3 context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx3 // uses last context
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Three contexts - uses last one
//
// Using the last of multiple context parameters is valid.
func goodUsesLastOfThreeContextsWaitGroup(ctx1, ctx2, ctx3 context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = ctx3 // uses last context
	})
	wg.Wait()
}
