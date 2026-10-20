// With -goroutine-deriver: capturing ctx, calling the deriver, both and neither.

package patternsderive

import (
	"context"
	"sync"

	"github.com/my-example-app/telemetry/apm"
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
