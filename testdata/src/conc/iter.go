// Package conc contains test fixtures for the conc context propagation checker.
// This file covers iter package patterns - ForEach, ForEachIdx, Map, MapErr.
package conc

import (
	"context"
	"fmt"

	"github.com/sourcegraph/conc/iter"
)

// ===== iter.ForEach =====

// [BAD]: iter.ForEach without ctx
//
// iter.ForEach callback does not use context.
func badIterForEach(ctx context.Context) {
	items := []int{1, 2, 3}
	iter.ForEach(items, func(item *int) { // want `iter.ForEach\(\) callback should use context "ctx"`
		fmt.Println(*item)
	})
}

// [GOOD]: iter.ForEach with ctx
//
// iter.ForEach callback uses context.
func goodIterForEach(ctx context.Context) {
	items := []int{1, 2, 3}
	iter.ForEach(items, func(item *int) {
		_ = ctx
		fmt.Println(*item)
	})
}

// [GOOD]: iter.ForEach no ctx param
//
// No context parameter - not checked.
func goodIterForEachNoCtxParam() {
	items := []int{1, 2, 3}
	iter.ForEach(items, func(item *int) {
		fmt.Println(*item)
	})
}

// ===== iter.ForEachIdx =====

// [BAD]: iter.ForEachIdx without ctx
//
// iter.ForEachIdx callback does not use context.
func badIterForEachIdx(ctx context.Context) {
	items := []int{1, 2, 3}
	iter.ForEachIdx(items, func(i int, item *int) { // want `iter.ForEachIdx\(\) callback should use context "ctx"`
		fmt.Println(i, *item)
	})
}

// [GOOD]: iter.ForEachIdx with ctx
//
// iter.ForEachIdx callback uses context.
func goodIterForEachIdx(ctx context.Context) {
	items := []int{1, 2, 3}
	iter.ForEachIdx(items, func(i int, item *int) {
		_ = ctx
		fmt.Println(i, *item)
	})
}

// ===== iter.Map =====

// [BAD]: iter.Map without ctx
//
// iter.Map callback does not use context.
func badIterMap(ctx context.Context) {
	items := []int{1, 2, 3}
	_ = iter.Map(items, func(item *int) string { // want `iter.Map\(\) callback should use context "ctx"`
		return fmt.Sprintf("%d", *item)
	})
}

// [GOOD]: iter.Map with ctx
//
// iter.Map callback uses context.
func goodIterMap(ctx context.Context) {
	items := []int{1, 2, 3}
	_ = iter.Map(items, func(item *int) string {
		_ = ctx
		return fmt.Sprintf("%d", *item)
	})
}

// ===== iter.MapErr =====

// [BAD]: iter.MapErr without ctx
//
// iter.MapErr callback does not use context.
func badIterMapErr(ctx context.Context) {
	items := []int{1, 2, 3}
	_, _ = iter.MapErr(items, func(item *int) (string, error) { // want `iter.MapErr\(\) callback should use context "ctx"`
		return fmt.Sprintf("%d", *item), nil
	})
}

// [GOOD]: iter.MapErr with ctx
//
// iter.MapErr callback uses context.
func goodIterMapErr(ctx context.Context) {
	items := []int{1, 2, 3}
	_, _ = iter.MapErr(items, func(item *int) (string, error) {
		_ = ctx
		return fmt.Sprintf("%d", *item), nil
	})
}

// ===== iter.Iterator methods =====

// [BAD]: Iterator.ForEach without ctx
//
// Iterator.ForEach callback does not use context.
func badIteratorForEach(ctx context.Context) {
	items := []int{1, 2, 3}
	it := iter.Iterator[int]{MaxGoroutines: 2}
	it.ForEach(items, func(item *int) { // want `iter.Iterator.ForEach\(\) callback should use context "ctx"`
		fmt.Println(*item)
	})
}

// [GOOD]: Iterator.ForEach with ctx
//
// Iterator.ForEach callback uses context.
func goodIteratorForEach(ctx context.Context) {
	items := []int{1, 2, 3}
	it := iter.Iterator[int]{MaxGoroutines: 2}
	it.ForEach(items, func(item *int) {
		_ = ctx
		fmt.Println(*item)
	})
}

// ===== iter.Mapper methods =====

// [BAD]: Mapper.Map without ctx
//
// Mapper.Map callback does not use context.
func badMapperMap(ctx context.Context) {
	items := []int{1, 2, 3}
	m := iter.Mapper[int, string]{MaxGoroutines: 2}
	_ = m.Map(items, func(item *int) string { // want `iter.Mapper.Map\(\) callback should use context "ctx"`
		return fmt.Sprintf("%d", *item)
	})
}

// [GOOD]: Mapper.Map with ctx
//
// Mapper.Map callback uses context.
func goodMapperMap(ctx context.Context) {
	items := []int{1, 2, 3}
	m := iter.Mapper[int, string]{MaxGoroutines: 2}
	_ = m.Map(items, func(item *int) string {
		_ = ctx
		return fmt.Sprintf("%d", *item)
	})
}

// ===== Variable function patterns =====

// [BAD]: iter.ForEach with variable func without ctx
//
// Variable function passed to ForEach without context.
func badIterForEachVariableFunc(ctx context.Context) {
	items := []int{1, 2, 3}
	fn := func(item *int) {
		fmt.Println(*item)
	}
	iter.ForEach(items, fn) // want `iter.ForEach\(\) callback should use context "ctx"`
}

// [GOOD]: iter.ForEach with variable func with ctx
//
// Variable function passed to ForEach uses context.
func goodIterForEachVariableFunc(ctx context.Context) {
	items := []int{1, 2, 3}
	fn := func(item *int) {
		_ = ctx
		fmt.Println(*item)
	}
	iter.ForEach(items, fn) // OK - fn uses ctx
}
