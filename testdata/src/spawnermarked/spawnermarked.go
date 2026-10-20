// Package spawnermarked tests the facts spawner.Analyzer exports.
package spawnermarked

// Run is an exported spawner, so it gets a fact for other packages.
//
//goroutinectx:spawner
func Run(fn func()) { go fn() } // want Run:"spawner"

// run is unexported, so no other package can call it, and it gets no fact.
//
//goroutinectx:spawner
func run(fn func()) { go fn() }

// Pool has a spawner method.
type Pool[T any] struct{}

// Spawn is a spawner method of a generic type.
//
//goroutinectx:spawner
func (p *Pool[T]) Spawn(fn func()) { go fn() } // want Spawn:"spawner"

// Plain has no directive.
func Plain(fn func()) { go fn() }

var _ = run
