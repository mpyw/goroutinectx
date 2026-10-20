// Generic methods (Go 1.27) have a receiver and a type parameter list of their
// own, so these cases pin down that scope detection and diagnostics behave
// exactly as they do for ordinary methods.

package generics

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

type Repo struct{}

// Cache is a generic type, so its methods may add type parameters of their own.
type Cache[K comparable] struct {
	keys []K
}

// ===== SHOULD REPORT =====

// [BAD]: Generic method whose goroutine drops the context
func (r *Repo) badGenericMethod[T any](ctx context.Context, v T) {
	go func() { // want `goroutine does not propagate context "ctx"`
		fmt.Println(v)
	}()
}

// [BAD]: Generic method on a generic type whose goroutine drops the context
func (c *Cache[K]) badGenericMethodOnGenericType[V any](ctx context.Context, v V) {
	go func() { // want `goroutine does not propagate context "ctx"`
		fmt.Println(len(c.keys), v)
	}()
}

// [BAD]: errgroup callback inside a generic method drops the context
func (r *Repo) badGenericMethodErrgroup[T any](ctx context.Context, v T) error {
	var g errgroup.Group
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		fmt.Println(v)
		return nil
	})
	return g.Wait()
}

// ===== SHOULD NOT REPORT =====

// [GOOD]: Generic method whose goroutine captures the context
func (r *Repo) goodGenericMethod[T any](ctx context.Context, v T) {
	go func() {
		_ = ctx.Err()
		fmt.Println(v)
	}()
}

// [GOOD]: Generic method on a generic type whose goroutine captures the context
func (c *Cache[K]) goodGenericMethodOnGenericType[V any](ctx context.Context, v V) {
	go func() {
		_ = ctx.Err()
		fmt.Println(len(c.keys), v)
	}()
}

// [GOOD]: errgroup callback inside a generic method captures the context
func (r *Repo) goodGenericMethodErrgroup[T any](ctx context.Context, v T) error {
	var g errgroup.Group
	g.Go(func() error {
		_ = ctx.Err()
		fmt.Println(v)
		return nil
	})
	return g.Wait()
}

// [GOOD]: Ignore directive suppresses the report inside a generic method
func (r *Repo) goodGenericMethodIgnored[T any](ctx context.Context, v T) {
	//goroutinectx:ignore goroutine
	go func() {
		fmt.Println(v)
	}()
}

// [GOOD]: Generic method with no context in scope
func (r *Repo) goodGenericMethodNoCtx[T any](v T) {
	go func() {
		fmt.Println(v)
	}()
}
