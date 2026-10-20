package goroutinederiveand

import (
	"context"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// =============================================================================
// BASIC: AND (plus) - all must be called
// Test flag: -goroutine-deriver=github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+github.com/newrelic/go-agent/v3/newrelic.NewContext
// =============================================================================

// ===== SHOULD NOT REPORT =====

// [GOOD]: AND - Calls both functions.
//
// AND - calls both functions.
func goodAndCallsBoth(ctx context.Context, txn *newrelic.Transaction) {
	go func() {
		txn = txn.NewGoroutine()
		ctx = newrelic.NewContext(ctx, txn)
		_ = ctx
	}()
}

// [GOOD]: AND - Calls both in different order.
//
// AND - calls both in different order.
func goodAndCallsBothReversed(ctx context.Context, txn *newrelic.Transaction) {
	go func() {
		ctx = newrelic.NewContext(ctx, txn)
		txn = txn.NewGoroutine()
		_ = ctx
		_ = txn
	}()
}

// [GOOD]: AND - Calls both with other code between.
//
// AND - calls both with other code between.
func goodAndCallsBothInterleaved(ctx context.Context, txn *newrelic.Transaction) {
	go func() {
		txn = txn.NewGoroutine()
		doSomething()
		ctx = newrelic.NewContext(ctx, txn)
		_ = ctx
	}()
}

// [NOTCHECKED]: AND - Has own context param.
//
// AND - has own context param.
func notCheckedAndOwnContextParam(ctx context.Context) {
	go func(ctx context.Context) {
		_ = ctx
	}(ctx)
}

// ===== SHOULD REPORT =====

// [BAD]: AND - Calls only first (method).
//
// AND - calls only first (method).
func badAndCallsOnlyFirst(ctx context.Context, txn *newrelic.Transaction) {
	go func() { // want "goroutine should call github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine\\+github.com/newrelic/go-agent/v3/newrelic.NewContext to derive context"
		txn = txn.NewGoroutine()
		_ = ctx
		_ = txn
	}()
}

// [BAD]: AND - Calls only second (function).
//
// AND - calls only second (function).
func badAndCallsOnlySecond(ctx context.Context, txn *newrelic.Transaction) {
	go func() { // want "goroutine should call github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine\\+github.com/newrelic/go-agent/v3/newrelic.NewContext to derive context"
		ctx = newrelic.NewContext(ctx, txn)
		_ = ctx
	}()
}

// [BAD]: AND - Calls neither function.
//
// AND - calls neither function.
func badAndCallsNeither(ctx context.Context) {
	go func() { // want "goroutine should call github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine\\+github.com/newrelic/go-agent/v3/newrelic.NewContext to derive context"
		_ = ctx
	}()
}

func doSomething() {}

// ===== DERIVERS IN THE GO STATEMENT'S ARGUMENTS (#144) =====

// [GOOD]: AND - NewGoroutine in the go statement's arguments.
//
// The form New Relic documents: NewGoroutine is called for the goroutine
// and passed to it, and NewContext is called in it.
func goodAndNewGoroutineAsArgument(ctx context.Context, txn *newrelic.Transaction) {
	go func(txn *newrelic.Transaction) {
		ctx := newrelic.NewContext(ctx, txn)
		_ = ctx
	}(txn.NewGoroutine())
}

// [GOOD]: AND - Both in the go statement's arguments.
//
// A call nested in an argument counts too.
func goodAndBothAsArguments(ctx context.Context, txn *newrelic.Transaction) {
	go func(ctx context.Context) {
		_ = ctx
	}(newrelic.NewContext(ctx, txn.NewGoroutine()))
}

// [BAD]: AND - Only NewGoroutine, in the go statement's arguments.
//
// NewContext is called nowhere.
func badAndOnlyNewGoroutineAsArgument(ctx context.Context, txn *newrelic.Transaction) {
	go func(txn *newrelic.Transaction) { // want "goroutine should call github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine\\+github.com/newrelic/go-agent/v3/newrelic.NewContext to derive context"
		_ = ctx
		_ = txn
	}(txn.NewGoroutine())
}
