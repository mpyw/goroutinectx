// A generic named function or factory is judged like its non-generic twin
// (#193).

package generics

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ===== Generic named function given a fresh context =====

// [BAD]: Generic named function given a fresh context
//
// context.Background() carries nothing of ctx.
func badGenericNamedFuncFreshCtxGoroutine(ctx context.Context) {
	go genericWorker(context.Background(), 1) // want `goroutine does not propagate context "ctx"`
}

// [GOOD]: Generic named function given ctx
//
// The function is given ctx.
func goodGenericNamedFuncCtxGoroutine(ctx context.Context) {
	go genericWorker(ctx, "a")
}

// ===== Generic factory given a fresh context =====

// [BAD]: Generic factory given a fresh context
//
// context.Background() carries nothing of ctx.
func badGenericFactoryFreshCtxGoroutine(ctx context.Context) {
	go genericMakeWorker[int](context.Background())() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Generic factory given a fresh context
//
// context.TODO() carries nothing of ctx.
func badGenericFactoryFreshCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(genericMakeErrWorker[int](context.TODO())) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Generic factory given a fresh context
//
// context.Background() carries nothing of ctx.
func badGenericFactoryFreshCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(genericMakeWorker[string](context.Background())) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Generic factory given a fresh context
//
// context.Background() carries nothing of ctx.
func badGenericFactoryFreshCtxSpawner(ctx context.Context) {
	GSpawn(1, genericMakeWorker[int](context.Background())) // want `GSpawn\(\) func argument should use context "ctx"`
}

// [GOOD]: Generic factory given ctx
//
// The factory is given ctx.
func goodGenericFactoryCtx(ctx context.Context) {
	go genericMakeWorker[int](ctx)()
	g := new(errgroup.Group)
	g.Go(genericMakeErrWorker[int](ctx))
	_ = g.Wait()
	var wg sync.WaitGroup
	wg.Go(genericMakeWorker[int](ctx))
	wg.Wait()
	GSpawn(1, genericMakeWorker[int](ctx))
}

// ===== Generic factory without a context =====

// [BAD]: Generic factory without a context
//
// The func the factory returns does not use ctx.
func badGenericFactoryNoCtxGoroutine(ctx context.Context) {
	go genericMakeNoCtx[string]()() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Generic factory without a context
//
// The func the factory returns does not use ctx.
func badGenericFactoryNoCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(genericMakeErrNoCtx[int]()) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Generic factory without a context
//
// The func the factory returns does not use ctx.
func badGenericFactoryNoCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(genericMakeNoCtx[int]()) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Generic factory without a context
//
// The func the factory returns does not use ctx.
func badGenericFactoryNoCtxSpawner(ctx context.Context) {
	GSpawn(1, genericMakeNoCtx[int]()) // want `GSpawn\(\) func argument should use context "ctx"`
}

// [GOOD]: Generic factory whose func captures ctx
//
// The factory is given ctx through a value of its type parameter.
func goodGenericFactoryCapturesCtx(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(genericMakeErrWith(ctx))
	_ = g.Wait()
}

func genericWorker[T any](ctx context.Context, v T) {
	_, _ = ctx, v
}

func genericMakeWorker[T any](ctx context.Context) func() {
	return func() {
		_ = ctx
	}
}

func genericMakeErrWorker[T any](ctx context.Context) func() error {
	return func() error {
		_ = ctx
		return nil
	}
}

func genericMakeNoCtx[T any]() func() {
	return func() {
		fmt.Println("no ctx")
	}
}

func genericMakeErrNoCtx[T any]() func() error {
	return func() error {
		fmt.Println("no ctx")
		return nil
	}
}

func genericMakeErrWith[T any](v T) func() error {
	return func() error {
		_ = v
		return nil
	}
}
