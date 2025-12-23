// Package gotask contains test fixtures for the gotask context derivation checker.
package gotask

import (
	"context"

	"github.com/my-example-app/telemetry/apm"
	gotask "github.com/siketyan/gotask/v2"
)

// ===== DoAllFnsSettled - SHOULD REPORT =====

// [BAD]: DoAllFnsSettled - func literal with deriver
//
// DoAllFnsSettled - func literal without deriver
func badDoAllFnsSettledNoDeriver(ctx context.Context) {
	_ = gotask.DoAllFnsSettled( // want `gotask\.DoAllFnsSettled\(\) 2nd argument should call goroutine deriver`
		ctx,
		func(ctx context.Context) bool {
			return true
		},
	)
}

// [BAD]: DoAllFnsSettled - multiple args, some without deriver
//
// Task function does not call the required context deriver.
func badDoAllFnsSettledPartialDeriver(ctx context.Context) {
	_ = gotask.DoAllFnsSettled( // want `gotask\.DoAllFnsSettled\(\) 3rd argument should call goroutine deriver` `gotask\.DoAllFnsSettled\(\) 5th argument should call goroutine deriver`
		ctx,
		func(ctx context.Context) error {
			_ = apm.NewGoroutineContext(ctx)
			return nil
		},
		func(ctx context.Context) error {
			return nil
		},
		func(ctx context.Context) error {
			_ = apm.NewGoroutineContext(ctx)
			return nil
		},
		func(ctx context.Context) error {
			return nil
		},
	)
}

// [BAD]: DoAllFnsSettled - deriver called on parent ctx (still bad - deriver must be inside task body)
//
// Task function does not call context deriver.
func badDoAllFnsSettledDerivedParentCtx(ctx context.Context) {
	_ = gotask.DoAllFnsSettled( // want `gotask\.DoAllFnsSettled\(\) 2nd argument should call goroutine deriver`
		apm.NewGoroutineContext(ctx),
		func(ctx context.Context) error {
			return nil
		},
	)
}

// ===== DoAllSettled with NewTask - SHOULD REPORT =====

// [BAD]: DoAllSettled - NewTask with deriver
//
// DoAllSettled - NewTask without deriver
func badDoAllSettledNewTaskNoDeriver(ctx context.Context) {
	_ = gotask.DoAllSettled( // want `gotask\.DoAllSettled\(\) 2nd argument should call goroutine deriver`
		ctx,
		gotask.NewTask(func(ctx context.Context) bool {
			return true
		}),
	)
}

// [BAD]: DoAllSettled - NewTask with deriver on parent ctx (still bad - deriver must be inside task body)
//
// Task function properly calls the context deriver.
func badDoAllSettledNewTaskDerivedParentCtx(ctx context.Context) {
	_ = gotask.DoAllSettled( // want `gotask\.DoAllSettled\(\) 2nd argument should call goroutine deriver`
		apm.NewGoroutineContext(ctx),
		gotask.NewTask(func(ctx context.Context) bool {
			return true
		}),
	)
}

// ===== DoAsync - SHOULD REPORT =====

// [BAD]: Task - without deriver on ctx
//
// Task.DoAsync without deriver on ctx
func badTaskDoAsyncNoDeriver(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		return nil
	})
	errChan := make(chan error)

	task.DoAsync(ctx, errChan) // want `gotask\.\(\*Task\)\.DoAsync\(\) 1st argument should call goroutine deriver`
}

// [BAD]: Task - with nil channel (ctx still needs deriver)
//
// Task.DoAsync with nil channel (ctx still needs deriver)
func badTaskDoAsyncNilChannel(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		return nil
	})

	task.DoAsync(ctx, nil) // want `gotask\.\(\*Task\)\.DoAsync\(\) 1st argument should call goroutine deriver`
}

// [BAD]: CancelableTask - with deriver
//
// CancelableTask.DoAsync without deriver on ctx
func badCancelableTaskDoAsyncNoDeriver(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		return nil
	}).Cancelable()
	errChan := make(chan error)

	task.DoAsync(ctx, errChan) // want `gotask\.\(\*CancelableTask\)\.DoAsync\(\) 1st argument should call goroutine deriver`
}

