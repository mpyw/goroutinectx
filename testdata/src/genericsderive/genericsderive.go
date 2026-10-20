// Package genericsderive tests gotask calls with explicit type arguments
// (#86), with -goroutine-deriver set.
package genericsderive

import (
	"context"

	"github.com/my-example-app/telemetry/apm"
	gotask "github.com/siketyan/gotask/v2"
)

// [GOOD]: NewTask with explicit type arguments wraps a deriving function.
func goodNewTaskExplicit(ctx context.Context) {
	_ = gotask.DoAllSettled(ctx, gotask.NewTask[int](func(ctx context.Context) int {
		_ = apm.NewGoroutineContext(ctx)
		return 0
	}))
}

// [BAD]: NewTask with explicit type arguments wraps a function that does not derive.
func badNewTaskExplicit(ctx context.Context) {
	_ = gotask.DoAllSettled(ctx, gotask.NewTask[int](func(ctx context.Context) int { // want `gotask\.DoAllSettled\(\) 2nd argument should call goroutine deriver`
		return 0
	}))
}

// [BAD]: DoAllFnsSettled with explicit type arguments.
func badDoAllFnsExplicit(ctx context.Context) {
	_ = gotask.DoAllFnsSettled[int](ctx, func(ctx context.Context) int { return 0 }) // want `gotask\.DoAllFnsSettled\(\) 2nd argument should call goroutine deriver`
}
