// With -goroutine-deriver: capturing ctx, calling the deriver, both and neither.

package patternsderive

import (
	"context"
	"slices"
	"sync"

	"github.com/my-example-app/telemetry/apm"
	"github.com/sourcegraph/conc/iter"
	"golang.org/x/sync/errgroup"
)

// ===== Captures ctx but does not call the deriver =====

// [BAD]: Basic - no deriver call.
//
// Goroutine does not call the required context deriver function.
func badNoDeriverCall(ctx context.Context) {
	go func() { // want "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
		_ = ctx
	}()
}

// [GOOD]: Callback captures context
func goodCapturesContextErrgroup(ctx context.Context) {
	g, _ := errgroup.WithContext(ctx)
	g.Go(func() error {
		_ = ctx // captures context
		return nil
	})
}

// [GOOD]: Callback captures context
func goodCapturesContextWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = ctx // captures context
	})
}

// [GOOD]: Callback captures context
func goodCapturesContextSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error {
		_ = ctx // captures context
		return nil
	})
}

// ===== Calls the deriver =====

// [GOOD]: Basic - calls deriver.
//
// Goroutine properly calls the required context deriver function.
func goodCallsDeriverGoroutine(ctx context.Context) {
	go func() {
		ctx := apm.NewGoroutineContext(ctx)
		_ = ctx
	}()
}

// [GOOD]: Callback calls deriver
func goodCallsDeriverErrgroup(ctx context.Context) {
	g, _ := errgroup.WithContext(ctx)
	g.Go(func() error {
		newCtx := apm.NewGoroutineContext(ctx)
		_ = newCtx
		return nil
	})
}

// [GOOD]: Callback calls deriver
func goodCallsDeriverWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		newCtx := apm.NewGoroutineContext(ctx)
		_ = newCtx
	})
}

// [GOOD]: Callback calls deriver
func goodCallsDeriverSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error {
		newCtx := apm.NewGoroutineContext(ctx)
		_ = newCtx
		return nil
	})
}

// ===== Calls the deriver without capturing ctx =====

// [GOOD]: Callback calls deriver without capturing ctx
func goodCallsDeriverOnlyErrgroup(ctx context.Context) {
	var g errgroup.Group
	g.Go(func() error {
		newCtx := apm.NewGoroutineContext(context.Background())
		_ = newCtx
		return nil
	})
}

// [GOOD]: Callback calls deriver without capturing ctx
func goodCallsDeriverOnlyWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		newCtx := apm.NewGoroutineContext(context.Background())
		_ = newCtx
	})
}

// [GOOD]: Callback calls deriver without capturing ctx
func goodCallsDeriverOnlySpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error {
		newCtx := apm.NewGoroutineContext(context.Background())
		_ = newCtx
		return nil
	})
}

// ===== Neither captures ctx nor calls the deriver =====

// [BAD]: Callback does not capture context or call deriver
func badNoContextNoDeriverErrgroup(ctx context.Context) {
	g, _ := errgroup.WithContext(ctx)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx" or call goroutine deriver`
		return nil
	})
}

// [BAD]: TryGo without context or deriver
func badTryGoNoContextNoDeriver(ctx context.Context) {
	g, _ := errgroup.WithContext(ctx)
	g.TryGo(func() error { // want `errgroup.Group.TryGo\(\) closure should use context "ctx" or call goroutine deriver`
		return nil
	})
}

// [BAD]: Callback does not capture context or call deriver
func badNoContextNoDeriverWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx" or call goroutine deriver`
	})
}

// [BAD]: Callback does not capture context or call deriver
func badNoContextNoDeriverSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error { // want `runWithGroup\(\) func argument should use context "ctx" or call goroutine deriver`
		return nil
	})
}

// [BAD]: WaitGroup callback without context or deriver
func badWaitGroupNoDeriver(ctx context.Context) {
	var wg sync.WaitGroup
	runWithWaitGroup(&wg, func() { // want `runWithWaitGroup\(\) func argument should use context "ctx" or call goroutine deriver`
	})
	wg.Wait()
}

// ===== A variable that calls the deriver =====