// ===== DoAllFnsSettled - SHOULD NOT REPORT =====

// [GOOD]: DoAllFnsSettled - func literal with deriver
//
// Task function properly calls the context deriver.
func goodDoAllFnsSettledWithDeriver(ctx context.Context) {
	_ = gotask.DoAllFnsSettled(
		ctx,
		func(ctx context.Context) bool {
			ctx = apm.NewGoroutineContext(ctx)
			_ = ctx
			return true
		},
	)
}

// [GOOD]: DoAllFnsSettled - deriver called but result assigned
//
// Task function properly calls context deriver.
func goodDoAllFnsSettledDerivedAssigned(ctx context.Context) {
	_ = gotask.DoAllFnsSettled(
		ctx,
		func(ctx context.Context) error {
			_ = apm.NewGoroutineContext(ctx)
			return nil
		},
	)
}

// [GOOD]: DoAllFnsSettled - all args have deriver
//
// Task function properly calls context deriver.
func goodDoAllFnsSettledAllWithDeriver(ctx context.Context) {
	_ = gotask.DoAllFnsSettled(
		ctx,
		func(ctx context.Context) error {
			_ = apm.NewGoroutineContext(ctx)
			return nil
		},
		func(ctx context.Context) error {
			_ = apm.NewGoroutineContext(ctx)
			return nil
		},
	)
}

// ===== DoAllSettled with NewTask - SHOULD NOT REPORT =====

// [GOOD]: DoAllSettled - NewTask with deriver
//
// Task function properly calls the context deriver.
func goodDoAllSettledNewTaskWithDeriver(ctx context.Context) {
	_ = gotask.DoAllSettled(
		ctx,
		gotask.NewTask(func(ctx context.Context) bool {
			ctx = apm.NewGoroutineContext(ctx)
			_ = ctx
			return true
		}),
	)
}

// ===== DoAsync - SHOULD NOT REPORT =====

// [GOOD]: Task - with deriver on ctx
//
// Task.DoAsync with deriver on ctx
func goodTaskDoAsyncWithDeriver(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		return nil
	})
	errChan := make(chan error)

	task.DoAsync(apm.NewGoroutineContext(ctx), errChan)
}

// [GOOD]: CancelableTask - with deriver
//
// CancelableTask.DoAsync with deriver on ctx
func goodCancelableTaskDoAsyncWithDeriver(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		return nil
	}).Cancelable()
	errChan := make(chan error)

	task.DoAsync(apm.NewGoroutineContext(ctx), errChan)
}

// [GOOD]: Task.DoAsync - callback calls deriver (ctx not derived)
//
// Task.DoAsync with callback calling deriver - ctx doesn't need to be derived
func goodTaskDoAsyncCallbackCallsDeriver(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		_ = apm.NewGoroutineContext(ctx) // Callback calls deriver
		return nil
	})
	errChan := make(chan error)

	task.DoAsync(ctx, errChan) // OK - callback already calls deriver
}

// [GOOD]: CancelableTask.DoAsync - callback calls deriver
//
// CancelableTask.DoAsync with callback calling deriver
func goodCancelableTaskDoAsyncCallbackCallsDeriver(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		_ = apm.NewGoroutineContext(ctx) // Callback calls deriver
		return nil
	}).Cancelable()
	errChan := make(chan error)

	task.DoAsync(ctx, errChan) // OK - callback already calls deriver
}

// [GOOD]: Task.DoAsync - both callback and ctx call deriver
//
// Task.DoAsync with both callback and ctx calling deriver
func goodTaskDoAsyncBothCallDeriver(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		_ = apm.NewGoroutineContext(ctx) // Callback calls deriver
		return nil
	})
	errChan := make(chan error)

	task.DoAsync(apm.NewGoroutineContext(ctx), errChan) // OK - both call deriver
}

// ===== Other Do* functions - SHOULD REPORT =====

