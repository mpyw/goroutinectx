// Package iter provides stub types for sourcegraph/conc/iter.
package iter

// ForEach executes f in parallel over each element in input.
func ForEach[T any](input []T, f func(*T)) {}

// ForEachIdx executes f in parallel over each element in input with index.
func ForEachIdx[T any](input []T, f func(int, *T)) {}

// Map applies f to each element of input, returning the mapped result.
func Map[T, R any](input []T, f func(*T) R) []R { return nil }

// MapErr applies f to each element of input with error handling.
func MapErr[T, R any](input []T, f func(*T) (R, error)) ([]R, error) { return nil, nil }

// Iterator is a stub for iter.Iterator[T] (generic).
type Iterator[T any] struct {
	MaxGoroutines int
}

// ForEach executes f in parallel over each element in input.
func (*Iterator[T]) ForEach(input []T, f func(*T)) {}

// ForEachIdx executes f in parallel over each element in input with index.
func (*Iterator[T]) ForEachIdx(input []T, f func(int, *T)) {}

// Mapper is a stub for iter.Mapper[T, R] (generic).
type Mapper[T, R any] struct {
	MaxGoroutines int
}

// Map applies f to each element of input, returning the mapped result.
func (*Mapper[T, R]) Map(input []T, f func(*T) R) []R { return nil }

// MapErr applies f to each element of input with error handling.
func (*Mapper[T, R]) MapErr(input []T, f func(*T) (R, error)) ([]R, error) { return nil, nil }
