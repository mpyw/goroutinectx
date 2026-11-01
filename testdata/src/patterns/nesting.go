// Spawns inside inner functions, IIFEs, and layers that add their own ctx.

package patterns

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"golang.org/x/sync/errgroup"

	"github.com/example/ctxholder"
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

// ===== Context from an element of a captured map =====

// [GOOD]: Context from an element of a captured map
//
// A context read from a map counts, as one read from a slice does (#253).
func goodCapturedMapCtxElemGoroutine(ctx context.Context, k string) {
	m := map[string]context.Context{"a": ctx}
	go func() { use(m[k]) }()
}

// [GOOD]: Context from an element of a captured map
//
// A context read from a map counts, as one read from a slice does (#253).
func goodCapturedMapCtxElemErrgroup(ctx context.Context, k string) {
	g := new(errgroup.Group)
	m := map[string]context.Context{"a": ctx}
	g.Go(func() error { use(m[k]); return nil })
	_ = g.Wait()
}

// [GOOD]: Context from an element of a captured map
//
// A context read from a map counts, as one read from a slice does (#253).
func goodCapturedMapCtxElemWaitGroup(ctx context.Context, k string) {
	var wg sync.WaitGroup
	m := map[string]context.Context{"a": ctx}
	wg.Go(func() { use(m[k]) })
	wg.Wait()
}

// [GOOD]: Context from an element of a captured map
//
// A context read from a map counts, as one read from a slice does (#253).
func goodCapturedMapCtxElemSpawner(ctx context.Context, k string) {
	g := new(errgroup.Group)
	m := map[string]context.Context{"a": ctx}
	runWithGroup(g, func() error { use(m[k]); return nil })
	_ = g.Wait()
}

// ===== Context from a range over a captured map =====

// [GOOD]: Context from a range over a captured map
//
// A context a range over a map gives counts (#253).
func goodCapturedMapCtxRangeGoroutine(ctx context.Context) {
	m := map[string]context.Context{"a": ctx}
	go func() {
		for _, c := range m {
			use(c)
		}
	}()
}

// [GOOD]: Context from a range over a captured map
//
// A context a range over a map gives counts (#253).
func goodCapturedMapCtxRangeErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	m := map[string]context.Context{"a": ctx}
	g.Go(func() error {
		for _, c := range m {
			use(c)
		}
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Context from a range over a captured map
//
// A context a range over a map gives counts (#253).
func goodCapturedMapCtxRangeWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	m := map[string]context.Context{"a": ctx}
	wg.Go(func() {
		for _, c := range m {
			use(c)
		}
	})
	wg.Wait()
}

// [GOOD]: Context from a range over a captured map
//
// A context a range over a map gives counts (#253).
func goodCapturedMapCtxRangeSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	m := map[string]context.Context{"a": ctx}
	runWithGroup(g, func() error {
		for _, c := range m {
			use(c)
		}
		return nil
	})
	_ = g.Wait()
}

// ===== Context from a comma-ok lookup of a captured map =====

// [GOOD]: Context from a comma-ok lookup of a captured map
//
// A context v, ok := m[k] gives counts (#253).
func goodCapturedMapCtxCommaOkGoroutine(ctx context.Context, k string) {
	m := map[string]context.Context{"a": ctx}
	go func() {
		if c, ok := m[k]; ok {
			use(c)
		}
	}()
}

