// Stub package for testing
package pool

import "context"

// Pool is a pool of goroutines.
type Pool struct{}

// New creates a new Pool.
func New() *Pool { return &Pool{} }

// Go submits a task to be run in the pool.
func (p *Pool) Go(f func()) {}

// Wait blocks until all submitted tasks complete.
func (p *Pool) Wait() {}

// WithMaxGoroutines sets the maximum number of goroutines.
func (p *Pool) WithMaxGoroutines(n int) *Pool { return p }

// WithContext returns a new ContextPool.
func (p *Pool) WithContext(ctx context.Context) *ContextPool { return &ContextPool{} }

// WithErrors returns a new ErrorPool.
func (p *Pool) WithErrors() *ErrorPool { return &ErrorPool{} }

// ErrorPool is a pool that collects errors.
type ErrorPool struct{}

// Go submits a task that may return an error.
func (p *ErrorPool) Go(f func() error) {}

// Wait blocks until all submitted tasks complete.
func (p *ErrorPool) Wait() error { return nil }

// WithMaxGoroutines sets the maximum number of goroutines.
func (p *ErrorPool) WithMaxGoroutines(n int) *ErrorPool { return p }

// WithContext returns a new ContextPool.
func (p *ErrorPool) WithContext(ctx context.Context) *ContextPool { return &ContextPool{} }

// ContextPool is a pool with context propagation.
type ContextPool struct{}

// Go submits a task that takes a context.
func (p *ContextPool) Go(f func(ctx context.Context) error) {}

// Wait blocks until all submitted tasks complete.
func (p *ContextPool) Wait() error { return nil }

// WithMaxGoroutines sets the maximum number of goroutines.
func (p *ContextPool) WithMaxGoroutines(n int) *ContextPool { return p }

// WithCancelOnError configures the pool to cancel on first error.
func (p *ContextPool) WithCancelOnError() *ContextPool { return p }

// WithFirstError configures the pool to return only the first error.
func (p *ContextPool) WithFirstError() *ContextPool { return p }

// ResultPool is a pool that collects results.
type ResultPool[T any] struct{}

// NewWithResults creates a new ResultPool.
func NewWithResults[T any]() *ResultPool[T] { return &ResultPool[T]{} }

// Go submits a task that returns a result.
func (p *ResultPool[T]) Go(f func() T) {}

// Wait blocks and returns all results.
func (p *ResultPool[T]) Wait() []T { return nil }

// WithMaxGoroutines sets the maximum number of goroutines.
func (p *ResultPool[T]) WithMaxGoroutines(n int) *ResultPool[T] { return p }

// ResultErrorPool is a pool that collects results and errors.
type ResultErrorPool[T any] struct{}

// Go submits a task that returns a result and error.
func (p *ResultErrorPool[T]) Go(f func() (T, error)) {}

// Wait blocks and returns all results and errors.
func (p *ResultErrorPool[T]) Wait() ([]T, error) { return nil, nil }

// WithMaxGoroutines sets the maximum number of goroutines.
func (p *ResultErrorPool[T]) WithMaxGoroutines(n int) *ResultErrorPool[T] { return p }

// ResultContextPool is a pool with context that collects results.
type ResultContextPool[T any] struct{}

// Go submits a task that takes context and returns a result and error.
func (p *ResultContextPool[T]) Go(f func(context.Context) (T, error)) {}

// Wait blocks and returns all results and errors.
func (p *ResultContextPool[T]) Wait() ([]T, error) { return nil, nil }

// WithMaxGoroutines sets the maximum number of goroutines.
func (p *ResultContextPool[T]) WithMaxGoroutines(n int) *ResultContextPool[T] { return p }
