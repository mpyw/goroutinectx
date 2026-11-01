// Package conc contains test fixtures for sourcegraph/conc context propagation checker.
// This file tests that the analyzer correctly detects context usage in conc pool APIs,
// including generic types like ResultPool[T].
package conc

import (
	"context"
	"fmt"
	"slices"

	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/iter"
	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/conc/stream"
)

// ===== conc.WaitGroup =====

// [BAD]: conc.WaitGroup.Go without ctx
func badConcWaitGroupGo(ctx context.Context) {
	wg := &conc.WaitGroup{}
	wg.Go(func() { // want `conc.WaitGroup.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
	})
	wg.Wait()
}

// [GOOD]: conc.WaitGroup.Go with ctx
func goodConcWaitGroupGo(ctx context.Context) {
	wg := &conc.WaitGroup{}
	wg.Go(func() {
		_ = ctx
	})
	wg.Wait()
}

// ===== pool.Pool =====

// [BAD]: pool.Pool.Go without ctx
func badPoolPoolGo(ctx context.Context) {
	p := &pool.Pool{}
	p.Go(func() { // want `pool.Pool.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
	})
	p.Wait()
}

// [GOOD]: pool.Pool.Go with ctx
func goodPoolPoolGo(ctx context.Context) {
	p := &pool.Pool{}
	p.Go(func() {
		_ = ctx
	})
	p.Wait()
}

// ===== pool.ResultPool[T] (generic) =====

// [BAD]: pool.ResultPool[T].Go without ctx
func badResultPoolGo(ctx context.Context) {
	p := &pool.ResultPool[int]{}
	p.Go(func() int { // want `pool.ResultPool.Go\(\) closure should use context "ctx"`
		return 42
	})
	_ = p.Wait()
}

// [GOOD]: pool.ResultPool[T].Go with ctx
func goodResultPoolGo(ctx context.Context) {
	p := &pool.ResultPool[int]{}
	p.Go(func() int {
		_ = ctx
		return 42
	})
	_ = p.Wait()
}

// ===== pool.ContextPool =====

// [BAD]: pool.ContextPool.Go without ctx capture (callback receives ctx as arg)
// Note: ContextPool passes ctx to callback, so this is actually OK
// The callback receives ctx as argument, not capturing from outside
func goodContextPoolGo(ctx context.Context) {
	p := &pool.ContextPool{}
	p.Go(func(ctx context.Context) error {
		// ctx is passed as argument, not captured - this is fine
		return nil
	})
	_ = p.Wait()
}

// ===== pool.ResultContextPool[T] (generic) =====

// [GOOD]: pool.ResultContextPool[T].Go - callback receives ctx
func goodResultContextPoolGo(ctx context.Context) {
	p := &pool.ResultContextPool[int]{}
	p.Go(func(ctx context.Context) (int, error) {
		// ctx is passed as argument
		return 42, nil
	})
	_, _ = p.Wait()
}

// ===== pool.ErrorPool =====

// [BAD]: pool.ErrorPool.Go without ctx
func badErrorPoolGo(ctx context.Context) {
	p := &pool.ErrorPool{}
	p.Go(func() error { // want `pool.ErrorPool.Go\(\) closure should use context "ctx"`
		return nil
	})
	_ = p.Wait()
}

// [GOOD]: pool.ErrorPool.Go with ctx
func goodErrorPoolGo(ctx context.Context) {
	p := &pool.ErrorPool{}
	p.Go(func() error {
		_ = ctx
		return nil
	})
	_ = p.Wait()
}

// ===== pool.ResultErrorPool[T] (generic) =====

// [BAD]: pool.ResultErrorPool[T].Go without ctx
func badResultErrorPoolGo(ctx context.Context) {
	p := &pool.ResultErrorPool[string]{}
	p.Go(func() (string, error) { // want `pool.ResultErrorPool.Go\(\) closure should use context "ctx"`
		return "result", nil
	})
	_, _ = p.Wait()
}

// [GOOD]: pool.ResultErrorPool[T].Go with ctx
func goodResultErrorPoolGo(ctx context.Context) {
	p := &pool.ResultErrorPool[string]{}
	p.Go(func() (string, error) {
		_ = ctx
		return "result", nil
	})
	_, _ = p.Wait()
}

// ===== stream.Stream =====

// [BAD]: stream.Stream.Go without ctx
func badStreamGo(ctx context.Context) {
	s := stream.New()
	s.Go(func() stream.Callback { // want `stream.Stream.Go\(\) closure should use context "ctx"`
		return func() {}
	})
	s.Wait()
}

// [GOOD]: stream.Stream.Go with ctx
func goodStreamGo(ctx context.Context) {
	s := stream.New()
	s.Go(func() stream.Callback {
		_ = ctx
		return func() {}
	})
	s.Wait()
}

// ===== iter.ForEach =====

// [BAD]: iter.ForEach without ctx
func badIterForEach(ctx context.Context) {
	items := []int{1, 2, 3}
	iter.ForEach(items, func(item *int) { // want `iter.ForEach\(\) closure should use context "ctx"`
		fmt.Println(*item)
	})
}

// [GOOD]: iter.ForEach with ctx
func goodIterForEach(ctx context.Context) {
	items := []int{1, 2, 3}
	iter.ForEach(items, func(item *int) {
		_ = ctx
		fmt.Println(*item)
	})
}

// ===== iter.ForEachIdx =====

