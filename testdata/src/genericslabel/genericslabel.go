// Package genericslabel tests -spawnerlabel with generic spawners (#86).
package genericslabel

// Pool is a generic type with a spawner method.
type Pool[T any] struct{}

// Spawn runs fn in a goroutine.
//
//goroutinectx:spawner
func (p *Pool[T]) Spawn(fn func()) { go fn() } // want Spawn:"spawner"

// GSpawn is a generic spawner.
//
//goroutinectx:spawner
func GSpawn[T any](fn func()) { go fn() } // want GSpawn:"spawner"

func callsMethod(fn func()) { // want `function "callsMethod" should have //goroutinectx:spawner directive \(calls Spawn with func argument\)`
	p := &Pool[int]{}
	p.Spawn(fn)
}

func callsExplicit(fn func()) { // want `function "callsExplicit" should have //goroutinectx:spawner directive \(calls GSpawn with func argument\)`
	GSpawn[int](fn)
}
