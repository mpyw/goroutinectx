// Package resolvederive tests that a goroutine or task given through a
// variable is resolved through SSA, with -goroutine-deriver set.
package resolvederive

import (
	"context"

	"github.com/my-example-app/telemetry/apm"
	gotask "github.com/siketyan/gotask/v2"
)

// ===== SHOULD REPORT =====

// [BAD]: The context derived after DoAsync is not the one given (#82).
func badLaterDerive(ctx context.Context) {
	t := gotask.NewTask(func(ctx context.Context) int { return 0 })
	t.DoAsync(ctx, nil) // want `gotask\.\(\*Task\)\.DoAsync\(\) 1st argument should call goroutine deriver`
	ctx = apm.NewGoroutineContext(ctx)
	_ = ctx
}

// [BAD]: A deriver called only in defer is reported, as it is inline (#83).
func badDeferIdent(ctx context.Context) {
	fn := func() {
		defer apm.NewGoroutineContext(ctx)
	}
	go fn() // want `goroutine calls github.com/my-example-app/telemetry/apm.NewGoroutineContext in defer, but it should be called at goroutine start`
}

// [BAD]: The same for a task (#83).
func badDeferIdentTask(ctx context.Context) {
	fn := func(ctx context.Context) int {
		defer apm.NewGoroutineContext(ctx)
		return 0
	}
	gotask.NewTask(fn).DoAsync(ctx, nil) // want `gotask\.\(\*Task\)\.DoAsync\(\) 1st argument should call goroutine deriver`
}

// ===== SHOULD NOT REPORT =====

// [GOOD]: The context derived before DoAsync is the one given.
func goodEarlierDerive(ctx context.Context) {
	t := gotask.NewTask(func(ctx context.Context) int { return 0 })
	ctx = apm.NewGoroutineContext(ctx)
	t.DoAsync(ctx, nil)
}

// [GOOD]: A closure through a variable that derives at its start.
func goodDeriveIdent(ctx context.Context) {
	fn := func() {
		ctx := apm.NewGoroutineContext(ctx)
		_ = ctx
	}
	go fn()
}