// [GOOD]: Variable func go fn() - calls deriver.
//
// Function stored in variable calls the deriver.
func goodVariableFuncCallsDeriver(ctx context.Context) {
	fn := func() {
		ctx := apm.NewGoroutineContext(ctx)
		_ = ctx
	}
	go fn()
}

// [GOOD]: Variable callback calls deriver
func goodVariableCallsDeriverErrgroup(ctx context.Context) {
	g, _ := errgroup.WithContext(ctx)
	fn := func() error {
		newCtx := apm.NewGoroutineContext(ctx)
		_ = newCtx
		return nil
	}
	g.Go(fn)
}

// [GOOD]: Variable callback calls deriver
func goodVariableCallsDeriverWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	fn := func() {
		newCtx := apm.NewGoroutineContext(ctx)
		_ = newCtx
	}
	wg.Go(fn)
}

// [GOOD]: Variable callback calls deriver
func goodVariableCallsDeriverSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error {
		newCtx := apm.NewGoroutineContext(ctx)
		_ = newCtx
		return nil
	}
	runWithGroup(g, fn)
}

// ===== A variable that does not call the deriver =====

// [BAD]: Variable func go fn() - missing deriver.
//
// Function stored in variable does not call the deriver.
func badVariableFuncMissingDeriver(ctx context.Context) {
	fn := func() {
		_ = ctx
	}
	go fn() // want "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
}

// [BAD]: Variable callback without deriver
func badVariableNoDeriverErrgroup(ctx context.Context) {
	g, _ := errgroup.WithContext(ctx)
	fn := func() error {
		return nil
	}
	g.Go(fn) // want `errgroup.Group.Go\(\) closure should use context "ctx" or call goroutine deriver`
}

// [BAD]: Variable callback without deriver
func badVariableNoDeriverWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	fn := func() {
	}
	wg.Go(fn) // want `sync.WaitGroup.Go\(\) closure should use context "ctx" or call goroutine deriver`
}

// [BAD]: Variable callback without deriver
func badVariableNoDeriverSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error {
		return nil
	}
	runWithGroup(g, fn) // want `runWithGroup\(\) func argument should use context "ctx" or call goroutine deriver`
}

// ===== A struct field that does not call the deriver =====

// [BAD]: Struct field go s.fn() - missing deriver.
//
// A func in a struct field is judged as a variable is (#116).
func badStructFieldMissingDeriver(ctx context.Context) {
	s := struct{ fn func() }{fn: func() {
		_ = ctx
	}}
	go s.fn() // want "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
}

// [GOOD]: Struct field go s.fn() - calls the deriver.
//
// A func in a struct field that calls the deriver passes.
func goodStructFieldCallsDeriver(ctx context.Context) {
	s := struct{ fn func() }{fn: func() {
		ctx := apm.NewGoroutineContext(ctx)
		_ = ctx
	}}
	go s.fn()
}

// [BAD]: Struct field callback without deriver
func badStructFieldNoDeriverErrgroup(ctx context.Context) {
	g, _ := errgroup.WithContext(ctx)
	s := struct{ fn func() error }{fn: func() error {
		return nil
	}}
	g.Go(s.fn) // want `errgroup.Group.Go\(\) closure should use context "ctx" or call goroutine deriver`
}

// ===== A decorator given a func that calls the deriver =====

func recoverDerive(f func()) func() {
	return func() {
		defer func() { _ = recover() }()
		f()
	}
}

// [GOOD]: Decorator given a func that calls the deriver.
//
// The decorator runs the func it is given, so that func is judged (#148).
func goodDecoratorCallsDeriver(ctx context.Context) {
	go recoverDerive(func() {
		ctx := apm.NewGoroutineContext(ctx)
		_ = ctx
	})()
}

// [BAD]: Decorator given a func that does not call the deriver.
//
// Neither the func nor the decorator calls the deriver.
func badDecoratorMissingDeriver(ctx context.Context) {
	go recoverDerive(func() { // want "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
		_ = ctx
	})()
}

// ===== A range-over-func loop body that calls the deriver =====

// [GOOD]: Deriver called in a range-over-func loop body.
//
// The loop body runs in the goroutine, as the body of a slice loop does (#196).
func goodRangeFuncCallsDeriverGoroutine(ctx context.Context, xs []int) {
	go func() {
		for x := range slices.Values(xs) {
			_ = apm.NewGoroutineContext(ctx)
			_ = x
		}
	}()
}