// [GOOD]: Context from a comma-ok lookup of a captured map
//
// A context v, ok := m[k] gives counts (#253).
func goodCapturedMapCtxCommaOkErrgroup(ctx context.Context, k string) {
	g := new(errgroup.Group)
	m := map[string]context.Context{"a": ctx}
	g.Go(func() error {
		if c, ok := m[k]; ok {
			use(c)
		}
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Context from a comma-ok lookup of a captured map
//
// A context v, ok := m[k] gives counts (#253).
func goodCapturedMapCtxCommaOkWaitGroup(ctx context.Context, k string) {
	var wg sync.WaitGroup
	m := map[string]context.Context{"a": ctx}
	wg.Go(func() {
		if c, ok := m[k]; ok {
			use(c)
		}
	})
	wg.Wait()
}

// [GOOD]: Context from a comma-ok lookup of a captured map
//
// A context v, ok := m[k] gives counts (#253).
func goodCapturedMapCtxCommaOkSpawner(ctx context.Context, k string) {
	g := new(errgroup.Group)
	m := map[string]context.Context{"a": ctx}
	runWithGroup(g, func() error {
		if c, ok := m[k]; ok {
			use(c)
		}
		return nil
	})
	_ = g.Wait()
}

// ===== Context from a map in a field of a captured struct =====

// [GOOD]: Context from a map in a field of a captured struct
//
// A context read from a map that a field holds counts (#253).
func goodCapturedFieldMapCtxGoroutine(ctx context.Context, id int) {
	r := &ctxByID{byID: map[int]context.Context{1: ctx}}
	go func() { use(r.byID[id]) }()
}

// [GOOD]: Context from a map in a field of a captured struct
//
// A context read from a map that a field holds counts (#253).
func goodCapturedFieldMapCtxErrgroup(ctx context.Context, id int) {
	g := new(errgroup.Group)
	r := &ctxByID{byID: map[int]context.Context{1: ctx}}
	g.Go(func() error { use(r.byID[id]); return nil })
	_ = g.Wait()
}

// [GOOD]: Context from a map in a field of a captured struct
//
// A context read from a map that a field holds counts (#253).
func goodCapturedFieldMapCtxWaitGroup(ctx context.Context, id int) {
	var wg sync.WaitGroup
	r := &ctxByID{byID: map[int]context.Context{1: ctx}}
	wg.Go(func() { use(r.byID[id]) })
	wg.Wait()
}

// [GOOD]: Context from a map in a field of a captured struct
//
// A context read from a map that a field holds counts (#253).
func goodCapturedFieldMapCtxSpawner(ctx context.Context, id int) {
	g := new(errgroup.Group)
	r := &ctxByID{byID: map[int]context.Context{1: ctx}}
	runWithGroup(g, func() error { use(r.byID[id]); return nil })
	_ = g.Wait()
}

// ===== Context received from a captured channel =====

// [GOOD]: Context received from a captured channel
//
// A context received from a channel counts (#253).
func goodCapturedChanCtxRecvGoroutine(ctx context.Context) {
	ch := make(chan context.Context, 1)
	ch <- ctx
	go func() { use(<-ch) }()
}

// [GOOD]: Context received from a captured channel
//
// A context received from a channel counts (#253).
func goodCapturedChanCtxRecvErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan context.Context, 1)
	ch <- ctx
	g.Go(func() error { use(<-ch); return nil })
	_ = g.Wait()
}

// [GOOD]: Context received from a captured channel
//
// A context received from a channel counts (#253).
func goodCapturedChanCtxRecvWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	ch := make(chan context.Context, 1)
	ch <- ctx
	wg.Go(func() { use(<-ch) })
	wg.Wait()
}

// [GOOD]: Context received from a captured channel
//
// A context received from a channel counts (#253).
func goodCapturedChanCtxRecvSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan context.Context, 1)
	ch <- ctx
	runWithGroup(g, func() error { use(<-ch); return nil })
	_ = g.Wait()
}

// ===== Context from a range over a captured channel =====

// [GOOD]: Context from a range over a captured channel
//
// A context a range over a channel gives counts (#253).
func goodCapturedChanCtxRangeGoroutine(ctx context.Context) {
	ch := make(chan context.Context, 1)
	ch <- ctx
	close(ch)
	go func() {
		for c := range ch {
			use(c)
		}
	}()
}

