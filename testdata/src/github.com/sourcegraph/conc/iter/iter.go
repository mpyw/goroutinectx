// Stub package for testing
package iter

// ForEach executes f in parallel over each element in input.
func ForEach[T any](input []T, f func(*T)) {}

// ForEachIdx executes f in parallel over each element with index.
func ForEachIdx[T any](input []T, f func(int, *T)) {}

// Map applies f to each element of input in parallel.
func Map[T, R any](input []T, f func(*T) R) []R { return nil }

// MapErr applies f to each element in parallel, collecting errors.
func MapErr[T, R any](input []T, f func(*T) (R, error)) ([]R, error) { return nil, nil }

// Iterator is a configurable parallel iterator.
type Iterator[T any] struct {
	MaxGoroutines int
}

// ForEach executes f in parallel over each element.
func (iter Iterator[T]) ForEach(input []T, f func(*T)) {}

// ForEachIdx executes f in parallel over each element with index.
func (iter Iterator[T]) ForEachIdx(input []T, f func(int, *T)) {}

// Mapper is a configurable parallel mapper.
type Mapper[T, R any] struct {
	MaxGoroutines int
}

// Map applies f to each element in parallel.
func (m Mapper[T, R]) Map(input []T, f func(*T) R) []R { return nil }

// MapErr applies f to each element in parallel, collecting errors.
func (m Mapper[T, R]) MapErr(input []T, f func(*T) (R, error)) ([]R, error) { return nil, nil }
