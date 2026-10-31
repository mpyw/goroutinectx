// Package checkerdef defines goroutinectx's checkers in one table. The
// analyzer's flags, the settings of goroutinectx.New and of the golangci-lint
// plugin, the checkers Run builds, and the names //goroutinectx:ignore
// accepts all come from [Defs].
package checkerdef

import (
	"github.com/mpyw/goroutinectx/internal/checker"
	"github.com/mpyw/goroutinectx/internal/checkers"
	"github.com/mpyw/goroutinectx/internal/checkers/spawnerlabel"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/directive/spawner"
	"github.com/mpyw/goroutinectx/internal/ssa"
)

// Def defines one checker.
type Def struct {
	// Name is the checker's name: its reports' name, and the name of its
	// flag and setting.
	Name checker.Name
	// Default is whether the checker is on when its flag is not set.
	Default bool
	// FlagDoc is the help of the checker's flag. A checker without one has
	// no flag and no setting, and is on whenever it can be built.
	FlagDoc string
	// NeedsDeriver builds the checker only when -goroutine-deriver is set.
	NeedsDeriver bool
	// Build builds the checker. What it returns runs as a go statement, a
	// call or a package checker by the methods it has, which Run tells apart.
	Build func(Deps) Checker
}

// Checker is a built checker.
type Checker interface {
	// Name returns the checker name, which its reports are ignored by.
	Name() checker.Name
}

// Deps holds what Run gives each checker it builds.
type Deps struct {
	// Derivers is -goroutine-deriver, nil when it is not set.
	Derivers *deriver.Matcher
	// Spawners are the functions marked as spawners, and -external-spawner.
	Spawners *spawner.Map
	// SSA is the package's SSA program.
	SSA *ssa.Program
}

// Defs defines every checker. Its order is the order Run builds them in, and
// the order the report of an unknown name in an ignore lists them in.
var Defs = []Def{
	{
		Name: checker.Goroutine, Default: true, FlagDoc: "enable goroutine checker",
		Build: func(Deps) Checker { return &checkers.Goroutine{} },
	},
	{
		Name: checker.GoroutineDerive, NeedsDeriver: true,
		Build: func(d Deps) Checker { return checkers.NewGoroutineDerive(d.Derivers) },
	},
	{
		Name: checker.Waitgroup, Default: true, FlagDoc: "enable waitgroup checker",
		Build: func(d Deps) Checker { return checkers.NewSpawnCallbackChecker(checker.Waitgroup, d.Derivers) },
	},
	{
		Name: checker.Errgroup, Default: true, FlagDoc: "enable errgroup checker",
		Build: func(d Deps) Checker { return checkers.NewSpawnCallbackChecker(checker.Errgroup, d.Derivers) },
	},
	{
		Name: checker.Conc, Default: true, FlagDoc: "enable conc (sourcegraph/conc) checker",
		Build: func(d Deps) Checker { return checkers.NewSpawnCallbackChecker(checker.Conc, d.Derivers) },
	},
	{
		Name: checker.Spawner, Default: true, FlagDoc: "enable spawner checker",
		Build: func(d Deps) Checker { return checkers.NewSpawnerChecker(d.Spawners, d.Derivers) },
	},
	{
		Name: checker.Spawnerlabel, FlagDoc: "enable spawnerlabel checker",
		Build: func(d Deps) Checker { return spawnerlabel.New(d.Spawners, d.SSA) },
	},
	{
		Name: checker.Gotask, Default: true, FlagDoc: "enable gotask checker (requires -goroutine-deriver)", NeedsDeriver: true,
		Build: func(d Deps) Checker { return checkers.NewGotaskChecker(d.Derivers) },
	},
}

// Names lists every checker's name, in the order of [Defs].
var Names = defNames()

// HasFlag reports whether the checker has a flag and a setting.
func (d Def) HasFlag() bool {
	return d.FlagDoc != ""
}

// Settable reports whether name is a checker that has a flag and a setting.
func Settable(name checker.Name) bool {
	for _, d := range Defs {
		if d.Name == name {
			return d.HasFlag()
		}
	}
	return false
}

// defNames lists the names of [Defs].
func defNames() []checker.Name {
	names := make([]checker.Name, len(Defs))
	for i, d := range Defs {
		names[i] = d.Name
	}
	return names
}
