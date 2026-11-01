// Package checkername names goroutinectx's checkers. A checker reports under
// its name, and //goroutinectx:ignore selects the reports to silence by it.
// The checkers themselves are defined in internal/checkers.
//
// It holds names only, so the directive packages and the registry can
// import it without importing the checkers.
package checkername

// Name is the name of a checker.
type Name string

// The checkers' names.
const (
	Goroutine       Name = "goroutine"
	GoroutineDerive Name = "goroutinederive"
	Waitgroup       Name = "waitgroup"
	Errgroup        Name = "errgroup"
	Spawner         Name = "spawner"
	Spawnerlabel    Name = "spawnerlabel"
	Gotask          Name = "gotask"
	Conc            Name = "conc"
)

// All lists every checker's name, in the order of checkers.Defs: the order
// Run builds them in, and the order the report of an unknown name in an
// ignore lists them in.
var All = []Name{
	Goroutine,
	GoroutineDerive,
	Waitgroup,
	Errgroup,
	Conc,
	Spawner,
	Spawnerlabel,
	Gotask,
}
