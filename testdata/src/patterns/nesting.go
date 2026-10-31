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

// ===== Context through a captured closure or struct =====

// [GOOD]: Context through a captured closure
//
// The closure it calls uses ctx, so it carries ctx (#149).
func goodCapturedClosureUsesCtxGoroutine(ctx context.Context) {
	work := func() { _ = ctx }
	go func() { work() }()
}

// [GOOD]: Context through a captured struct field
//
// The field it reads holds ctx (#149).
func goodCapturedStructCtxFieldGoroutine(ctx context.Context) {
	h := &ctxHolder{ctx: ctx}
	go func() { _ = h.ctx }()
}

// [BAD]: Captured struct without reading its context
//
// Capturing a struct that holds ctx is not using it.
func badCapturedStructOtherFieldGoroutine(ctx context.Context) {
	h := &ctxHolder{ctx: ctx, name: "x"}
	go func() { _ = h.name }() // want `goroutine does not propagate context "ctx"`
}

// [GOOD]: Context through a captured closure
//
// The closure it calls uses ctx, so it carries ctx (#149).
func goodCapturedClosureUsesCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	work := func() { _ = ctx }
	g.Go(func() error { work(); return nil })
	_ = g.Wait()
}

// [GOOD]: Context through a captured struct field
//
// The field it reads holds ctx (#149).
func goodCapturedStructCtxFieldErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	h := &ctxHolder{ctx: ctx}
	g.Go(func() error { _ = h.ctx; return nil })
	_ = g.Wait()
}

// [BAD]: Captured struct without reading its context
//
// Capturing a struct that holds ctx is not using it.
func badCapturedStructOtherFieldErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	h := &ctxHolder{ctx: ctx, name: "x"}
	g.Go(func() error { _ = h.name; return nil }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [GOOD]: Context through a captured closure
//
// The closure it calls uses ctx, so it carries ctx (#149).
func goodCapturedClosureUsesCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	work := func() { _ = ctx }
	wg.Go(func() { work() })
	wg.Wait()
}

// [GOOD]: Context through a captured struct field
//
// The field it reads holds ctx (#149).
func goodCapturedStructCtxFieldWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	h := &ctxHolder{ctx: ctx}
	wg.Go(func() { _ = h.ctx })
	wg.Wait()
}

// [BAD]: Captured struct without reading its context
//
// Capturing a struct that holds ctx is not using it.
func badCapturedStructOtherFieldWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	h := &ctxHolder{ctx: ctx, name: "x"}
	wg.Go(func() { _ = h.name }) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [GOOD]: Context through a captured closure
//
// The closure it calls uses ctx, so it carries ctx (#149).
func goodCapturedClosureUsesCtxSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	work := func() { _ = ctx }
	runWithGroup(g, func() error { work(); return nil })
	_ = g.Wait()
}

// [GOOD]: Context through a captured struct field
//
// The field it reads holds ctx (#149).
func goodCapturedStructCtxFieldSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	h := &ctxHolder{ctx: ctx}
	runWithGroup(g, func() error { _ = h.ctx; return nil })
	_ = g.Wait()
}

// [BAD]: Captured struct without reading its context
//
// Capturing a struct that holds ctx is not using it.
func badCapturedStructOtherFieldSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	h := &ctxHolder{ctx: ctx, name: "x"}
	runWithGroup(g, func() error { _ = h.name; return nil }) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== Captured struct field with only a fresh context =====

