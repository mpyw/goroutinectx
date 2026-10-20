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

// GenRun runs f, whose type is a type parameter, in a goroutine. The label
// is needed: the spawner checker checks its func arguments (#136).
//
//goroutinectx:spawner
func GenRun[F ~func()](f F) { go f() } // want GenRun:"spawner"

// funcConstraint allows func types only.
type funcConstraint interface {
	~func() | ~func() error
}

// GenRunUnion runs f, whose constraint is a union of func types.
//
//goroutinectx:spawner
func GenRunUnion[F funcConstraint](f F) { _ = f } // want GenRunUnion:"spawner"

// GenNotFunc takes a type parameter that is not a func, so its label is
// unnecessary.
//
//goroutinectx:spawner
func GenNotFunc[T any](v T) { _ = v } // want `function "GenNotFunc" has unnecessary //goroutinectx:spawner directive` GenNotFunc:"spawner"