// [GOOD]: Callback calls deriver in a range-over-func loop body
func goodRangeFuncCallsDeriverErrgroup(ctx context.Context, xs []int) {
	var g errgroup.Group
	g.Go(func() error {
		for x := range slices.Values(xs) {
			_ = apm.NewGoroutineContext(context.Background())
			_ = x
		}
		return nil
	})
}

// [GOOD]: Callback calls deriver in a range-over-func loop body
func goodRangeFuncCallsDeriverWaitGroup(ctx context.Context, xs []int) {
	var wg sync.WaitGroup
	wg.Go(func() {
		for x := range slices.Values(xs) {
			_ = apm.NewGoroutineContext(context.Background())
			_ = x
		}
	})
}

// [GOOD]: Callback calls deriver in a range-over-func loop body
func goodRangeFuncCallsDeriverSpawner(ctx context.Context, xs []int) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error {
		for x := range slices.Values(xs) {
			_ = apm.NewGoroutineContext(context.Background())
			_ = x
		}
		return nil
	})
}

// [BAD]: Range-over-func loop body without deriver.
func badRangeFuncNoDeriverGoroutine(ctx context.Context, xs []int) {
	go func() { // want "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
		for x := range slices.Values(xs) {
			_ = ctx
			_ = x
		}
	}()
}

// [BAD]: Callback with a range-over-func loop body that neither captures ctx nor calls deriver
func badRangeFuncNoDeriverErrgroup(ctx context.Context, xs []int) {
	var g errgroup.Group
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx" or call goroutine deriver`
		for x := range slices.Values(xs) {
			_ = x
		}
		return nil
	})
}

// [BAD]: Callback with a range-over-func loop body that neither captures ctx nor calls deriver
func badRangeFuncNoDeriverWaitGroup(ctx context.Context, xs []int) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx" or call goroutine deriver`
		for x := range slices.Values(xs) {
			_ = x
		}
	})
}

// [BAD]: Callback with a range-over-func loop body that neither captures ctx nor calls deriver
func badRangeFuncNoDeriverSpawner(ctx context.Context, xs []int) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error { // want `runWithGroup\(\) func argument should use context "ctx" or call goroutine deriver`
		for x := range slices.Values(xs) {
			_ = x
		}
		return nil
	})
}

// ===== Callback takes a struct that holds a context =====

// ctxJob is a job that holds a context.
type ctxJob struct {
	Ctx  context.Context
	Name string
}

//goroutinectx:spawner
func runWithJob(j *ctxJob, fn func(*ctxJob)) {
	go fn(j)
}

// [BAD]: Callback reads ctx from its parameter, without the deriver
//
// A struct that holds a context is not a context parameter of its own, so
// the callback must still call the deriver (#256).
func badHolderParamNoDeriverConc(ctx context.Context, jobs []ctxJob) {
	iter.ForEach(jobs, func(j *ctxJob) { _ = j.Ctx }) // want `iter.ForEach\(\) closure should use context "ctx" or call goroutine deriver`
}

// [BAD]: Callback reads ctx from its parameter, without the deriver
//
// A struct that holds a context is not a context parameter of its own, so
// the callback must still call the deriver (#256).
func badHolderParamNoDeriverSpawner(ctx context.Context) {
	runWithJob(&ctxJob{Ctx: ctx}, func(j *ctxJob) { _ = j.Ctx }) // want `runWithJob\(\) func argument should use context "ctx" or call goroutine deriver`
}

// [GOOD]: Callback reads ctx from its parameter, with the deriver
func goodHolderParamCallsDeriverConc(ctx context.Context, jobs []ctxJob) {
	iter.ForEach(jobs, func(j *ctxJob) { _ = apm.NewGoroutineContext(j.Ctx) })
}

// [GOOD]: Callback reads ctx from its parameter, with the deriver
func goodHolderParamCallsDeriverSpawner(ctx context.Context) {
	runWithJob(&ctxJob{Ctx: ctx}, func(j *ctxJob) { _ = apm.NewGoroutineContext(j.Ctx) })
}
