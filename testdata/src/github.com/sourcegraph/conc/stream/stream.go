// Stub package for testing
package stream

// Callback is executed sequentially after a task completes.
type Callback func()

// Task is a function that returns a callback.
type Task func() Callback

// Stream processes tasks concurrently with ordered callbacks.
type Stream struct{}

// New creates a new Stream.
func New() *Stream { return &Stream{} }

// Go submits a task to be run in the stream's pool.
func (s *Stream) Go(f Task) {}

// Wait blocks until all tasks and callbacks complete.
func (s *Stream) Wait() {}

// WithMaxGoroutines sets the maximum number of goroutines.
func (s *Stream) WithMaxGoroutines(n int) *Stream { return s }
