// Package gotask contains advanced test fixtures for the gotask context derivation checker.
package gotask

import (
	"context"

	"github.com/my-example-app/telemetry/apm"
	"github.com/samber/lo"
	gotask "github.com/siketyan/gotask/v2"
)

// ===== lo.Map with variadic expansion =====

// UnprocessedProduct represents an unprocessed product for testing.
type UnprocessedProduct struct {
	ID string
}

// ProcessedProduct represents a processed product for testing.
type ProcessedProduct struct {
	ID string
}

// [GOOD]: lo.Map callback returning func with deriver in inner func
//
// Higher-order function callback returning FuncLit with deriver is detected.
func goodLoMapWithDeriver(ctx context.Context) {
	chunk := []UnprocessedProduct{{ID: "1"}, {ID: "2"}}
	_ = gotask.DoAllFnsSettled(ctx,
		lo.Map(chunk, func(p UnprocessedProduct, _ int) func(context.Context) ProcessedProduct {
			return func(ctx context.Context) ProcessedProduct {
				_ = apm.NewGoroutineContext(ctx)
				return ProcessedProduct{ID: p.ID}
			}
		})...)
}

// [BAD]: lo.Map callback returning func without deriver
//
// Higher-order function callback returning FuncLit without deriver is detected.
func badLoMapWithoutDeriver(ctx context.Context) {
	chunk := []UnprocessedProduct{{ID: "1"}, {ID: "2"}}
	_ = gotask.DoAllFnsSettled(ctx, // want `gotask\.DoAllFnsSettled\(\) variadic argument should call goroutine deriver`
		lo.Map(chunk, func(p UnprocessedProduct, _ int) func(context.Context) ProcessedProduct {
			return func(ctx context.Context) ProcessedProduct {
				// No deriver called
				return ProcessedProduct{ID: p.ID}
			}
		})...)
}

// ===== More complex patterns =====

// [GOOD]: lo.Map callback returning gotask.NewTask
//
// Callback returning NewTask wrapping a FuncLit with deriver is detected.
func goodLoMapReturningNewTask(ctx context.Context) {
	chunk := []UnprocessedProduct{{ID: "1"}, {ID: "2"}}
	_ = gotask.DoAllSettled(ctx,
		lo.Map(chunk, func(p UnprocessedProduct, _ int) gotask.Task[ProcessedProduct] {
			return gotask.NewTask(func(ctx context.Context) ProcessedProduct {
				_ = apm.NewGoroutineContext(ctx)
				return ProcessedProduct{ID: p.ID}
			})
		})...)
}

// [BAD]: lo.Map callback returning gotask.NewTask
//
// Callback returning NewTask wrapping a FuncLit without deriver is detected.
func badLoMapReturningNewTask(ctx context.Context) {
	chunk := []UnprocessedProduct{{ID: "1"}, {ID: "2"}}
	_ = gotask.DoAllSettled(ctx, // want `gotask\.DoAllSettled\(\) variadic argument should call goroutine deriver`
		lo.Map(chunk, func(p UnprocessedProduct, _ int) gotask.Task[ProcessedProduct] {
			return gotask.NewTask(func(ctx context.Context) ProcessedProduct {
				return ProcessedProduct{ID: p.ID}
			})
		})...)
}

// [GOOD]: Nested lo.Map (inner returns func with deriver)
//
// Nested higher-order function pattern with deriver in innermost FuncLit.
func goodNestedLoMap(ctx context.Context) {
	outer := [][]UnprocessedProduct{{{ID: "1"}}, {{ID: "2"}}}
	_ = gotask.DoAllFnsSettled(ctx,
		lo.Map(outer, func(inner []UnprocessedProduct, _ int) func(context.Context) []ProcessedProduct {
			return func(ctx context.Context) []ProcessedProduct {
				_ = apm.NewGoroutineContext(ctx)
				return lo.Map(inner, func(p UnprocessedProduct, _ int) ProcessedProduct {
					return ProcessedProduct{ID: p.ID}
				})
			}
		})...)
}

// [GOOD]: Callback with early return containing deriver
//
// Early return path with deriver is detected.
func goodLoMapEarlyReturn(ctx context.Context) {
	chunk := []UnprocessedProduct{{ID: "1"}, {ID: "2"}}
	_ = gotask.DoAllFnsSettled(ctx,
		lo.Map(chunk, func(p UnprocessedProduct, _ int) func(context.Context) ProcessedProduct {
			if p.ID == "" {
				return func(ctx context.Context) ProcessedProduct {
					_ = apm.NewGoroutineContext(ctx)
					return ProcessedProduct{}
				}
			}
			return func(ctx context.Context) ProcessedProduct {
				_ = apm.NewGoroutineContext(ctx)
				return ProcessedProduct{ID: p.ID}
			}
		})...)
}

