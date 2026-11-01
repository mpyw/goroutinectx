//declscope:core

package checkers

import (
	"github.com/mpyw/goroutinectx/internal/checkername"
	"github.com/mpyw/goroutinectx/internal/env"
)

// Def defines one checker.
type Def struct {
	// Name is the checker's name: its reports' name, and the name of its
	// flag and setting.
	Name checkername.Name
	// Default is whether the checker is on when its flag is not set.
	Default bool
	// FlagDoc is the help of the checker's flag. A checker without one has
	// no flag and no setting, and is on whenever it can be built.
	FlagDoc string
	// NeedsDeriver builds the checker only when -goroutine-deriver is set.
	NeedsDeriver bool
	// Build builds the checker from the pass's Env. What it returns runs as
	// a go statement, a call or a package checker by the methods it has,
	// which Run tells apart.
	Build func(*env.Env) Checker
}

// Checker is a built checker.
type Checker interface {
	// Name returns the checker name, which its reports are ignored by.
	Name() checkername.Name
}

// Defs defines every checker, in the order of [checkername.All]: the order
// Run builds them in.
var Defs = []Def{
	{
		Name: checkername.Goroutine, Default: true, FlagDoc: "enable goroutine checker",
		Build: func(*env.Env) Checker { return &Goroutine{} },
	},
	{
		Name: checkername.GoroutineDerive, NeedsDeriver: true,
		Build: func(e *env.Env) Checker { return newGoroutineDerive(e.Derivers) },
	},
	{
		Name: checkername.Waitgroup, Default: true, FlagDoc: "enable waitgroup checker",
		Build: func(e *env.Env) Checker { return newSpawnCallbackChecker(checkername.Waitgroup, e.Derivers) },
	},
	{
		Name: checkername.Errgroup, Default: true, FlagDoc: "enable errgroup checker",
		Build: func(e *env.Env) Checker { return newSpawnCallbackChecker(checkername.Errgroup, e.Derivers) },
	},
	{
		Name: checkername.Conc, Default: true, FlagDoc: "enable conc (sourcegraph/conc) checker",
		Build: func(e *env.Env) Checker { return newSpawnCallbackChecker(checkername.Conc, e.Derivers) },
	},
	{
		Name: checkername.Spawner, Default: true, FlagDoc: "enable spawner checker",
		Build: func(e *env.Env) Checker { return newSpawnerChecker(e.Spawners, e.Derivers) },
	},
	{
		Name: checkername.Spawnerlabel, FlagDoc: "enable spawnerlabel checker",
		Build: func(e *env.Env) Checker { return newSpawnerlabelChecker(e.Spawners, e.SSA, e.Flow) },
	},
	{
		Name: checkername.Gotask, Default: true, FlagDoc: "enable gotask checker (requires -goroutine-deriver)", NeedsDeriver: true,
		Build: func(e *env.Env) Checker { return newGotaskChecker(e.Derivers) },
	},
}

// HasFlag reports whether the checker has a flag and a setting.
func (d Def) HasFlag() bool {
	return d.FlagDoc != ""
}

// Settable reports whether name is a checker that has a flag and a setting.
func Settable(name checkername.Name) bool {
	for _, d := range Defs {
		if d.Name == name {
			return d.HasFlag()
		}
	}
	return false
}
