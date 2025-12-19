package goroutinederive

import (
	"context"

	"github.com/my-example-app/telemetry/apm"
)

// Test cases for goroutine-derive checker with -goroutine-deriver=github.com/my-example-app/telemetry/apm.NewGoroutineContext

// ===== SHOULD NOT REPORT =====

// [GOOD]: Basic - calls deriver.
//
// Goroutine properly calls the required context deriver function.
func goodCallsDeriver(ctx context.Context) {
	go func() {
		ctx := apm.NewGoroutineContext(ctx)
		_ = ctx
	}()
}

// [GOOD]: Basic - nested goroutines both call deriver.
//
// Both outer and inner goroutines call the deriver function.
func goodNestedBothCallDeriver(ctx context.Context) {
	go func() {
		ctx := apm.NewGoroutineContext(ctx)
		go func() {
			ctx := apm.NewGoroutineContext(ctx)
			_ = ctx
		}()
		_ = ctx
	}()
}

// [NOTCHECKED]: Basic - has own context param.
//
// Function declares its own context parameter, so outer context not required.
func notCheckedOwnContextParam(ctx context.Context) {
	go func(ctx context.Context) {
		_ = ctx
	}(ctx)
}

// [NOTCHECKED]: Basic - named function call (not checked).
//
// Named function call pattern is not checked for deriver.
func notCheckedNamedFuncCall(ctx context.Context) {
	go namedFunc(ctx)
}

//vt:helper
func namedFunc(ctx context.Context) {
	_ = ctx
}

// ===== SHOULD REPORT =====

// [BAD]: Basic - no deriver call.
//
// Goroutine does not call the required context deriver function.
func badNoDeriverCall(ctx context.Context) {
	go func() { // want "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
		_ = ctx
	}()
}

// [BAD]: Basic - uses different function (not deriver).
//
// Goroutine calls a function, but not the required deriver.
func badUsesDifferentFunc(ctx context.Context) {
	go func() { // want "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
		ctx := context.WithValue(ctx, "key", "value")
		_ = ctx
	}()
}

// [BAD]: Basic - nested, inner missing deriver.
//
// Inner goroutine does not call the required deriver.
func badNestedInnerMissingDeriver(ctx context.Context) {
	go func() {
		ctx := apm.NewGoroutineContext(ctx)
		go func() { // want "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
			_ = ctx
		}()
		_ = ctx
	}()
}

// ===== HIGHER-ORDER PATTERNS =====

// [GOOD]: Higher-order go fn()() - returned func calls deriver.
//
// Factory function returns a func that calls the required deriver.
func goodHigherOrderCallsDeriver(ctx context.Context) {
	makeWorker := func() func() {
		return func() {
			ctx := apm.NewGoroutineContext(ctx)
			_ = ctx
		}
	}
	go makeWorker()()
}

// [BAD]: Higher-order go fn()() - returned func missing deriver.
//
// Factory function returns a func that does not call the deriver.
func badHigherOrderMissingDeriver(ctx context.Context) {
	makeWorker := func() func() {
		return func() {
			_ = ctx
		}
	}
	go makeWorker()() // want "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
}

// [GOOD]: Higher-order returns variable - with deriver.
//
// Factory function returns a variable that calls the deriver.
func goodHigherOrderReturnsVariableWithDeriver(ctx context.Context) {
	makeWorker := func() func() {
		worker := func() {
			ctx := apm.NewGoroutineContext(ctx)
			_ = ctx
		}
		return worker // Returns variable, not literal
	}
	go makeWorker()()
}

// [BAD]: Higher-order returns variable - missing deriver.
//
// Factory function returns a variable that does not call the deriver.
func badHigherOrderReturnsVariableMissingDeriver(ctx context.Context) {
	makeWorker := func() func() {
		worker := func() {
			_ = ctx
		}
		return worker // Returns variable, not literal
	}
	go makeWorker()() // want "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
}

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

// [BAD]: Variable func go fn() - missing deriver.
//
// Function stored in variable does not call the deriver.
func badVariableFuncMissingDeriver(ctx context.Context) {
	fn := func() {
		_ = ctx
	}
	go fn() // want "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
}
