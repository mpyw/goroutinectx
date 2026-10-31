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

// ResultOk creates a result with a value.
func ResultOk[T any](ok T) Result[T] {
	return Result[T]{Value: ok}
}

// ResultErr creates a result with an error.
func ResultErr[T any](err error) Result[T] {
	return Result[T]{Err: err}
}

// Parallel creates a task that runs each task in a goroutine of its own.
func Parallel[T any](tasks ...Task[Result[T]]) Task[Result[[]T]] {
	return NewTask(func(ctx context.Context) Result[[]T] {
		ch := make(chan Result[T])
		for _, task := range tasks {
			task.DoAsync(ctx, ch)
		}
		values := make([]T, 0, len(tasks))
		for range tasks {
			r := <-ch
			if r.Err != nil {
				return ResultErr[[]T](r.Err)
			}
			values = append(values, r.Value)
		}
		return ResultOk(values)
	})
}

// ParallelSettled creates a task that runs each task in a goroutine of its
// own, and gives every value.
func ParallelSettled[T any](tasks ...Task[T]) Task[[]T] {
	return NewTask(func(ctx context.Context) []T {
		ch := make(chan T)
		for _, task := range tasks {
			task.DoAsync(ctx, ch)
		}
		values := make([]T, 0, len(tasks))
		for range tasks {
			values = append(values, <-ch)
		}
		return values
	})
}

// Race creates a task that runs each task in a goroutine of its own, and
// gives the first value.
func Race[T any](tasks ...Task[T]) Task[T] {
	return NewTask(func(ctx context.Context) T {
		ch := make(chan T)
		for _, task := range tasks {
			task.DoAsync(ctx, ch)
		}
		return <-ch
	})
}

// Then creates a task that runs task, then fn with its value.
func Then[T, U any](task Task[Result[T]], fn func(context.Context, T) Result[U]) Task[Result[U]] {
	return NewTask(func(ctx context.Context) Result[U] {
		r := task.Do(ctx)
		if r.Err != nil {
			return ResultErr[U](r.Err)
		}
		return fn(ctx, r.Value)
	})
}

// Catch creates a task that runs task, then onError if it fails.
func Catch[T any](task Task[Result[T]], onError func(context.Context, error) T) Task[T] {
	return NewTask(func(ctx context.Context) T {
		r := task.Do(ctx)
		if r.Err != nil {
			return onError(ctx, r.Err)
		}
		return r.Value
	})
}