// [GOOD]: Context from a range over a captured channel
//
// A context a range over a channel gives counts (#253).
func goodCapturedChanCtxRangeErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan context.Context, 1)
	ch <- ctx
	close(ch)
	g.Go(func() error {
		for c := range ch {
			use(c)
		}
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Context from a range over a captured channel
//
// A context a range over a channel gives counts (#253).
func goodCapturedChanCtxRangeWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	ch := make(chan context.Context, 1)
	ch <- ctx
	close(ch)
	wg.Go(func() {
		for c := range ch {
			use(c)
		}
	})
	wg.Wait()
}

// [GOOD]: Context from a range over a captured channel
//
// A context a range over a channel gives counts (#253).
func goodCapturedChanCtxRangeSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan context.Context, 1)
	ch <- ctx
	close(ch)
	runWithGroup(g, func() error {
		for c := range ch {
			use(c)
		}
		return nil
	})
	_ = g.Wait()
}

// ===== Context from a type assertion of a captured value =====

// [GOOD]: Context from a type assertion of a captured value
//
// A context a type assertion gives counts (#253).
func goodCapturedAnyCtxAssertGoroutine(ctx context.Context) {
	var v any = ctx
	go func() { use(v.(context.Context)) }()
}

// [GOOD]: Context from a type assertion of a captured value
//
// A context a type assertion gives counts (#253).
func goodCapturedAnyCtxAssertErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	var v any = ctx
	g.Go(func() error { use(v.(context.Context)); return nil })
	_ = g.Wait()
}

// [GOOD]: Context from a type assertion of a captured value
//
// A context a type assertion gives counts (#253).
func goodCapturedAnyCtxAssertWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	var v any = ctx
	wg.Go(func() { use(v.(context.Context)) })
	wg.Wait()
}

// [GOOD]: Context from a type assertion of a captured value
//
// A context a type assertion gives counts (#253).
func goodCapturedAnyCtxAssertSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	var v any = ctx
	runWithGroup(g, func() error { use(v.(context.Context)); return nil })
	_ = g.Wait()
}

// ===== Context from a comma-ok type assertion of a captured value =====

// [GOOD]: Context from a comma-ok type assertion of a captured value
//
// A context v, ok := x.(T) gives counts (#253).
func goodCapturedAnyCtxAssertCommaOkGoroutine(ctx context.Context) {
	var v any = ctx
	go func() {
		if c, ok := v.(context.Context); ok {
			use(c)
		}
	}()
}

// [GOOD]: Context from a comma-ok type assertion of a captured value
//
// A context v, ok := x.(T) gives counts (#253).
func goodCapturedAnyCtxAssertCommaOkErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	var v any = ctx
	g.Go(func() error {
		if c, ok := v.(context.Context); ok {
			use(c)
		}
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Context from a comma-ok type assertion of a captured value
//
// A context v, ok := x.(T) gives counts (#253).
func goodCapturedAnyCtxAssertCommaOkWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	var v any = ctx
	wg.Go(func() {
		if c, ok := v.(context.Context); ok {
			use(c)
		}
	})
	wg.Wait()
}

// [GOOD]: Context from a comma-ok type assertion of a captured value
//
// A context v, ok := x.(T) gives counts (#253).
func goodCapturedAnyCtxAssertCommaOkSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	var v any = ctx
	runWithGroup(g, func() error {
		if c, ok := v.(context.Context); ok {
			use(c)
		}
		return nil
	})
	_ = g.Wait()
}

// ===== Captured map of values that are not contexts =====

