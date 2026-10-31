// Package holder has a type of the analyzed module that holds a context in
// an unexported field.
package holder

import "context"

// Service holds the context it is made with.
type Service struct{ ctx context.Context }

// New gives a Service that holds ctx.
func New(ctx context.Context) *Service { return &Service{ctx: ctx} }

// Run uses the context of s.
func (s *Service) Run() { _ = s.ctx.Err() }
