// Package stream provides stub types for sourcegraph/conc/stream.
package stream

// Callback is the callback function type returned by tasks.
type Callback func()

// Stream is a stub for stream.Stream.
type Stream struct{}

// New creates a new Stream.
func New() *Stream { return &Stream{} }

// Go submits a task to the stream.
func (*Stream) Go(f func() Callback) {}

// Wait waits for all tasks and callbacks to complete.
func (*Stream) Wait() {}