// [BAD]: Captured map of values that are not contexts
//
// A map that holds no context gives none.
func badCapturedMapNoCtxGoroutine(ctx context.Context, k string) {
	m := map[string]string{"a": "b"}
	go func() { _ = m[k] }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Captured map of values that are not contexts
//
// A map that holds no context gives none.
func badCapturedMapNoCtxErrgroup(ctx context.Context, k string) {
	g := new(errgroup.Group)
	m := map[string]string{"a": "b"}
	g.Go(func() error { _ = m[k]; return nil }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Captured map of values that are not contexts
//
// A map that holds no context gives none.
func badCapturedMapNoCtxWaitGroup(ctx context.Context, k string) {
	var wg sync.WaitGroup
	m := map[string]string{"a": "b"}
	wg.Go(func() { _ = m[k] }) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Captured map of values that are not contexts
//
// A map that holds no context gives none.
func badCapturedMapNoCtxSpawner(ctx context.Context, k string) {
	g := new(errgroup.Group)
	m := map[string]string{"a": "b"}
	runWithGroup(g, func() error { _ = m[k]; return nil }) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== Captured channel of values that are not contexts =====

// [BAD]: Captured channel of values that are not contexts
//
// A channel that carries no context gives none.
func badCapturedChanNoCtxGoroutine(ctx context.Context) {
	ch := make(chan int, 1)
	ch <- 1
	close(ch)
	go func() { // want `goroutine does not propagate context "ctx"`
		for n := range ch {
			_ = n
		}
	}()
}

// [BAD]: Captured channel of values that are not contexts
//
// A channel that carries no context gives none.
func badCapturedChanNoCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan int, 1)
	ch <- 1
	close(ch)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		for n := range ch {
			_ = n
		}
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Captured channel of values that are not contexts
//
// A channel that carries no context gives none.
func badCapturedChanNoCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	ch := make(chan int, 1)
	ch <- 1
	close(ch)
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		for n := range ch {
			_ = n
		}
	})
	wg.Wait()
}

// [BAD]: Captured channel of values that are not contexts
//
// A channel that carries no context gives none.
func badCapturedChanNoCtxSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan int, 1)
	ch <- 1
	close(ch)
	runWithGroup(g, func() error { // want `runWithGroup\(\) func argument should use context "ctx"`
		for n := range ch {
			_ = n
		}
		return nil
	})
	_ = g.Wait()
}

// ===== Type assertion of a captured value to a type that is not a context =====

// [BAD]: Type assertion of a captured value to a type that is not a context
//
// A value asserted to a type that holds no context gives none.
func badCapturedAnyAssertNoCtxGoroutine(ctx context.Context) {
	var v any = "s"
	go func() { _ = v.(string) }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Type assertion of a captured value to a type that is not a context
//
// A value asserted to a type that holds no context gives none.
func badCapturedAnyAssertNoCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	var v any = "s"
	g.Go(func() error { _ = v.(string); return nil }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Type assertion of a captured value to a type that is not a context
//
// A value asserted to a type that holds no context gives none.
func badCapturedAnyAssertNoCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	var v any = "s"
	wg.Go(func() { _ = v.(string) }) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Type assertion of a captured value to a type that is not a context
//
// A value asserted to a type that holds no context gives none.
func badCapturedAnyAssertNoCtxSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	var v any = "s"
	runWithGroup(g, func() error { _ = v.(string); return nil }) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== Captured map that holds only a fresh context =====

// [LIMITATION]: Captured map that holds only a fresh context
//
// What a captured map holds is not followed, so a context read from it
// counts even when it is only fresh, and it passes. It should be reported.
func limitationCapturedMapFreshCtxGoroutine(ctx context.Context, k string) {
	m := map[string]context.Context{"a": context.Background()}
	go func() { use(m[k]) }()
}

// [LIMITATION]: Captured map that holds only a fresh context
//
// What a captured map holds is not followed, so a context read from it
// counts even when it is only fresh, and it passes. It should be reported.
func limitationCapturedMapFreshCtxErrgroup(ctx context.Context, k string) {
	g := new(errgroup.Group)
	m := map[string]context.Context{"a": context.Background()}
	g.Go(func() error { use(m[k]); return nil })
	_ = g.Wait()
}

// [LIMITATION]: Captured map that holds only a fresh context
//
// What a captured map holds is not followed, so a context read from it
// counts even when it is only fresh, and it passes. It should be reported.
func limitationCapturedMapFreshCtxWaitGroup(ctx context.Context, k string) {
	var wg sync.WaitGroup
	m := map[string]context.Context{"a": context.Background()}
	wg.Go(func() { use(m[k]) })
	wg.Wait()
}

// [LIMITATION]: Captured map that holds only a fresh context
//
// What a captured map holds is not followed, so a context read from it
// counts even when it is only fresh, and it passes. It should be reported.
func limitationCapturedMapFreshCtxSpawner(ctx context.Context, k string) {
	g := new(errgroup.Group)
	m := map[string]context.Context{"a": context.Background()}
	runWithGroup(g, func() error { use(m[k]); return nil })
	_ = g.Wait()
}

// ctxByID holds contexts by an ID.
type ctxByID struct{ byID map[int]context.Context }

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

// ===== Captured struct that holds a context, given to a function =====

// [GOOD]: Captured struct that holds a context, given to a function
//
// The function may read the context, as for go serveCtxService(s) (#227).
func goodCapturedStructCtxArgGoroutine(ctx context.Context) {
	s := &ctxService{ctx: ctx}
	go func() { serveCtxService(s) }()
}

// [GOOD]: Captured struct that holds a context, given to a function
//
// The function may read the context, as for go serveCtxService(s) (#227).
func goodCapturedStructCtxArgErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	s := &ctxService{ctx: ctx}
	g.Go(func() error { serveCtxService(s); return nil })
	_ = g.Wait()
}

// [GOOD]: Captured struct that holds a context, given to a function
//
// The function may read the context, as for go serveCtxService(s) (#227).
func goodCapturedStructCtxArgWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	s := &ctxService{ctx: ctx}
	wg.Go(func() { serveCtxService(s) })
	wg.Wait()
}

