// Package ctxworker has functions and factories of another package that take
// a context, for testing how a function of another package is judged by what
// it is given.
package ctxworker

import "context"

// Work uses ctx.
func Work(ctx context.Context) { _ = ctx }

// Make gives a func that uses ctx.
func Make(ctx context.Context) func() {
	return func() { _ = ctx }
}

// MakeErr gives a func that uses ctx.
func MakeErr(ctx context.Context) func() error {
	return func() error { return ctx.Err() }
}

// MakeErrOf is a generic MakeErr.
func MakeErrOf[T any](ctx context.Context, _ T) func() error {
	return func() error { return ctx.Err() }
}

// Deps holds a context the caller sets.
type Deps struct{ Ctx context.Context }

// WorkWithDeps uses the context of d.
func WorkWithDeps(_ context.Context, d *Deps) { _ = d.Ctx }

// MakeWithDeps gives a func that uses the context of d.
func MakeWithDeps(_ context.Context, d *Deps) func() {
	return func() { _ = d.Ctx }
}

// MakeErrWithDeps gives a func that uses the context of d.
func MakeErrWithDeps(_ context.Context, d *Deps) func() error {
	return func() error { return d.Ctx.Err() }
}

// Svc holds a context the caller sets.
type Svc struct{ Ctx context.Context }

// Run uses the context of s.
func (s *Svc) Run(_ context.Context) { _ = s.Ctx }

// Make gives a func that uses the context of s.
func (s *Svc) Make(_ context.Context) func() {
	return func() { _ = s.Ctx }
}

// MakeErr gives a func that uses the context of s.
func (s *Svc) MakeErr(_ context.Context) func() error {
	return func() error { return s.Ctx.Err() }
}

// Plain holds nothing.
type Plain struct{}

// Run uses ctx.
func (Plain) Run(ctx context.Context) { _ = ctx }

// MakeErr gives a func that uses ctx.
func (Plain) MakeErr(ctx context.Context) func() error {
	return func() error { return ctx.Err() }
}