// [BAD]: iter.ForEachIdx without ctx
func badIterForEachIdx(ctx context.Context) {
	items := []int{1, 2, 3}
	iter.ForEachIdx(items, func(idx int, item *int) { // want `iter.ForEachIdx\(\) closure should use context "ctx"`
		fmt.Println(idx, *item)
	})
}

// [GOOD]: iter.ForEachIdx with ctx
func goodIterForEachIdx(ctx context.Context) {
	items := []int{1, 2, 3}
	iter.ForEachIdx(items, func(idx int, item *int) {
		_ = ctx
		fmt.Println(idx, *item)
	})
}

// ===== iter.Map =====

// [BAD]: iter.Map without ctx
func badIterMap(ctx context.Context) {
	items := []int{1, 2, 3}
	_ = iter.Map(items, func(item *int) int { // want `iter.Map\(\) closure should use context "ctx"`
		return *item * 2
	})
}

// [GOOD]: iter.Map with ctx
func goodIterMap(ctx context.Context) {
	items := []int{1, 2, 3}
	_ = iter.Map(items, func(item *int) int {
		_ = ctx
		return *item * 2
	})
}

// [BAD]: iter.Map without ctx, in the range expression of a range-over-func loop
//
// The function around the loop makes the call (#254).
func badIterMapRangeFuncExpr(ctx context.Context, items []int) {
	f := func(item *int) int { return *item * 2 }
	for r := range slices.Values(iter.Map(items, f)) { // want `iter.Map\(\) closure should use context "ctx"`
		_ = r
	}
}

// [GOOD]: iter.Map with ctx, in the range expression of a range-over-func loop
func goodIterMapRangeFuncExpr(ctx context.Context, items []int) {
	f := func(item *int) int {
		_ = ctx
		return *item * 2
	}
	for r := range slices.Values(iter.Map(items, f)) {
		_ = r
	}
}

// ===== iter.MapErr =====

// [BAD]: iter.MapErr without ctx
func badIterMapErr(ctx context.Context) {
	items := []int{1, 2, 3}
	_, _ = iter.MapErr(items, func(item *int) (int, error) { // want `iter.MapErr\(\) closure should use context "ctx"`
		return *item * 2, nil
	})
}

// [GOOD]: iter.MapErr with ctx
func goodIterMapErr(ctx context.Context) {
	items := []int{1, 2, 3}
	_, _ = iter.MapErr(items, func(item *int) (int, error) {
		_ = ctx
		return *item * 2, nil
	})
}

// ===== iter.Iterator =====

// [BAD]: iter.Iterator.ForEach without ctx
func badIteratorForEach(ctx context.Context) {
	items := []int{1, 2, 3}
	it := iter.Iterator[int]{MaxGoroutines: 2}
	it.ForEach(items, func(item *int) { // want `iter.Iterator.ForEach\(\) closure should use context "ctx"`
		fmt.Println(*item)
	})
}

// [GOOD]: iter.Iterator.ForEach with ctx
func goodIteratorForEach(ctx context.Context) {
	items := []int{1, 2, 3}
	it := iter.Iterator[int]{MaxGoroutines: 2}
	it.ForEach(items, func(item *int) {
		_ = ctx
		fmt.Println(*item)
	})
}

// [BAD]: iter.Iterator.ForEachIdx without ctx
func badIteratorForEachIdx(ctx context.Context) {
	items := []int{1, 2, 3}
	it := iter.Iterator[int]{MaxGoroutines: 2}
	it.ForEachIdx(items, func(idx int, item *int) { // want `iter.Iterator.ForEachIdx\(\) closure should use context "ctx"`
		fmt.Println(idx, *item)
	})
}

// [GOOD]: iter.Iterator.ForEachIdx with ctx
func goodIteratorForEachIdx(ctx context.Context) {
	items := []int{1, 2, 3}
	it := iter.Iterator[int]{MaxGoroutines: 2}
	it.ForEachIdx(items, func(idx int, item *int) {
		_ = ctx
		fmt.Println(idx, *item)
	})
}

// ===== iter.Mapper =====

// [BAD]: iter.Mapper.Map without ctx
func badMapperMap(ctx context.Context) {
	items := []int{1, 2, 3}
	m := iter.Mapper[int, int]{MaxGoroutines: 2}
	_ = m.Map(items, func(item *int) int { // want `iter.Mapper.Map\(\) closure should use context "ctx"`
		return *item * 2
	})
}

// [GOOD]: iter.Mapper.Map with ctx
func goodMapperMap(ctx context.Context) {
	items := []int{1, 2, 3}
	m := iter.Mapper[int, int]{MaxGoroutines: 2}
	_ = m.Map(items, func(item *int) int {
		_ = ctx
		return *item * 2
	})
}

// [BAD]: iter.Mapper.MapErr without ctx
func badMapperMapErr(ctx context.Context) {
	items := []int{1, 2, 3}
	m := iter.Mapper[int, int]{MaxGoroutines: 2}
	_, _ = m.MapErr(items, func(item *int) (int, error) { // want `iter.Mapper.MapErr\(\) closure should use context "ctx"`
		return *item * 2, nil
	})
}

// [GOOD]: iter.Mapper.MapErr with ctx
func goodMapperMapErr(ctx context.Context) {
	items := []int{1, 2, 3}
	m := iter.Mapper[int, int]{MaxGoroutines: 2}
	_, _ = m.MapErr(items, func(item *int) (int, error) {
		_ = ctx
		return *item * 2, nil
	})
}