// [GOOD]: Captured struct that holds a context, given to a function
//
// The function may read the context, as for go serveCtxService(s) (#227).
func goodCapturedStructCtxArgSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	s := &ctxService{ctx: ctx}
	runWithGroup(g, func() error { serveCtxService(s); return nil })
	_ = g.Wait()
}

// [GOOD]: Captured struct that holds a context, given to a method as a later argument
//
// The holder counts at any argument position, and as an interface too (#227).
func goodCapturedStructCtxLaterArgGoroutine(ctx context.Context, srv *ctxServer) {
	s := &ctxService{ctx: ctx}
	go func() { srv.handle("x", s) }()
}

// [GOOD]: Captured struct that holds a context, given as an interface
//
// The holder counts at any argument position, and as an interface too (#227).
func goodCapturedStructCtxInterfaceArgGoroutine(ctx context.Context) {
	s := &ctxService{ctx: ctx}
	go func() { runAny(s) }()
}

// ===== Captured *testing.T given to a function =====

// [BAD]: Captured *testing.T given to a function
//
// t holds the test's own context in an unexported field, not ctx, so giving
// it to a function does not count, as for a method of t (#191).
func badCapturedTestingTArgGoroutine(ctx context.Context, t *testing.T) {
	go func() { logTo(t) }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Captured *testing.T given to a function
//
// t holds the test's own context in an unexported field, not ctx, so giving
// it to a function does not count, as for a method of t (#191).
func badCapturedTestingTArgErrgroup(ctx context.Context, t *testing.T) {
	g := new(errgroup.Group)
	g.Go(func() error { logTo(t); return nil }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Captured *testing.T given to a function
//
// t holds the test's own context in an unexported field, not ctx, so giving
// it to a function does not count, as for a method of t (#191).
func badCapturedTestingTArgWaitGroup(ctx context.Context, t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() { logTo(t) }) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Captured *testing.T given to a function
//
// t holds the test's own context in an unexported field, not ctx, so giving
// it to a function does not count, as for a method of t (#191).
func badCapturedTestingTArgSpawner(ctx context.Context, t *testing.T) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error { logTo(t); return nil }) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Captured struct with no context given to a function
//
// A value that holds no context gives the function none.
func badCapturedStructNoCtxArgGoroutine(ctx context.Context) {
	h := &plainHolder{name: "x"}
	go func() { runAny(h) }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Captured struct that holds a context, given only to a builtin
//
// Appending the value to a slice does not read its context.
func badCapturedStructCtxBuiltinArgGoroutine(ctx context.Context) {
	s := &ctxService{ctx: ctx}
	go func() { _ = append([]*ctxService(nil), s) }() // want `goroutine does not propagate context "ctx"`
}

// ===== Method of a captured *testing.T =====

// [BAD]: Method of a captured *testing.T
//
// t holds the test's own context in an unexported field, not ctx, so a
// method of t does not count (#191).
func badCapturedTestingTMethodGoroutine(ctx context.Context, t *testing.T) {
	go func() { t.Log("x") }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Method of a captured *testing.T
//
// t holds the test's own context in an unexported field, not ctx, so a
// method of t does not count (#191).
func badCapturedTestingTMethodErrgroup(ctx context.Context, t *testing.T) {
	g := new(errgroup.Group)
	g.Go(func() error { t.Log("x"); return nil }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Method of a captured *testing.T
//
// t holds the test's own context in an unexported field, not ctx, so a
// method of t does not count (#191).
func badCapturedTestingTMethodWaitGroup(ctx context.Context, t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() { t.Log("x") }) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Method of a captured *testing.T
//
// t holds the test's own context in an unexported field, not ctx, so a
// method of t does not count (#191).
func badCapturedTestingTMethodSpawner(ctx context.Context, t *testing.T) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error { t.Log("x"); return nil }) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== Method of a captured value of another package with an unexported context, module unknown =====

// [NOTCHECKED]: Method of a captured value of another package with an unexported context, module unknown
//
// This test runs in GOPATH mode, so the module is not known. The package is
// not of the standard library and may be of the module, so the receiver
// counts and it passes. testdata/receivermodule reports it in module mode
// (#191).
func notCheckedCapturedHiddenCtxMethodGoroutine(ctx context.Context) {
	h := ctxholder.NewHidden()
	go func() { h.Do() }()
}

// [NOTCHECKED]: Method of a captured value of another package with an unexported context, module unknown
//
// This test runs in GOPATH mode, so the module is not known. The package is
// not of the standard library and may be of the module, so the receiver
// counts and it passes. testdata/receivermodule reports it in module mode
// (#191).
func notCheckedCapturedHiddenCtxMethodErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	h := ctxholder.NewHidden()
	g.Go(func() error { return h.Run() })
	_ = g.Wait()
}

// [NOTCHECKED]: Method of a captured value of another package with an unexported context, module unknown
//
// This test runs in GOPATH mode, so the module is not known. The package is
// not of the standard library and may be of the module, so the receiver
// counts and it passes. testdata/receivermodule reports it in module mode
// (#191).
func notCheckedCapturedHiddenCtxMethodWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	h := ctxholder.NewHidden()
	wg.Go(func() { h.Do() })
	wg.Wait()
}

// [NOTCHECKED]: Method of a captured value of another package with an unexported context, module unknown
//
// This test runs in GOPATH mode, so the module is not known. The package is
// not of the standard library and may be of the module, so the receiver
// counts and it passes. testdata/receivermodule reports it in module mode
// (#191).
func notCheckedCapturedHiddenCtxMethodSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	h := ctxholder.NewHidden()
	runWithGroup(g, func() error { return h.Run() })
	_ = g.Wait()
}

// ===== Method of a captured value of another package with an exported context =====

// [GOOD]: Method of a captured value of another package with an exported context
//
// The caller can set the context, as it does here.
func goodCapturedOpenCtxMethodGoroutine(ctx context.Context) {
	o := &ctxholder.Open{Ctx: ctx}
	go func() { _ = o.Run() }()
}

// [GOOD]: Method of a captured value of another package with an exported context
//
// The caller can set the context, as it does here.
func goodCapturedOpenCtxMethodErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	o := &ctxholder.Open{Ctx: ctx}
	g.Go(func() error { return o.Run() })
	_ = g.Wait()
}

// [GOOD]: Method of a captured value of another package with an exported context
//
// The caller can set the context, as it does here.
func goodCapturedOpenCtxMethodWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	o := &ctxholder.Open{Ctx: ctx}
	wg.Go(func() { _ = o.Run() })
	wg.Wait()
}