// [BAD]: Captured struct field with only a fresh context
//
// A fresh context does not count when it is read from a field either (#182).
func badCapturedStructFreshCtxFieldGoroutine(ctx context.Context) {
	h := &ctxHolder{ctx: context.Background()}
	go func() { use(h.ctx) }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Captured struct field with only a fresh context
//
// A fresh context does not count when it is read from a field either (#182).
func badCapturedStructFreshCtxFieldErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	h := &ctxHolder{ctx: context.Background()}
	g.Go(func() error { use(h.ctx); return nil }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Captured struct field with only a fresh context
//
// A fresh context does not count when it is read from a field either (#182).
func badCapturedStructFreshCtxFieldWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	h := &ctxHolder{ctx: context.Background()}
	wg.Go(func() { use(h.ctx) }) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Captured struct field with only a fresh context
//
// A fresh context does not count when it is read from a field either (#182).
func badCapturedStructFreshCtxFieldSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	h := &ctxHolder{ctx: context.Background()}
	runWithGroup(g, func() error { use(h.ctx); return nil }) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== Captured slice element with only a fresh context =====

// [BAD]: Captured slice element with only a fresh context
//
// A fresh context does not count when it is read from an element either (#182).
func badCapturedSliceFreshCtxElemGoroutine(ctx context.Context) {
	cs := []context.Context{context.TODO()}
	go func() { use(cs[0]) }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Captured slice element with only a fresh context
//
// A fresh context does not count when it is read from an element either (#182).
func badCapturedSliceFreshCtxElemErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	cs := []context.Context{context.TODO()}
	g.Go(func() error { use(cs[0]); return nil }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Captured slice element with only a fresh context
//
// A fresh context does not count when it is read from an element either (#182).
func badCapturedSliceFreshCtxElemWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	cs := []context.Context{context.TODO()}
	wg.Go(func() { use(cs[0]) }) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Captured slice element with only a fresh context
//
// A fresh context does not count when it is read from an element either (#182).
func badCapturedSliceFreshCtxElemSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	cs := []context.Context{context.TODO()}
	runWithGroup(g, func() error { use(cs[0]); return nil }) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== Context through a copy of a captured struct =====

// [GOOD]: Context through a copy of a captured struct
//
// The copy holds ctx, as the struct it is copied from does.
func goodCapturedStructCopyCtxFieldGoroutine(ctx context.Context) {
	h := &ctxHolder{ctx: ctx}
	go func() { v := *h; use(v.ctx) }()
}

// [GOOD]: Context through a copy of a captured struct
//
// The copy holds ctx, as the struct it is copied from does.
func goodCapturedStructCopyCtxFieldErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	h := &ctxHolder{ctx: ctx}
	g.Go(func() error { v := *h; use(v.ctx); return nil })
	_ = g.Wait()
}

// [GOOD]: Context through a copy of a captured struct
//
// The copy holds ctx, as the struct it is copied from does.
func goodCapturedStructCopyCtxFieldWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	h := &ctxHolder{ctx: ctx}
	wg.Go(func() { v := *h; use(v.ctx) })
	wg.Wait()
}

// [GOOD]: Context through a copy of a captured struct
//
// The copy holds ctx, as the struct it is copied from does.
func goodCapturedStructCopyCtxFieldSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	h := &ctxHolder{ctx: ctx}
	runWithGroup(g, func() error { v := *h; use(v.ctx); return nil })
	_ = g.Wait()
}

// ===== Method of a captured struct that holds a context =====

// [GOOD]: Method of a captured struct that holds a context
//
// The receiver may hold a context, as for a method spawned directly (#184).
func goodCapturedStructCtxMethodGoroutine(ctx context.Context) {
	s := &ctxService{ctx: ctx}
	go func() { s.run() }()
}

// [GOOD]: Method of a captured struct that holds a context
//
// The receiver may hold a context, as for a method spawned directly (#184).
func goodCapturedStructCtxMethodErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	s := &ctxService{ctx: ctx}
	g.Go(func() error { s.run(); return nil })
	_ = g.Wait()
}

// [GOOD]: Method of a captured struct that holds a context
//
// The receiver may hold a context, as for a method spawned directly (#184).
func goodCapturedStructCtxMethodWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	s := &ctxService{ctx: ctx}
	wg.Go(func() { s.run() })
	wg.Wait()
}

// [GOOD]: Method of a captured struct that holds a context
//
// The receiver may hold a context, as for a method spawned directly (#184).
func goodCapturedStructCtxMethodSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	s := &ctxService{ctx: ctx}
	runWithGroup(g, func() error { s.run(); return nil })
	_ = g.Wait()
}

// ===== Method of a captured struct that holds only a fresh context =====

// [NOTCHECKED]: Method of a captured struct that holds only a fresh context
//
// The method is not looked into, so it passes, as `go s.run()` does.
func notCheckedCapturedStructFreshCtxMethodGoroutine(ctx context.Context) {
	s := &ctxService{ctx: context.Background()}
	go func() { s.run() }()
}

