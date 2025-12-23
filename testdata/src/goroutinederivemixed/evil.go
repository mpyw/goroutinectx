package goroutinederivemixed

import (
	"context"

	"github.com/my-example-app/telemetry/apm"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// =============================================================================
// EVIL: Mixed AND/OR - adversarial patterns
// Test flag: -goroutine-deriver=github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+github.com/newrelic/go-agent/v3/newrelic.NewContext,github.com/my-example-app/telemetry/apm.NewGoroutineContext
// =============================================================================

// ===== SHOULD NOT REPORT =====

// [GOOD]: Mixed - nested 2-level, outer satisfies AND group, inner satisfies OR alternative.
//
// Nested goroutines where both levels call required derivers.
func goodMixedNested2LevelDifferentApproaches(ctx context.Context, txn *newrelic.Transaction) {
	go func() {
		txn = txn.NewGoroutine()
		ctx = newrelic.NewContext(ctx, txn)
		go func() {
			ctx = apm.NewGoroutineContext(ctx) // Inner satisfies OR alternative
			_ = ctx
		}()
		_ = ctx
	}()
}

// [GOOD]: Mixed - nested 2-level, outer satisfies OR alternative, inner satisfies AND group.
//
// Nested goroutines where both levels call required derivers.
func goodMixedNested2LevelReversedApproaches(ctx context.Context, txn *newrelic.Transaction) {
	go func() {
		ctx = apm.NewGoroutineContext(ctx)
		go func() {
			txn = txn.NewGoroutine()
			ctx = newrelic.NewContext(ctx, txn)
			_ = ctx
		}()
		_ = ctx
	}()
}

// ===== SHOULD REPORT =====

// [GOOD]: Mixed - nested 2-level, inner satisfies neither.
//
// Inner goroutine satisfies neither OR nor AND requirement (outer handles).
func goodMixedNested2LevelInnerSatisfiesNeither(ctx context.Context, txn *newrelic.Transaction) {
	go func() {
		ctx = apm.NewGoroutineContext(ctx)
		go func() { // want "goroutine should call github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine\\+github.com/newrelic/go-agent/v3/newrelic.NewContext,github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
			ctx = newrelic.NewContext(ctx, txn) // Only second of AND, not OR alt
			_ = ctx
		}()
		_ = ctx
	}()
}

// [GOOD]: Mixed - AND group split between outer and IIFE - SSA detects
//
// SSA traverses into IIFE and correctly detects deriver calls.
func goodMixedSplitDeriversAcrossLevels(ctx context.Context, txn *newrelic.Transaction) {
	go func() { // SSA detects deriver calls
		txn = txn.NewGoroutine() // First of AND
		func() {
			ctx = newrelic.NewContext(ctx, txn) // Second of AND in IIFE - SSA detects
			_ = ctx
		}()
		_ = txn
	}()
}

// [GOOD]: Mixed - OR alternative in nested IIFE - SSA detects
//
// SSA traverses into IIFE and correctly detects OR alternative.
func goodMixedOrAlternativeInNestedIIFE(ctx context.Context) {
	go func() { // SSA detects deriver call in IIFE
		func() {
			ctx = apm.NewGoroutineContext(ctx)
			_ = ctx
		}()
	}()
}

// [BAD]: Mixed - nested 3-level, outer only has first of AND.
//
// Nested pattern where outer only calls first deriver of AND group.
// SSA correctly detects ctx capture at each level, but deriver conditions not met.
func badMixedNested3LevelOuterPartial(ctx context.Context, txn *newrelic.Transaction) {
	go func() { // want `goroutine should call github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine\+github.com/newrelic/go-agent/v3/newrelic.NewContext,github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context`
		txn = txn.NewGoroutine() // Only first of AND
		go func() { // want "goroutine should call github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine\\+github.com/newrelic/go-agent/v3/newrelic.NewContext,github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
			ctx = newrelic.NewContext(ctx, txn) // Only second of AND
			go func() { // want "goroutine should call github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine\\+github.com/newrelic/go-agent/v3/newrelic.NewContext,github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
				_ = ctx // Neither AND nor OR
			}()
			_ = ctx
		}()
		_ = txn
	}()
}

// ===== HIGHER-ORDER PATTERNS =====

// [BAD]: Higher-order go fn()() - only first of AND
//
// Higher-order go fn()() - returned func only has first of AND, not OR alternative.
func badHigherOrderReturnedFuncPartialDeriver(ctx context.Context, txn *newrelic.Transaction) {
	makeWorker := func() func() {
		return func() {
			txn = txn.NewGoroutine() // Only first of AND, not OR alt
			_ = ctx
			_ = txn
		}
	}
	go makeWorker()() // want "goroutine should call github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine\\+github.com/newrelic/go-agent/v3/newrelic.NewContext,github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
}

// ===== VARIABLE REASSIGNMENT =====

// [BAD]: Variable reassignment - last assignment with incomplete derivers should warn.
//
// Last reassigned value has incomplete deriver calls.
//
// See also:
//   goroutinederiveand: badReassignedFuncIncompleteDeriver
func badReassignedFuncIncompleteDeriver(ctx context.Context, txn *newrelic.Transaction) {
	fn := func() {
		txn = txn.NewGoroutine()
		ctx = newrelic.NewContext(ctx, txn)
		_ = ctx
	}
	fn = func() {
		txn = txn.NewGoroutine() // Only first of AND, not OR alt
		_ = ctx
		_ = txn
	}
	go fn() // want "goroutine should call github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine\\+github.com/newrelic/go-agent/v3/newrelic.NewContext,github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
}

// [GOOD]: Variable reassignment - last assignment satisfies OR alternative should pass.
//
// Last reassigned value satisfies OR alternative requirement.
func goodReassignedFuncOrAlternative(ctx context.Context) {
	fn := func() {
		_ = ctx // First assignment has no deriver
	}
	fn = func() {
		ctx = apm.NewGoroutineContext(ctx) // OR alternative
		_ = ctx
	}
	go fn() // OK - last assignment satisfies OR alternative
}
