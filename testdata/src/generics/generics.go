// Package generics tests calls of generic functions and of methods of
// generic types (#86).
package generics

import (
	"context"

	"github.com/sourcegraph/conc/iter"
)

// Pool is a generic type with a spawner method.
type Pool[T any] struct{}

// Spawn runs fn in a goroutine.
//
//goroutinectx:spawner
func (p *Pool[T]) Spawn(fn func()) { go fn() }

// GSpawn is a generic spawner.
//
//goroutinectx:spawner
func GSpawn[T any](v T, fn func()) { go fn() }

// [BAD]: A spawner method of a generic type.
func badGenericMethod(ctx context.Context) {
	p := &Pool[int]{}
	p.Spawn(func() {}) // want `Spawn\(\) func argument should use context "ctx"`
}

// [BAD]: A generic spawner with inferred type arguments.
func badGenericInferred(ctx context.Context) {
	GSpawn(1, func() {}) // want `GSpawn\(\) func argument should use context "ctx"`
}

// [BAD]: A generic spawner with explicit type arguments.
func badGenericExplicit(ctx context.Context) {
	GSpawn[int](1, func() {}) // want `GSpawn\(\) func argument should use context "ctx"`
}

// [BAD]: A supported API with explicit type arguments.
func badConcExplicit(ctx context.Context) {
	iter.ForEach[int]([]int{1}, func(*int) {}) // want `iter.ForEach\(\) closure should use context "ctx"`
}

// [GOOD]: The same, with a closure that uses ctx.
func goodGenericMethod(ctx context.Context) {
	p := &Pool[int]{}
	p.Spawn(func() { _ = ctx })
	GSpawn[int](1, func() { _ = ctx })
	iter.ForEach[int]([]int{1}, func(*int) { _ = ctx })
}