// [BAD]: DoAll with deriver
//
// DoAll without deriver
func badDoAllNoDeriver(ctx context.Context) {
	_ = gotask.DoAll( // want `gotask\.DoAll\(\) 2nd argument should call goroutine deriver`
		ctx,
		gotask.NewTask(func(ctx context.Context) gotask.Result[int] {
			return gotask.Result[int]{Value: 1}
		}),
	)
}

// [BAD]: DoAllFns with deriver
//
// DoAllFns without deriver
func badDoAllFnsNoDeriver(ctx context.Context) {
	_ = gotask.DoAllFns( // want `gotask\.DoAllFns\(\) 2nd argument should call goroutine deriver`
		ctx,
		func(ctx context.Context) gotask.Result[int] {
			return gotask.Result[int]{Value: 1}
		},
	)
}

// [BAD]: DoRace with deriver
//
// DoRace without deriver
func badDoRaceNoDeriver(ctx context.Context) {
	_ = gotask.DoRace( // want `gotask\.DoRace\(\) 2nd argument should call goroutine deriver`
		ctx,
		gotask.NewTask(func(ctx context.Context) int {
			return 1
		}),
	)
}

// [BAD]: DoRaceFns with deriver
//
// DoRaceFns without deriver
func badDoRaceFnsNoDeriver(ctx context.Context) {
	_ = gotask.DoRaceFns( // want `gotask\.DoRaceFns\(\) 2nd argument should call goroutine deriver`
		ctx,
		func(ctx context.Context) int {
			return 1
		},
	)
}

// ===== Other Do* functions - SHOULD NOT REPORT =====

// [GOOD]: DoAll with deriver
//
// Task function properly calls the context deriver.
func goodDoAllWithDeriver(ctx context.Context) {
	_ = gotask.DoAll(
		ctx,
		gotask.NewTask(func(ctx context.Context) gotask.Result[int] {
			_ = apm.NewGoroutineContext(ctx)
			return gotask.Result[int]{Value: 1}
		}),
	)
}

// [GOOD]: DoAllFns with deriver
//
// Task function properly calls the context deriver.
func goodDoAllFnsWithDeriver(ctx context.Context) {
	_ = gotask.DoAllFns(
		ctx,
		func(ctx context.Context) gotask.Result[int] {
			_ = apm.NewGoroutineContext(ctx)
			return gotask.Result[int]{Value: 1}
		},
	)
}

// [GOOD]: DoRace with deriver
//
// Task function properly calls the context deriver.
func goodDoRaceWithDeriver(ctx context.Context) {
	_ = gotask.DoRace(
		ctx,
		gotask.NewTask(func(ctx context.Context) int {
			_ = apm.NewGoroutineContext(ctx)
			return 1
		}),
	)
}

// [GOOD]: DoRaceFns with deriver
//
// Task function properly calls the context deriver.
func goodDoRaceFnsWithDeriver(ctx context.Context) {
	_ = gotask.DoRaceFns(
		ctx,
		func(ctx context.Context) int {
			_ = apm.NewGoroutineContext(ctx)
			return 1
		},
	)
}

// ===== Ignore directive =====

// [GOOD]: Ignore directive on DoAllFnsSettled
//
// The //goroutinectx:ignore directive suppresses the warning.
func goodIgnoreDoAllFnsSettled(ctx context.Context) {
	//goroutinectx:ignore
	_ = gotask.DoAllFnsSettled(
		ctx,
		func(ctx context.Context) bool {
			return true
		},
	)
}

// [GOOD]: Ignore directive on Task
//
// Ignore directive on Task.DoAsync
func goodIgnoreTaskDoAsync(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error { return nil })

	//goroutinectx:ignore
	task.DoAsync(ctx, nil)
}

// ===== No ctx param - SHOULD NOT REPORT =====

// [GOOD]: No ctx param
//
// No ctx param - not checked
func goodNoCtxParam() {
	_ = gotask.DoAllFnsSettled(
		context.Background(),
		func(ctx context.Context) bool {
			return true
		},
	)
}
