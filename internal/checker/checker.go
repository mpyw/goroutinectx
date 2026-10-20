// Package checker names goroutinectx's checkers. A checker reports under its
// name, and //goroutinectx:ignore selects the reports to silence by it.
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

// Names lists every checker, in the order the report of an unknown name in
// an ignore lists them.
var Names = []Name{Goroutine, GoroutineDerive, Waitgroup, Errgroup, Conc, Spawner, Spawnerlabel, Gotask}
