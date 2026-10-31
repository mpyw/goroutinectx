// Package other is of a module other than the analyzed one, and has a type
// that holds a context in an unexported field.
package other

import "context"

// Hidden holds a context the analyzed module cannot set.
type Hidden struct{ ctx context.Context }

// NewHidden gives a Hidden with a context of its own.
func NewHidden() *Hidden { return &Hidden{ctx: context.Background()} }

// Do uses the context of h.
func (h *Hidden) Do() { _ = h.ctx.Err() }