// [GOOD]: Method of a captured value of another package with an exported context
//
// The caller can set the context, as it does here.
func goodCapturedOpenCtxMethodSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	o := &ctxholder.Open{Ctx: ctx}
	runWithGroup(g, func() error { return o.Run() })
	_ = g.Wait()
}

// ===== Context from a method of a captured value =====

// [GOOD]: Context from a method of a captured value
//
// A method that returns a context, as req.Context() does, is a context read
// from the value.
func goodCapturedRequestContextGoroutine(ctx context.Context, req *http.Request) {
	go func() { use(req.Context()) }()
}

// [GOOD]: Context from a method of a captured value
//
// A method that returns a context, as req.Context() does, is a context read
// from the value.
func goodCapturedRequestContextErrgroup(ctx context.Context, req *http.Request) {
	g := new(errgroup.Group)
	g.Go(func() error { return req.Context().Err() })
	_ = g.Wait()
}

// [GOOD]: Context from a method of a captured value
//
// A method that returns a context, as req.Context() does, is a context read
// from the value.
func goodCapturedRequestContextWaitGroup(ctx context.Context, req *http.Request) {
	var wg sync.WaitGroup
	wg.Go(func() { use(req.Context()) })
	wg.Wait()
}

// [GOOD]: Context from a method of a captured value
//
// A method that returns a context, as req.Context() does, is a context read
// from the value.
func goodCapturedRequestContextSpawner(ctx context.Context, req *http.Request) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error { return req.Context().Err() })
	_ = g.Wait()
}

