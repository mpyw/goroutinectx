// ctx shadowed by another variable or by an inner ctx parameter.

package patterns

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ===== Shadow with non-ctx type - string =====

// [BAD]: Shadow with non-ctx type - string
//
// Shadow with non-ctx type (string)
func badShadowingNonContextGoroutine(ctx context.Context) {
	go func() { // want `goroutine does not propagate context "ctx"`
		ctx := "not a context" // shadows with string
		_ = ctx
	}()
}

// [BAD]: Shadow with non-ctx type - string
//
// Shadow with non-ctx type (string)
func badShadowingNonContextErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		ctx := "not a context"
		_ = ctx
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Shadow with non-ctx type - string
//
// Shadow with non-ctx type (string)
func badShadowingNonContextWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		ctx := "not a context"
		_ = ctx
	})
	wg.Wait()
}

// ===== Uses ctx before shadow =====

// [GOOD]: Uses ctx before shadow
//
// Uses ctx before shadow - valid usage
func goodUsesCtxBeforeShadowingGoroutine(ctx context.Context) {
	go func() {
		_ = ctx.Done() // use ctx before shadowing
		ctx := "shadow"
		_ = ctx
	}()
}

// [GOOD]: Uses ctx before shadow
//
// Uses ctx before shadow - valid usage
func goodUsesCtxBeforeShadowingErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		_ = ctx.Done() // use ctx before shadowing
		ctx := "shadow"
		_ = ctx
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Uses ctx before shadow
//
// Uses ctx before shadow - valid usage
func goodUsesCtxBeforeShadowingWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = ctx.Done() // use ctx before shadowing
		ctx := "shadow"
		_ = ctx
	})
	wg.Wait()
}

// ===== Inner func has own ctx param =====

// [GOOD]: Inner func has own ctx param
//
// Inner function declares its own context parameter and uses it.
func goodShadowingInnerCtxParam(outerCtx context.Context) {
	go func(ctx context.Context) {
		_ = ctx.Done() // uses inner ctx - OK
	}(outerCtx)
}

// [GOOD]: Inner func has own ctx param
//
// Inner function declares its own context parameter and uses it.
func goodNestedInnerHasOwnCtxErrgroup(outerCtx context.Context) {
	innerFunc := func(ctx context.Context) {
		g := new(errgroup.Group)
		g.Go(func() error {
			_ = ctx // uses inner ctx
			return nil
		})
		_ = g.Wait()
	}
	innerFunc(outerCtx)
}

// [GOOD]: Inner func has own ctx param
//
// Inner function declares its own context parameter and uses it.
func goodNestedInnerHasOwnCtxWaitGroup(outerCtx context.Context) {
	innerFunc := func(ctx context.Context) {
		var wg sync.WaitGroup
		wg.Go(func() {
			_ = ctx // uses inner ctx
		})
		wg.Wait()
	}
	innerFunc(outerCtx)
}

// ===== Shadowing - inner ctx shadows outer =====

// [BAD]: Shadowing - inner ctx shadows outer
//
// Inner function's context parameter shadows the outer one.
func evilShadowingInnerHasCtxErrgroup(outerCtx context.Context) {
	innerFunc := func(ctx context.Context) {
		g := new(errgroup.Group)
		g.Go(func() error {
			_ = ctx // uses inner ctx
			return nil
		})
		_ = g.Wait()
	}
	innerFunc(outerCtx)
}

// [BAD]: Shadowing - inner ctx shadows outer
//
// Inner function's context parameter shadows the outer one.
func evilShadowingInnerHasCtxWaitGroup(outerCtx context.Context) {
	innerFunc := func(ctx context.Context) {
		var wg sync.WaitGroup
		wg.Go(func() {
			_ = ctx // uses inner ctx
		})
		wg.Wait()
	}
	innerFunc(outerCtx)
}

// ===== Shadowing - inner ignores ctx =====

// [BAD]: Shadowing - inner ignores ctx
//
// Inner function ignores the shadowed context.
func evilShadowingInnerIgnoresCtxErrgroup(outerCtx context.Context) {
	innerFunc := func(ctx context.Context) {
		g := new(errgroup.Group)
		g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
			return nil
		})
		_ = g.Wait()
	}
	innerFunc(outerCtx)
}

// [BAD]: Shadowing - inner ignores ctx
//
// Inner function ignores the shadowed context.
func evilShadowingInnerIgnoresCtxWaitGroup(outerCtx context.Context) {
	innerFunc := func(ctx context.Context) {
		var wg sync.WaitGroup
		wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		})
		wg.Wait()
	}
	innerFunc(outerCtx)
}

// ===== Two levels of shadowing =====

// [BAD]: Two levels of shadowing
//
// Context is shadowed through two levels of function nesting.
func evilShadowingTwoLevelsErrgroup(ctx1 context.Context) {
	func(ctx2 context.Context) {
		func(ctx3 context.Context) {
			g := new(errgroup.Group)
			g.Go(func() error {
				_ = ctx3 // uses ctx3
				return nil
			})
			_ = g.Wait()
		}(ctx2)
	}(ctx1)
}

// [BAD]: Two levels of shadowing
//
// Context is shadowed through two levels of function nesting.
func evilShadowingTwoLevelsWaitGroup(ctx1 context.Context) {
	func(ctx2 context.Context) {
		func(ctx3 context.Context) {
			var wg sync.WaitGroup
			wg.Go(func() {
				_ = ctx3 // uses ctx3
			})
			wg.Wait()
		}(ctx2)
	}(ctx1)
}

// [BAD]: Two levels of shadowing
//
// Context is shadowed through two levels of function nesting.
func evilShadowingTwoLevelsBadErrgroup(ctx1 context.Context) {
	func(ctx2 context.Context) {
		func(ctx3 context.Context) {
			g := new(errgroup.Group)
			g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx3"`
				return nil
			})
			_ = g.Wait()
		}(ctx2)
	}(ctx1)
}

// [BAD]: Two levels of shadowing
//
// Context is shadowed through two levels of function nesting.
func evilShadowingTwoLevelsBadWaitGroup(ctx1 context.Context) {
	func(ctx2 context.Context) {
		func(ctx3 context.Context) {
			var wg sync.WaitGroup
			wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx3"`
			})
			wg.Wait()
		}(ctx2)
	}(ctx1)
}
