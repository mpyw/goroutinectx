// Package lo provides a stub for github.com/samber/lo for testing.
package lo

// Map manipulates a slice and transforms it to a slice of another type.
func Map[T any, R any](collection []T, iteratee func(item T, index int) R) []R {
	result := make([]R, len(collection))
	for i, item := range collection {
		result[i] = iteratee(item, i)
	}
	return result
}

// Filter iterates over elements of collection, returning an array of all elements predicate returns truthy for.
func Filter[T any](collection []T, predicate func(item T, index int) bool) []T {
	var result []T
	for i, item := range collection {
		if predicate(item, i) {
			result = append(result, item)
		}
	}
	return result
}