// ===== Method spawned directly on a value with an unexported context =====

// [NOTCHECKED]: Method spawned directly on a value with an unexported context
//
// A method spawned directly is not judged by its receiver, so it passes,
// whatever the receiver holds.
func notCheckedDirectHiddenCtxMethodGoroutine(ctx context.Context, t *testing.T) {
	go t.Log("x")
}

// [NOTCHECKED]: Method spawned directly on a value with an unexported context
//
// A method spawned directly is not judged by its receiver, so it passes,
// whatever the receiver holds.
func notCheckedDirectHiddenCtxMethodErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	h := ctxholder.NewHidden()
	g.Go(h.Run)
	_ = g.Wait()
}

// [NOTCHECKED]: Method spawned directly on a value with an unexported context
//
// A method spawned directly is not judged by its receiver, so it passes,
// whatever the receiver holds.
func notCheckedDirectHiddenCtxMethodWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	h := ctxholder.NewHidden()
	wg.Go(h.Do)
	wg.Wait()
}

// [NOTCHECKED]: Method spawned directly on a value with an unexported context
//
// A method spawned directly is not judged by its receiver, so it passes,
// whatever the receiver holds.
func notCheckedDirectHiddenCtxMethodSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	h := ctxholder.NewHidden()
	runWithGroup(g, h.Run)
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

func serveCtxService(s *ctxService) { s.run() }

// ctxServer takes a ctxService to handle.
type ctxServer struct{}

func (*ctxServer) handle(name string, s *ctxService) { s.run() }

func runAny(v any) { _ = v }

func logTo(t *testing.T) { t.Log("x") }

// plainHolder holds no context.
type plainHolder struct{ name string }

// ctxWrapper is a context with a value of its own.
type ctxWrapper struct {
	context.Context
	user string
}
