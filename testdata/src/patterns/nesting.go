// Spawns inside inner functions, IIFEs, and layers that add their own ctx.

package patterns

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ===== Go call inside an inner function =====

// [BAD]: Go call inside inner named func without ctx
//
// Go call in nested function does not use the outer context.
func badNestedInnerFuncErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	innerFunc := func() {
		g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
			return nil
		})
	}
	innerFunc()
	_ = g.Wait()
}

// [BAD]: Go call inside inner named func without ctx
//
// Go call in nested function does not use the outer context.
func badNestedInnerFuncWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	innerFunc := func() {
		wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		})
	}
	innerFunc()
	wg.Wait()
}

// [BAD]: Go call inside IIFE without ctx
//
// Go call inside immediately invoked function expression without context.
func badNestedClosureErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	func() {
		g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
			return nil
		})
	}()
	_ = g.Wait()
}

// [BAD]: Go call inside IIFE without ctx
//
// Go call inside immediately invoked function expression without context.
func badNestedClosureWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	func() {
		wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		})
	}()
	wg.Wait()
}

// [BAD]: Go call inside deeply nested IIFE without ctx
//
// Go call inside immediately invoked function expression without context.
func badNestedDeepErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	func() {
		func() {
			g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
				return nil
			})
		}()
	}()
	_ = g.Wait()
}

// [BAD]: Go call inside deeply nested IIFE without ctx
//
// Go call inside immediately invoked function expression without context.
func badNestedDeepWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	func() {
		func() {
			wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
			})
		}()
	}()
	wg.Wait()
}

// [GOOD]: Go call inside inner func with ctx
//
// Nested function properly captures and uses the outer context.
func goodNestedWithCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	innerFunc := func() {
		g.Go(func() error {
			_ = ctx
			return nil
		})
	}
	innerFunc()
	_ = g.Wait()
}

// [GOOD]: Go call inside inner func with ctx
//
// Nested function properly captures and uses the outer context.
func goodNestedWithCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	innerFunc := func() {
		wg.Go(func() {
			_ = ctx
		})
	}
	innerFunc()
	wg.Wait()
}

// ===== Middle layer introduces ctx =====

// [GOOD]: Middle layer introduces ctx - goroutine uses it
//
// Middle layer introduces context that inner goroutine uses.
func goodMiddleLayerIntroducesCtxUsed() {
	func(ctx context.Context) {
		go func() {
			_ = ctx
		}()
	}(context.Background())
}

// [GOOD]: Middle layer introduces ctx - goroutine uses it
//
// Middle layer introduces context that inner goroutine uses.
func evilMiddleLayerIntroducesCtxErrgroup() {
	func(ctx context.Context) {
		g := new(errgroup.Group)
		g.Go(func() error {
			_ = ctx
			return nil
		})
		func() {
			g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
				return nil
			})
		}()
		_ = g.Wait()
	}(context.Background())
}

// [GOOD]: Middle layer introduces ctx - goroutine uses it
//
// Middle layer introduces context that inner goroutine uses.
func evilMiddleLayerIntroducesCtxWaitGroup() {
	func(ctx context.Context) {
		var wg sync.WaitGroup
		wg.Go(func() {
			_ = ctx
		})
		func() {
			wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
			})
		}()
		wg.Wait()
	}(context.Background())
}

// [GOOD]: Middle layer introduces ctx
//
// Middle layer introduces context that inner goroutine uses.
func evilMiddleLayerIntroducesCtxGoodErrgroup() {
	func(ctx context.Context) {
		g := new(errgroup.Group)
		func() {
			g.Go(func() error {
				_ = ctx
				return nil
			})
		}()
		_ = g.Wait()
	}(context.Background())
}

// [GOOD]: Middle layer introduces ctx
//
// Middle layer introduces context that inner goroutine uses.
func evilMiddleLayerIntroducesCtxGoodWaitGroup() {
	func(ctx context.Context) {
		var wg sync.WaitGroup
		func() {
			wg.Go(func() {
				_ = ctx
			})
		}()
		wg.Wait()
	}(context.Background())
}

// ===== Interleaved layers =====

// [BAD]: Interleaved layers
//
// Nested function layers where goroutine ignores available context.
func evilInterleavedLayersErrgroup(outerCtx context.Context) {
	func() {
		func(middleCtx context.Context) {
			g := new(errgroup.Group)
			func() {
				g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "middleCtx"`
					return nil
				})
			}()
			_ = g.Wait()
		}(outerCtx)
	}()
}

// [BAD]: Interleaved layers
//
// Nested function layers where goroutine ignores available context.
func evilInterleavedLayersWaitGroup(outerCtx context.Context) {
	func() {
		func(middleCtx context.Context) {
			var wg sync.WaitGroup
			func() {
				wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "middleCtx"`
				})
			}()
			wg.Wait()
		}(outerCtx)
	}()
}

// [GOOD]: Interleaved layers
//
// Nested function layers with context shadowing handled correctly.
func evilInterleavedLayersGoodErrgroup(outerCtx context.Context) {
	func() {
		func(middleCtx context.Context) {
			g := new(errgroup.Group)
			func() {
				g.Go(func() error {
					_ = middleCtx
					return nil
				})
			}()
			_ = g.Wait()
		}(outerCtx)
	}()
}

// [GOOD]: Interleaved layers
//
// Nested function layers with context shadowing handled correctly.
func evilInterleavedLayersGoodWaitGroup(outerCtx context.Context) {
	func() {
		func(middleCtx context.Context) {
			var wg sync.WaitGroup
			func() {
				wg.Go(func() {
					_ = middleCtx
				})
			}()
			wg.Wait()
		}(outerCtx)
	}()
}
