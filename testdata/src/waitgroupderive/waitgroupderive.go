package waitgroupderive

import (
	"context"
	"sync"

	"github.com/my-example-app/telemetry/apm"
)

// Test cases for waitgroup checker with -goroutine-deriver flag.
// When deriver is configured, callbacks should either capture context OR call deriver.

// ===== GOOD: Context captured =====

// [GOOD]: Callback captures context
func goodCapturesContext(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = ctx // captures context
	})
}

// ===== GOOD: Deriver called =====

// [GOOD]: Callback calls deriver
func goodCallsDeriver(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		newCtx := apm.NewGoroutineContext(ctx)
		_ = newCtx
	})
}

// [GOOD]: Callback calls deriver without capturing ctx
func goodCallsDeriverOnly(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		newCtx := apm.NewGoroutineContext(context.Background())
		_ = newCtx
	})
}

// ===== BAD: Neither context nor deriver =====

// [BAD]: Callback does not capture context or call deriver
func badNoContextNoDeriver(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx" or call goroutine deriver`
	})
}

// ===== Variable patterns =====

// [GOOD]: Variable callback calls deriver
func goodVariableCallsDeriver(ctx context.Context) {
	var wg sync.WaitGroup
	fn := func() {
		newCtx := apm.NewGoroutineContext(ctx)
		_ = newCtx
	}
	wg.Go(fn)
}

// [BAD]: Variable callback without deriver
func badVariableNoDeriver(ctx context.Context) {
	var wg sync.WaitGroup
	fn := func() {
	}
	wg.Go(fn) // want `sync.WaitGroup.Go\(\) closure should use context "ctx" or call goroutine deriver`
}
