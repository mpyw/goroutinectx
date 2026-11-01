// Package gotask contains advanced test fixtures for the gotask context derivation checker.
package gotask

import (
	"context"
	"slices"

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
	_ = gotask.DoAllFnsSettled(ctx,
		lo.Map(chunk, func(p UnprocessedProduct, _ int) func(context.Context) ProcessedProduct { // want `gotask\.DoAllFnsSettled\(\) variadic argument should call goroutine deriver`
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
	_ = gotask.DoAllSettled(ctx,
		lo.Map(chunk, func(p UnprocessedProduct, _ int) gotask.Task[ProcessedProduct] { // want `gotask\.DoAllSettled\(\) variadic argument should call goroutine deriver`
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

// [GOOD]: Callback with variable assignment before return
//
// The closure assigned to a variable before its return is resolved through SSA.
func goodLoMapVariableAssignment(ctx context.Context) {
	chunk := []UnprocessedProduct{{ID: "1"}, {ID: "2"}}
	_ = gotask.DoAllFnsSettled(ctx,
		lo.Map(chunk, func(p UnprocessedProduct, _ int) func(context.Context) ProcessedProduct {
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

func makeTaskFactory(p UnprocessedProduct) func(context.Context) ProcessedProduct {
	return func(ctx context.Context) ProcessedProduct {
		_ = apm.NewGoroutineContext(ctx)
		return ProcessedProduct{ID: p.ID}
	}
}

func makeTaskFactoryNoDeriver(p UnprocessedProduct) func(context.Context) ProcessedProduct {
	return func(ctx context.Context) ProcessedProduct {
		return ProcessedProduct{ID: p.ID}
	}
}

// [GOOD]: lo.Map callback calling external factory function
//
// The factory function the callback calls is resolved through SSA.
func goodLoMapExternalFactory(ctx context.Context) {
	chunk := []UnprocessedProduct{{ID: "1"}, {ID: "2"}}
	_ = gotask.DoAllFnsSettled(ctx,
		lo.Map(chunk, func(p UnprocessedProduct, _ int) func(context.Context) ProcessedProduct {
			return makeTaskFactory(p) // Returns func with deriver
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

// ===== Callback Variable Patterns =====

// [GOOD]: DoAsync with callback variable
//
// Callback assigned to variable then passed to NewTask, callback calls deriver.
func goodDoAsyncCallbackVariable(ctx context.Context) {
	callback := func(ctx context.Context) error {
		_ = apm.NewGoroutineContext(ctx)
		return nil
	}
	task := gotask.NewTask(callback)
	errChan := make(chan error, 1)
	task.DoAsync(ctx, errChan) // OK - callback calls deriver
}

// [BAD]: DoAsync with callback variable
//
// Callback assigned to variable then passed to NewTask, callback doesn't call deriver.
func badDoAsyncCallbackVariable(ctx context.Context) {
	callback := func(ctx context.Context) error {
		// No deriver call
		return nil
	}
	task := gotask.NewTask(callback)
	errChan := make(chan error, 1)
	task.DoAsync(ctx, errChan) // want `gotask\.Task\.DoAsync\(\) 1st argument should call goroutine deriver`
}

// [GOOD]: DoAllFnsSettled with callback variable
//
// Callback assigned to variable that calls deriver, used in variadic call.
func goodDoAllFnsSettledCallbackVariable(ctx context.Context) {
	fn := func(ctx context.Context) int {
		_ = apm.NewGoroutineContext(ctx)
		return 42
	}
	_ = gotask.DoAllFnsSettled(ctx, fn)
}

// [BAD]: DoAllFnsSettled with callback variable
//
// Callback assigned to variable without deriver call.
func badDoAllFnsSettledCallbackVariable(ctx context.Context) {
	fn := func(ctx context.Context) int {
		// No deriver call
		return 42
	}
	_ = gotask.DoAllFnsSettled(ctx, fn) // want `gotask\.DoAllFnsSettled\(\) 2nd argument should call goroutine deriver`
}

// [GOOD]: DoAsync with derived context variable
//
// Derived context stored in variable and passed to DoAsync.
func goodDoAsyncDerivedVariable(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		return nil
	})
	derivedCtx := apm.NewGoroutineContext(ctx)
	errChan := make(chan error, 1)
	task.DoAsync(derivedCtx, errChan) // OK - derivedCtx is a deriver call result
}

// [BAD]: DoAsync with derived context variable
//
// Non-deriver call result stored in variable and passed to DoAsync.
func badDoAsyncNonDeriverVariable(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		return nil
	})
	nonDerivedCtx := context.Background()
	errChan := make(chan error, 1)
	task.DoAsync(nonDerivedCtx, errChan) // want `gotask\.Task\.DoAsync\(\) 1st argument should call goroutine deriver`
}

// ===== Combinators =====

func derivingTask() gotask.Task[gotask.Result[int]] {
	return gotask.NewTask(func(ctx context.Context) gotask.Result[int] {
		_ = apm.NewGoroutineContext(ctx)
		return gotask.ResultOk(1)
	})
}

// [BAD]: Parallel given a task without deriver
//
// Parallel runs each task in a goroutine of its own (#197).
func badParallelNoDeriver(ctx context.Context) {
	bad := gotask.NewTask(func(ctx context.Context) gotask.Result[int] {
		return gotask.ResultOk(1)
	})
	_ = gotask.DoAll(ctx, gotask.Parallel(bad)) // want `gotask\.DoAll\(\) 2nd argument should call goroutine deriver`
}

// [BAD]: Parallel given one task without deriver among several
func badParallelPartialDeriver(ctx context.Context) {
	good := gotask.NewTask(func(ctx context.Context) gotask.Result[int] {
		_ = apm.NewGoroutineContext(ctx)
		return gotask.ResultOk(1)
	})
	bad := gotask.NewTask(func(ctx context.Context) gotask.Result[int] {
		return gotask.ResultOk(2)
	})
	_ = gotask.DoAll(ctx, gotask.Parallel(good, bad)) // want `gotask\.DoAll\(\) 2nd argument should call goroutine deriver`
}

// [GOOD]: Parallel given tasks that all call deriver
func goodParallelWithDeriver(ctx context.Context) {
	good := gotask.NewTask(func(ctx context.Context) gotask.Result[int] {
		_ = apm.NewGoroutineContext(ctx)
		return gotask.ResultOk(1)
	})
	_ = gotask.DoAll(ctx, gotask.Parallel(good, good))
}

// [BAD]: Parallel given a slice literal holding a task without deriver
func badParallelExpandedSlice(ctx context.Context) {
	tasks := []gotask.Task[gotask.Result[int]]{
		gotask.NewTask(func(ctx context.Context) gotask.Result[int] {
			return gotask.ResultOk(1)
		}),
	}
	_ = gotask.DoAll(ctx, gotask.Parallel(tasks...)) // want `gotask\.DoAll\(\) 2nd argument should call goroutine deriver`
}

// [LIMITATION]: Parallel given a slice parameter
//
// The elements of a slice parameter are not traced, so the task passes.
func limitationParallelSliceParam(ctx context.Context, tasks []gotask.Task[gotask.Result[int]]) {
	_ = gotask.DoAll(ctx, gotask.Parallel(tasks...))
}

// [BAD]: ParallelSettled given a task without deriver
func badParallelSettledNoDeriver(ctx context.Context) {
	bad := gotask.NewTask(func(ctx context.Context) int { return 1 })
	_ = gotask.DoAllSettled(ctx, gotask.ParallelSettled(bad)) // want `gotask\.DoAllSettled\(\) 2nd argument should call goroutine deriver`
}

// [BAD]: Race given a task without deriver
func badRaceNoDeriver(ctx context.Context) {
	bad := gotask.NewTask(func(ctx context.Context) int { return 1 })
	_ = gotask.DoRace(ctx, gotask.Race(bad)) // want `gotask\.DoRace\(\) 2nd argument should call goroutine deriver`
}

// [GOOD]: Race given a task that calls deriver
func goodRaceWithDeriver(ctx context.Context) {
	good := gotask.NewTask(func(ctx context.Context) int {
		_ = apm.NewGoroutineContext(ctx)
		return 1
	})
	_ = gotask.DoRace(ctx, gotask.Race(good))
}

// [GOOD]: Then given a task without deriver and a func that calls it
//
// The task and the func run in one goroutine, so one deriver call is enough.
func goodThenFuncCallsDeriver(ctx context.Context) {
	bad := gotask.NewTask(func(ctx context.Context) gotask.Result[int] {
		return gotask.ResultOk(1)
	})
	_ = gotask.DoAll(ctx, gotask.Then(bad, func(ctx context.Context, v int) gotask.Result[int] {
		_ = apm.NewGoroutineContext(ctx)
		return gotask.ResultOk(v)
	}))
}

// [GOOD]: Then given a task that calls deriver and a func without it
func goodThenTaskCallsDeriver(ctx context.Context) {
	_ = gotask.DoAll(ctx, gotask.Then(derivingTask(), func(ctx context.Context, v int) gotask.Result[int] {
		return gotask.ResultOk(v)
	}))
}

// [BAD]: Then given a task and a func that both lack deriver
func badThenNoDeriver(ctx context.Context) {
	bad := gotask.NewTask(func(ctx context.Context) gotask.Result[int] {
		return gotask.ResultOk(1)
	})
	_ = gotask.DoAll(ctx, gotask.Then(bad, func(ctx context.Context, v int) gotask.Result[int] { // want `gotask\.DoAll\(\) 2nd argument should call goroutine deriver`
		return gotask.ResultOk(v)
	}))
}

// [GOOD]: Catch given a task without deriver and a func that calls it
func goodCatchFuncCallsDeriver(ctx context.Context) {
	bad := gotask.NewTask(func(ctx context.Context) gotask.Result[int] {
		return gotask.ResultOk(1)
	})
	_ = gotask.DoRace(ctx, gotask.Catch(bad, func(ctx context.Context, err error) int {
		_ = apm.NewGoroutineContext(ctx)
		return 0
	}))
}

// [BAD]: Catch given a task and a func that both lack deriver
func badCatchNoDeriver(ctx context.Context) {
	bad := gotask.NewTask(func(ctx context.Context) gotask.Result[int] {
		return gotask.ResultOk(1)
	})
	_ = gotask.DoRace(ctx, gotask.Catch(bad, func(ctx context.Context, err error) int { // want `gotask\.DoRace\(\) 2nd argument should call goroutine deriver`
		return 0
	}))
}

// [BAD]: Nested combinators with an inner task without deriver
func badNestedCombinators(ctx context.Context) {
	bad := gotask.NewTask(func(ctx context.Context) gotask.Result[int] {
		return gotask.ResultOk(1)
	})
	inner := gotask.Then(bad, func(ctx context.Context, v int) gotask.Result[int] {
		return gotask.ResultOk(v)
	})
	_ = gotask.DoAll(ctx, gotask.Parallel(derivingTask(), inner)) // want `gotask\.DoAll\(\) 2nd argument should call goroutine deriver`
}

// [BAD]: DoAsync of a combinator given a task without deriver
func badCombinatorDoAsync(ctx context.Context) {
	bad := gotask.NewTask(func(ctx context.Context) gotask.Result[int] {
		return gotask.ResultOk(1)
	})
	ch := make(chan gotask.Result[[]int], 1)
	gotask.Parallel(bad).DoAsync(ctx, ch) // want `gotask\.Task\.DoAsync\(\) 1st argument should call goroutine deriver`
}

// [GOOD]: DoAsync of a combinator given tasks that call deriver
func goodCombinatorDoAsync(ctx context.Context) {
	ch := make(chan gotask.Result[[]int], 1)
	gotask.Parallel(derivingTask()).DoAsync(ctx, ch)
}

// ===== Range-over-func loops =====

// [GOOD]: Deriver called in a range-over-func loop body
//
// The loop body runs in the task, as the body of a slice loop does (#196).
func goodRangeFuncCallsDeriver(ctx context.Context, xs []int) {
	_ = gotask.DoAllFnsSettled(ctx, func(ctx context.Context) error {
		for x := range slices.Values(xs) {
			_ = apm.NewGoroutineContext(ctx)
			_ = x
		}
		return nil
	})
}

// [GOOD]: Variadic call in the range expression of a range-over-func loop
//
// The function around the loop makes the call, so its tasks are traced as in
// a plain statement (#254).
func goodRangeFuncExprVariadic(ctx context.Context) {
	fns := []func(context.Context) error{
		func(ctx context.Context) error {
			_ = apm.NewGoroutineContext(ctx)
			return nil
		},
	}
	for r := range slices.Values(gotask.DoAllFnsSettled(ctx, fns...)) {
		_ = r
	}
}

// [BAD]: Variadic call in the range expression of a range-over-func loop, without deriver
func badRangeFuncExprVariadic(ctx context.Context) {
	fns := []func(context.Context) error{
		func(ctx context.Context) error { return nil },
	}
	for r := range slices.Values(gotask.DoAllFnsSettled(ctx, fns...)) { // want `gotask\.DoAllFnsSettled\(\) variadic argument should call goroutine deriver`
		_ = r
	}
}

// [GOOD]: Range expression of a range-over-func loop in a range-over-func loop body
func goodRangeFuncExprNested(ctx context.Context, xs []int) {
	for x := range slices.Values(xs) {
		fns := []func(context.Context) error{
			func(ctx context.Context) error {
				_ = apm.NewGoroutineContext(ctx)
				return nil
			},
		}
		for r := range slices.Values(gotask.DoAllFnsSettled(ctx, fns...)) {
			_, _ = x, r
		}
	}
}

// [BAD]: Range expression of a range-over-func loop in a range-over-func loop body, without deriver
func badRangeFuncExprNested(ctx context.Context, xs []int) {
	for x := range slices.Values(xs) {
		fns := []func(context.Context) error{
			func(ctx context.Context) error { return nil },
		}
		for r := range slices.Values(gotask.DoAllFnsSettled(ctx, fns...)) { // want `gotask\.DoAllFnsSettled\(\) variadic argument should call goroutine deriver`
			_, _ = x, r
		}
	}
}