// [LIMITATION]: Callback with variable assignment before return
//
// Variable assignment in callback is not traced (would need SSA).
func limitationLoMapVariableAssignment(ctx context.Context) {
	chunk := []UnprocessedProduct{{ID: "1"}, {ID: "2"}}
	_ = gotask.DoAllFnsSettled(ctx, // want `gotask\.DoAllFnsSettled\(\) variadic argument should call goroutine deriver`
		lo.Map(chunk, func(p UnprocessedProduct, _ int) func(context.Context) ProcessedProduct {
			// Variable assignment - can't trace without SSA
			fn := func(ctx context.Context) ProcessedProduct {
				_ = apm.NewGoroutineContext(ctx)
				return ProcessedProduct{ID: p.ID}
			}
			return fn
		})...)
}

// [LIMITATION]: Deriver only in one return path
//
// Only one return path has deriver, but we detect ANY return with deriver as OK.
// This is a known trade-off for simplicity.
func limitationLoMapPartialDeriver(ctx context.Context) {
	chunk := []UnprocessedProduct{{ID: "1"}, {ID: "2"}}
	// Does NOT report because one return path has deriver
	_ = gotask.DoAllFnsSettled(ctx,
		lo.Map(chunk, func(p UnprocessedProduct, _ int) func(context.Context) ProcessedProduct {
			if p.ID == "" {
				return func(ctx context.Context) ProcessedProduct {
					// No deriver in this path!
					return ProcessedProduct{}
				}
			}
			return func(ctx context.Context) ProcessedProduct {
				_ = apm.NewGoroutineContext(ctx)
				return ProcessedProduct{ID: p.ID}
			}
		})...)
}

// ===== Deep nesting and chaining patterns =====

//vt:helper
func makeTaskFactory(p UnprocessedProduct) func(context.Context) ProcessedProduct {
	return func(ctx context.Context) ProcessedProduct {
		_ = apm.NewGoroutineContext(ctx)
		return ProcessedProduct{ID: p.ID}
	}
}

//vt:helper
func makeTaskFactoryNoDeriver(p UnprocessedProduct) func(context.Context) ProcessedProduct {
	return func(ctx context.Context) ProcessedProduct {
		return ProcessedProduct{ID: p.ID}
	}
}

// [LIMITATION]: lo.Map callback calling external factory function
//
// Factory function calls in callback return cannot be traced.
func limitationLoMapExternalFactory(ctx context.Context) {
	chunk := []UnprocessedProduct{{ID: "1"}, {ID: "2"}}
	_ = gotask.DoAllFnsSettled(ctx, // want `gotask\.DoAllFnsSettled\(\) variadic argument should call goroutine deriver`
		lo.Map(chunk, func(p UnprocessedProduct, _ int) func(context.Context) ProcessedProduct {
			return makeTaskFactory(p) // Returns func with deriver, but can't trace
		})...)
}

// [GOOD]: Inline factory function with deriver in return
//
// Inline factory returning FuncLit with deriver is detected.
func goodLoMapInlineFactory(ctx context.Context) {
	chunk := []UnprocessedProduct{{ID: "1"}, {ID: "2"}}
	makeTask := func(p UnprocessedProduct) func(context.Context) ProcessedProduct {
		return func(ctx context.Context) ProcessedProduct {
			_ = apm.NewGoroutineContext(ctx)
			return ProcessedProduct{ID: p.ID}
		}
	}
	_ = gotask.DoAllFnsSettled(ctx,
		lo.Map(chunk, func(p UnprocessedProduct, _ int) func(context.Context) ProcessedProduct {
			return makeTask(p)
		})...)
}

// [GOOD]: Multiple lo.Map chained (first is the task source)
//
// Chained lo.Map where outer one provides tasks with deriver.
func goodChainedLoMap(ctx context.Context) {
	ids := []string{"1", "2"}
	chunk := lo.Map(ids, func(id string, _ int) UnprocessedProduct {
		return UnprocessedProduct{ID: id}
	})
	_ = gotask.DoAllFnsSettled(ctx,
		lo.Map(chunk, func(p UnprocessedProduct, _ int) func(context.Context) ProcessedProduct {
			return func(ctx context.Context) ProcessedProduct {
				_ = apm.NewGoroutineContext(ctx)
				return ProcessedProduct{ID: p.ID}
			}
		})...)
}

// [GOOD]: Filter then Map pattern
//
// lo.Filter followed by lo.Map with deriver is detected.
func goodFilterThenMap(ctx context.Context) {
	chunk := []UnprocessedProduct{{ID: "1"}, {ID: ""}, {ID: "2"}}
	filtered := lo.Filter(chunk, func(p UnprocessedProduct, _ int) bool {
		return p.ID != ""
	})
	_ = gotask.DoAllFnsSettled(ctx,
		lo.Map(filtered, func(p UnprocessedProduct, _ int) func(context.Context) ProcessedProduct {
			return func(ctx context.Context) ProcessedProduct {
				_ = apm.NewGoroutineContext(ctx)
				return ProcessedProduct{ID: p.ID}
			}
		})...)
}
