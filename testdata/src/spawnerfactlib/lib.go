// Package spawnerfactlib declares spawners that another package calls.
package spawnerfactlib

// Spawn runs fn in a goroutine.
//
//goroutinectx:spawner
func Spawn(fn func()) { go fn() }

// Pool runs functions in goroutines.
type Pool struct{}

// Submit runs fn in a goroutine.
//
//goroutinectx:spawner
func (*Pool) Submit(fn func()) { go fn() }

// Run is not a spawner.
func Run(fn func()) { fn() }
