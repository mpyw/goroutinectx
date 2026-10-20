// Package genericslabel tests -spawnerlabel with generic spawners (#86).
package genericslabel

// Pool is a generic type with a spawner method.
type Pool[T any] struct{}

// Spawn runs fn in a goroutine.
//
//goroutinectx:spawner
func (p *Pool[T]) Spawn(fn func()) { go fn() }

// GSpawn is a generic spawner.
//
//goroutinectx:spawner
func GSpawn[T any](fn func()) { go fn() }

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
func GenRun[F ~func()](f F) { go f() }

// funcConstraint allows func types only.
type funcConstraint interface {
	~func() | ~func() error
}

// GenRunUnion runs f, whose constraint is a union of func types.
//
//goroutinectx:spawner
func GenRunUnion[F funcConstraint](f F) { _ = f }

// GenNotFunc takes a type parameter that is not a func, so its label is
// unnecessary.
//
//goroutinectx:spawner
func GenNotFunc[T any](v T) { _ = v } // want `function "GenNotFunc" has unnecessary //goroutinectx:spawner directive`

// nestedFuncConstraint embeds a constraint that allows only func types.
type nestedFuncConstraint interface {
	funcConstraint
}

// GenRunNested runs f, whose constraint embeds a func-only one.
//
//goroutinectx:spawner
func GenRunNested[F nestedFuncConstraint](f F) { _ = f }

// GenRunExact runs f, whose constraint is the exact func type.
//
//goroutinectx:spawner
func GenRunExact[F interface{ func() }](f F) { _ = f }

// GenNotFuncUnion takes a type parameter that may be a func or an int, so
// its label is unnecessary.
//
//goroutinectx:spawner
func GenNotFuncUnion[T ~func() | ~int](v T) { _ = v } // want `function "GenNotFuncUnion" has unnecessary //goroutinectx:spawner directive`

// AnyParam takes an interface, not a func, so its label is unnecessary.
//
//goroutinectx:spawner
func AnyParam(v any) { _ = v } // want `function "AnyParam" has unnecessary //goroutinectx:spawner directive`