// [NOTCHECKED]: Method of a captured struct that holds only a fresh context
//
// The method is not looked into, so it passes, as `go s.run()` does.
func notCheckedCapturedStructFreshCtxMethodErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	s := &ctxService{ctx: context.Background()}
	g.Go(func() error { s.run(); return nil })
	_ = g.Wait()
}

// [NOTCHECKED]: Method of a captured struct that holds only a fresh context
//
// The method is not looked into, so it passes, as `go s.run()` does.
func notCheckedCapturedStructFreshCtxMethodWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	s := &ctxService{ctx: context.Background()}
	wg.Go(func() { s.run() })
	wg.Wait()
}

// [NOTCHECKED]: Method of a captured struct that holds only a fresh context
//
// The method is not looked into, so it passes, as `go s.run()` does.
func notCheckedCapturedStructFreshCtxMethodSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	s := &ctxService{ctx: context.Background()}
	runWithGroup(g, func() error { s.run(); return nil })
	_ = g.Wait()
}

// ===== Captured value that implements context.Context =====

// [GOOD]: Captured value that implements context.Context
//
// A struct that embeds context.Context is a context itself (#184).
func goodCapturedContextImplGoroutine(ctx context.Context) {
	c := ctxWrapper{Context: ctx, user: "u"}
	go func() { use(c) }()
}

// [GOOD]: Captured value that implements context.Context
//
// A struct that embeds context.Context is a context itself (#184).
func goodCapturedContextImplErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	c := ctxWrapper{Context: ctx, user: "u"}
	g.Go(func() error { use(c); return nil })
	_ = g.Wait()
}

// [GOOD]: Captured value that implements context.Context
//
// A struct that embeds context.Context is a context itself (#184).
func goodCapturedContextImplWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	c := ctxWrapper{Context: ctx, user: "u"}
	wg.Go(func() { use(c) })
	wg.Wait()
}

// [GOOD]: Captured value that implements context.Context
//
// A struct that embeds context.Context is a context itself (#184).
func goodCapturedContextImplSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	c := ctxWrapper{Context: ctx, user: "u"}
	runWithGroup(g, func() error { use(c); return nil })
	_ = g.Wait()
}

// ===== Captured context.Context implementation made from a fresh context =====

// [LIMITATION]: Captured context.Context implementation made from a fresh context
//
// A value whose type implements context.Context counts without looking at
// what it is made from, so it passes. It should be reported.
func limitationCapturedContextImplFreshGoroutine(ctx context.Context) {
	c := ctxWrapper{Context: context.Background(), user: "u"}
	go func() { use(c) }()
}

// [LIMITATION]: Captured context.Context implementation made from a fresh context
//
// A value whose type implements context.Context counts without looking at
// what it is made from, so it passes. It should be reported.
func limitationCapturedContextImplFreshErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	c := ctxWrapper{Context: context.Background(), user: "u"}
	g.Go(func() error { use(c); return nil })
	_ = g.Wait()
}

// [LIMITATION]: Captured context.Context implementation made from a fresh context
//
// A value whose type implements context.Context counts without looking at
// what it is made from, so it passes. It should be reported.
func limitationCapturedContextImplFreshWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	c := ctxWrapper{Context: context.Background(), user: "u"}
	wg.Go(func() { use(c) })
	wg.Wait()
}

// [LIMITATION]: Captured context.Context implementation made from a fresh context
//
// A value whose type implements context.Context counts without looking at
// what it is made from, so it passes. It should be reported.
func limitationCapturedContextImplFreshSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	c := ctxWrapper{Context: context.Background(), user: "u"}
	runWithGroup(g, func() error { use(c); return nil })
	_ = g.Wait()
}

// ctxHolder holds a context in a field.
type ctxHolder struct {
	ctx  context.Context
	name string
}

// ctxService holds a context that its method uses.
type ctxService struct{ ctx context.Context }

func (s *ctxService) run() { use(s.ctx) }

// ctxWrapper is a context with a value of its own.
type ctxWrapper struct {
	context.Context
	user string
}
