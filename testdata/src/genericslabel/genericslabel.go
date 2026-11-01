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

// genericHeld holds the funcs GenRunUnion and GenRunNested store, for a
// goroutine to run later.
var genericHeld []any

// GenRunUnion stores f, whose constraint is a union of func types. The
// label stays: f is a func param, and it escapes where the checker cannot
// follow it.
//
//goroutinectx:spawner
func GenRunUnion[F funcConstraint](f F) { genericHeld = append(genericHeld, f) }

// GenNotFunc takes a type parameter that is not a func, so its label is
// unnecessary.
//
//goroutinectx:spawner
func GenNotFunc[T any](v T) { _ = v } // want `function "GenNotFunc" has unnecessary //goroutinectx:spawner directive`

// nestedFuncConstraint embeds a constraint that allows only func types.
type nestedFuncConstraint interface {
	funcConstraint
}

// GenRunNested stores f, whose constraint embeds a func-only one.
//
//goroutinectx:spawner
func GenRunNested[F nestedFuncConstraint](f F) { genericHeld = append(genericHeld, f) }

// GenRunExact runs f, whose constraint is the exact func type.
//
//goroutinectx:spawner
func GenRunExact[F interface{ func() }](f F) { go f() }

// GenOnlyCalls calls f, whose type is a type parameter, in place, so its
// label is unnecessary (#377).
//
//goroutinectx:spawner
func GenOnlyCalls[F ~func()](f F) { f() } // want `function "GenOnlyCalls" has unnecessary //goroutinectx:spawner directive`

// callsGenRun passes its func param to a generic spawner.
func callsGenRun(fn func()) { GenRun(fn) } // want `function "callsGenRun" should have //goroutinectx:spawner directive \(calls GenRun with func argument\)`

// callsGenRunLiteral passes its own literal to a generic spawner (#377).
func callsGenRunLiteral() { GenRun(func() {}) }

// GenNotFuncUnion takes a type parameter that may be a func or an int, so
// its label is unnecessary.
//
//goroutinectx:spawner
func GenNotFuncUnion[T ~func() | ~int](v T) { _ = v } // want `function "GenNotFuncUnion" has unnecessary //goroutinectx:spawner directive`

// AnyParam takes an interface, not a func, so its label is unnecessary.
//
//goroutinectx:spawner
func AnyParam(v any) { _ = v } // want `function "AnyParam" has unnecessary //goroutinectx:spawner directive`
