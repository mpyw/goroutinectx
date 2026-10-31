package goroutinederive

import (
	"context"
	"slices"

	"github.com/my-example-app/telemetry/apm"
)

// Test cases for goroutine-derive checker with -goroutine-deriver=github.com/my-example-app/telemetry/apm.NewGoroutineContext
// The cases it shares with the callback checkers are in testdata/src/patternsderive.

// ===== SHOULD NOT REPORT =====

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

func namedFunc(ctx context.Context) {
	_ = ctx
}

// ===== SHOULD REPORT =====

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

// ===== DEFER PATTERNS =====

// [BAD]: Deriver called only in defer.
//
// Deriver must be called at goroutine start, not in defer.
func badDeriverOnlyInDefer(ctx context.Context) {
	go func() { // want "goroutine calls github.com/my-example-app/telemetry/apm.NewGoroutineContext in defer, but it should be called at goroutine start"
		defer apm.NewGoroutineContext(ctx)
		_ = ctx
	}()
}

// [BAD]: Deriver in defer with IIFE wrapper.
//
// Deriver called in defer via IIFE is still considered defer-only.
func badDeriverInDeferIIFE(ctx context.Context) {
	go func() { // want "goroutine calls github.com/my-example-app/telemetry/apm.NewGoroutineContext in defer, but it should be called at goroutine start"
		defer func() {
			_ = apm.NewGoroutineContext(ctx)
		}()
		_ = ctx
	}()
}

// [GOOD]: Deriver in a closure both deferred and called at start.
//
// The direct call counts, though the defer comes first (#143).
func goodDeriverClosureDeferredAndCalled(ctx context.Context) {
	go func() {
		setup := func() { ctx = apm.NewGoroutineContext(ctx) }
		defer setup()
		setup()
		_ = ctx
	}()
}

// [BAD]: Deriver in a closure that is only deferred.
//
// The closure is deferred twice, but never called at start.
func badDeriverClosureOnlyDeferred(ctx context.Context) {
	go func() { // want "goroutine calls github.com/my-example-app/telemetry/apm.NewGoroutineContext in defer, but it should be called at goroutine start"
		setup := func() { ctx = apm.NewGoroutineContext(ctx) }
		defer setup()
		defer setup()
		_ = ctx
	}()
}

// [GOOD]: Deriver at start with defer for cleanup.
//
// Deriver called at goroutine start is OK, even if there's also a defer.
func goodDeriverAtStartWithDefer(ctx context.Context) {
	go func() {
		ctx := apm.NewGoroutineContext(ctx)
		defer func() {
			// Some cleanup
			_ = ctx
		}()
		_ = ctx
	}()
}

// ===== IIFE FACTORY PATTERNS =====

// [GOOD]: IIFE factory calls deriver
//
// Inline factory function returns a func that calls the required deriver.
func goodIIFEFactoryCallsDeriver(ctx context.Context) {
	go (func() func() {
		return func() {
			ctx := apm.NewGoroutineContext(ctx)
			_ = ctx
		}
	})()()
}

// [BAD]: IIFE factory calls deriver
//
// Inline factory function returns a func that does not call the deriver.
func badIIFEFactoryMissingDeriver(ctx context.Context) {
	go (func() func() { // want "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
		return func() {
			_ = ctx
		}
	})()()
}

// [NOTCHECKED]: IIFE factory has own context param
//
// Factory function has its own context parameter, so not checked.
func notCheckedIIFEFactoryOwnCtxParam(ctx context.Context) {
	go (func(ctx context.Context) func() {
		return func() {
			_ = ctx
		}
	})(ctx)()
}

// [NOTCHECKED]: IIFE factory returns func with context param
//
// Returned function has its own context parameter, so not checked.
func notCheckedIIFEFactoryReturnsCtxParam(ctx context.Context) {
	go (func() func(context.Context) {
		return func(ctx context.Context) {
			_ = ctx
		}
	})()(ctx)
}

// ===== VARIABLE FACTORY PATTERNS =====

// [NOTCHECKED]: Variable factory has own context param
//
// Factory variable has its own context parameter, so not checked.
func notCheckedVariableFactoryOwnCtxParam(ctx context.Context) {
	factory := func(ctx context.Context) func() {
		return func() {
			_ = ctx
		}
	}
	go factory(ctx)()
}

// [BAD]: Goroutine runs a factory whose result has its own ctx param
//
// The goroutine runs the factory itself, not the func it returns, and the
// factory neither uses ctx nor calls the deriver. The parentheses around the
// factory used to hide it (#118).
func badFactoryNestedFuncWithCtxParam(ctx context.Context) {
	go (func() func(context.Context) { // want `goroutine does not propagate context "ctx"` "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
		return func(ctx context.Context) {
			_ = ctx
		}
	})()
}

// [GOOD]: Factory returns variable with ctx param
//
// Factory returns a variable pointing to a func with its own ctx param.
func goodFactoryReturnsVariableWithCtxParam(ctx context.Context) {
	factory := func() func() {
		worker := func(ctx context.Context) {
			_ = ctx
		}
		// Return wrapper that calls worker
		return func() {
			worker(ctx)
		}
	}
	go factory()()
}

// ===== TRACING LIMITATION PATTERNS =====

// [LIMITATION]: Factory from function parameter - can't trace
//
// Factory function passed as parameter can't be traced.
func limitationFactoryFromParam(ctx context.Context, factory func() func()) {
	// factory can't be traced to a FuncLit - assumes OK
	go factory()()
}

// [BAD]: Returned value from external func - can't trace
//
// Return value from external function can't be traced.
func badReturnedFromExternal(ctx context.Context) {
	factory := func() func() {
		// Return from external function call - can't trace
		return getExternalFunc()
	}
	go factory()() // want "goroutine does not propagate context \"ctx\"" "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
}

func getExternalFunc() func() {
	return func() {}
}

// [BAD]: Deriver in a range-over-func loop body of a deferred IIFE.
//
// The loop body is walked as deferred, as the IIFE it is in is.
func badDeriverInDeferredRangeFunc(ctx context.Context, xs []int) {
	go func() { // want "goroutine calls github.com/my-example-app/telemetry/apm.NewGoroutineContext in defer, but it should be called at goroutine start"
		defer func() {
			for range slices.Values(xs) {
				_ = apm.NewGoroutineContext(ctx)
			}
		}()
	}()
}

// [GOOD]: Deriver in a nested range-over-func loop body.
func goodDeriverInNestedRangeFunc(ctx context.Context, xss [][]int) {
	go func() {
		for xs := range slices.Values(xss) {
			for range slices.Values(xs) {
				_ = apm.NewGoroutineContext(ctx)
			}
		}
	}()
}
