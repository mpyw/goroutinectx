// Package ctxholder has types of another package that hold a context, for
// testing which receivers count as holding the caller's context.
package ctxholder

import "context"

// Hidden holds a context the caller cannot set, as *testing.T does.
type Hidden struct{ ctx context.Context }

// NewHidden gives a Hidden with a context of its own.
func NewHidden() *Hidden { return &Hidden{ctx: context.Background()} }

// Run uses the context of h.
func (h *Hidden) Run() error { return h.ctx.Err() }

// Do uses the context of h.
func (h *Hidden) Do() { _ = h.ctx.Err() }

// Open holds a context the caller sets.
type Open struct{ Ctx context.Context }

// Run uses the context of o.
func (o *Open) Run() error { return o.Ctx.Err() }
