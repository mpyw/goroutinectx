// Package gotask provides a stub for github.com/siketyan/gotask/v2 for testing.
package gotask

import "context"

// Result represents a result with optional error.
type Result[T any] struct {
	Value T
	Err   error
}

// Task wraps a function for async execution.
type Task[T any] struct {
	fn func(context.Context) T
}

// NewTask creates a new task from a function.
func NewTask[T any](fn func(context.Context) T) Task[T] {
	return Task[T]{fn: fn}
}

// TasksFrom creates multiple tasks from functions.
func TasksFrom[T any](fns ...func(context.Context) T) []Task[T] {
	tasks := make([]Task[T], len(fns))
	for i, fn := range fns {
		tasks[i] = NewTask(fn)
	}
	return tasks
}

// Do executes the task synchronously.
func (t Task[T]) Do(ctx context.Context) T {
	return t.fn(ctx)
}

// DoAsync executes the task asynchronously, sending result to channel.
func (t Task[T]) DoAsync(ctx context.Context, valueChan chan<- T) {
	go func() {
		valueChan <- t.fn(ctx)
	}()
}

// Cancelable wraps the task with cancellation support.
func (t Task[T]) Cancelable() CancelableTask[T] {
	return CancelableTask[T]{Task: t}
}

// CancelableTask is a task that can be cancelled.
type CancelableTask[T any] struct {
	Task[T]
	cancel *context.CancelFunc
}

// Cancel cancels the task if still running.
func (t CancelableTask[T]) Cancel() {
	if t.cancel != nil {
		(*t.cancel)()
	}
}

// DoAll executes all tasks in parallel and returns results.
func DoAll[T any](ctx context.Context, tasks ...Task[Result[T]]) Result[[]T] {
	results := make([]T, len(tasks))
	for i, task := range tasks {
		r := task.Do(ctx)
		if r.Err != nil {
			return Result[[]T]{Err: r.Err}
		}
		results[i] = r.Value
	}
	return Result[[]T]{Value: results}
}

// DoAllFns is a shorthand for DoAll with function arguments.
func DoAllFns[T any](ctx context.Context, fns ...func(context.Context) Result[T]) Result[[]T] {
	return DoAll(ctx, TasksFrom(fns...)...)
}

// DoAllSettled executes all tasks in parallel without stopping on errors.
func DoAllSettled[T any](ctx context.Context, tasks ...Task[T]) []T {
	results := make([]T, len(tasks))
	for i, task := range tasks {
		results[i] = task.Do(ctx)
	}
	return results
}

// DoAllFnsSettled is a shorthand for DoAllSettled with function arguments.
func DoAllFnsSettled[T any](ctx context.Context, fns ...func(context.Context) T) []T {
	return DoAllSettled(ctx, TasksFrom(fns...)...)
}

// DoRace executes tasks and returns the first result.
func DoRace[T any](ctx context.Context, tasks ...Task[T]) T {
	if len(tasks) == 0 {
		var zero T
		return zero
	}
	return tasks[0].Do(ctx)
}

// DoRaceFns is a shorthand for DoRace with function arguments.
func DoRaceFns[T any](ctx context.Context, fns ...func(context.Context) T) T {
	return DoRace(ctx, TasksFrom(fns...)...)
}
