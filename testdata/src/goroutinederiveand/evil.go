package goroutinederiveand

import (
	"context"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// =============================================================================
// EVIL: AND (plus) - adversarial patterns
// Test flag: -goroutine-deriver=github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+github.com/newrelic/go-agent/v3/newrelic.NewContext
// =============================================================================

// ===== SHOULD NOT REPORT =====

// [GOOD]: AND - Nested 2-level, both have both derivers.
//
// AND - nested 2-level, both have both derivers.
func goodAndNested2LevelBothHaveBothDerivers(ctx context.Context, txn *newrelic.Transaction) {
	go func() {
		txn = txn.NewGoroutine()
		ctx = newrelic.NewContext(ctx, txn)
		go func() {
			txn = txn.NewGoroutine()
			ctx = newrelic.NewContext(ctx, txn)
			_ = ctx
		}()
		_ = ctx
	}()
}

// [GOOD]: AND - Both derivers in different order across conditional branches.
//
// AND - both derivers in different order across conditional branches.
func goodAndDifferentOrderInBranches(ctx context.Context, txn *newrelic.Transaction, cond bool) {
	go func() {
		if cond {
			txn = txn.NewGoroutine()
			ctx = newrelic.NewContext(ctx, txn)
		} else {
			ctx = newrelic.NewContext(ctx, txn)
			txn = txn.NewGoroutine()
		}
		_ = ctx
		_ = txn
	}()
}

// ===== SHOULD REPORT =====

// [BAD]: AND - Nested 2-level, inner missing one deriver.
//
// AND - nested 2-level, inner missing one deriver.
func badAndNested2LevelInnerMissingOneDeriver(ctx context.Context, txn *newrelic.Transaction) {
	go func() {
		txn = txn.NewGoroutine()
		ctx = newrelic.NewContext(ctx, txn)
		go func() { // want "goroutine should call github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine\\+github.com/newrelic/go-agent/v3/newrelic.NewContext to derive context"
			ctx = newrelic.NewContext(ctx, txn) // Missing NewGoroutine
			_ = ctx
		}()
		_ = ctx
	}()
}

// [GOOD]: AND - Both derivers in nested IIFE - SSA detects
//
// SSA traverses into IIFE and correctly detects both deriver calls.
func goodAndBothDeriverInNestedIIFE(ctx context.Context, txn *newrelic.Transaction) {
	go func() { // SSA detects deriver calls in IIFE
		func() {
			txn = txn.NewGoroutine()
			ctx = newrelic.NewContext(ctx, txn)
			_ = ctx
		}()
	}()
}

// [GOOD]: AND - Split derivers across levels - SSA detects
//
// SSA traverses into IIFE and correctly detects deriver calls.
func goodAndSplitDeriversAcrossLevels(ctx context.Context, txn *newrelic.Transaction) {
	go func() { // SSA detects deriver calls
		txn = txn.NewGoroutine() // First deriver at outer level
		func() {
			ctx = newrelic.NewContext(ctx, txn) // Second deriver in IIFE - SSA detects
			_ = ctx
		}()
		_ = txn
	}()
}

// [BAD]: AND - Nested 3-level, outer only has first deriver.
//
// AND - nested 3-level, outer only has first deriver.
// SSA correctly detects ctx capture at each level, but deriver AND conditions not met.
func badAndNested3LevelOuterPartial(ctx context.Context, txn *newrelic.Transaction) {
	go func() { // want `goroutine should call github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine\+github.com/newrelic/go-agent/v3/newrelic.NewContext to derive context`
		txn = txn.NewGoroutine() // Only first deriver
		go func() { // want "goroutine should call github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine\\+github.com/newrelic/go-agent/v3/newrelic.NewContext to derive context"
			ctx = newrelic.NewContext(ctx, txn) // Only second deriver
			go func() { // want "goroutine should call github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine\\+github.com/newrelic/go-agent/v3/newrelic.NewContext to derive context"
				_ = ctx // Neither deriver
			}()
			_ = ctx
		}()
		_ = txn
	}()
}

// ===== HIGHER-ORDER PATTERNS =====

// [BAD]: Higher-order go fn()()
//
// Higher-order go fn()() - returned func only has first deriver.
func badHigherOrderReturnedFuncPartialDeriver(ctx context.Context, txn *newrelic.Transaction) {
	makeWorker := func() func() {
		return func() {
			txn = txn.NewGoroutine() // Only first deriver, missing NewContext
			_ = ctx
			_ = txn
		}
	}
	go makeWorker()() // want "goroutine should call github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine\\+github.com/newrelic/go-agent/v3/newrelic.NewContext to derive context"
}

// ===== VARIABLE REASSIGNMENT =====

// [BAD]: Variable reassignment - last assignment with incomplete derivers should warn.
//
// Last reassigned value has incomplete deriver calls.
//
// See also:
//   goroutinederivemixed: badReassignedFuncIncompleteDeriver
func badReassignedFuncIncompleteDeriver(ctx context.Context, txn *newrelic.Transaction) {
	fn := func() {
		txn = txn.NewGoroutine()
		ctx = newrelic.NewContext(ctx, txn)
		_ = ctx
	}
	fn = func() {
		txn = txn.NewGoroutine() // Only first deriver
		_ = ctx
		_ = txn
	}
	go fn() // want "goroutine should call github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine\\+github.com/newrelic/go-agent/v3/newrelic.NewContext to derive context"
}

// [GOOD]: Variable reassignment - last assignment with both derivers should pass.
//
// Last reassigned value calls both required derivers.
func goodReassignedFuncBothDerivers(ctx context.Context, txn *newrelic.Transaction) {
	fn := func() {
		_ = ctx // First assignment has no deriver
	}
	fn = func() {
		txn = txn.NewGoroutine()
		ctx = newrelic.NewContext(ctx, txn)
		_ = ctx
	}
	go fn() // OK - last assignment has both derivers
}
