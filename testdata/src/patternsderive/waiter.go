// Goroutines that only wait and signal (#230), with -goroutine-deriver. The
// waiter rule does not change the deriver requirement: each still has to
// call the deriver (#243).

package patternsderive

import (
	"context"
	"sync"

	"github.com/my-example-app/telemetry/apm"
	"golang.org/x/sync/errgroup"
)

// ===== WaitGroup.Wait, then close, without the deriver =====

// [BAD]: WaitGroup.Wait, then close, without the deriver
//
// A go statement must call the deriver.
func badWaiterNoDeriverGoroutine(ctx context.Context) {
	var wg sync.WaitGroup
	done := make(chan struct{})
	go func() { // want "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
		wg.Wait()
		close(done)
	}()
	<-done
}

// [BAD]: WaitGroup.Wait, then close, without the deriver
//
// A callback must capture ctx or call the deriver.
func badWaiterNoDeriverErrgroup(ctx context.Context) error {
	var wg sync.WaitGroup
	g := new(errgroup.Group)
	done := make(chan struct{})
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx" or call goroutine deriver`
		wg.Wait()
		close(done)
		return nil
	})
	<-done
	return g.Wait()
}

// [BAD]: WaitGroup.Wait, then close, without the deriver
//
// A callback must capture ctx or call the deriver.
func badWaiterNoDeriverWaitGroup(ctx context.Context) {
	var inner, outer sync.WaitGroup
	done := make(chan struct{})
	outer.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx" or call goroutine deriver`
		inner.Wait()
		close(done)
	})
	<-done
}

// [BAD]: WaitGroup.Wait, then close, without the deriver
//
// A callback must capture ctx or call the deriver.
func badWaiterNoDeriverSpawner(ctx context.Context) {
	var wg sync.WaitGroup
	g := new(errgroup.Group)
	done := make(chan struct{})
	runWithGroup(g, func() error { // want `runWithGroup\(\) func argument should use context "ctx" or call goroutine deriver`
		wg.Wait()
		close(done)
		return nil
	})
	<-done
}

// ===== WaitGroup.Wait, then close, with the deriver =====

// [GOOD]: WaitGroup.Wait, then close, with the deriver
func goodWaiterCallsDeriverGoroutine(ctx context.Context) {
	var wg sync.WaitGroup
	done := make(chan struct{})
	go func() {
		_ = apm.NewGoroutineContext(ctx)
		wg.Wait()
		close(done)
	}()
	<-done
}

// [GOOD]: WaitGroup.Wait, then close, with the deriver
func goodWaiterCallsDeriverErrgroup(ctx context.Context) error {
	var wg sync.WaitGroup
	g := new(errgroup.Group)
	done := make(chan struct{})
	g.Go(func() error {
		_ = apm.NewGoroutineContext(ctx)
		wg.Wait()
		close(done)
		return nil
	})
	<-done
	return g.Wait()
}

// [GOOD]: WaitGroup.Wait, then close, with the deriver
func goodWaiterCallsDeriverWaitGroup(ctx context.Context) {
	var inner, outer sync.WaitGroup
	done := make(chan struct{})
	outer.Go(func() {
		_ = apm.NewGoroutineContext(ctx)
		inner.Wait()
		close(done)
	})
	<-done
}

// [GOOD]: WaitGroup.Wait, then close, with the deriver
func goodWaiterCallsDeriverSpawner(ctx context.Context) {
	var wg sync.WaitGroup
	g := new(errgroup.Group)
	done := make(chan struct{})
	runWithGroup(g, func() error {
		_ = apm.NewGoroutineContext(ctx)
		wg.Wait()
		close(done)
		return nil
	})
	<-done
}
