// Package checker names goroutinectx's checkers. A checker reports under its
// name, and //goroutinectx:ignore selects the reports to silence by it. The
// checkers themselves are defined in internal/checkerdef.
package checker

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
