// Package env holds what the checkers read about the package being analyzed.
// Run builds one [Env] for each pass, before it builds the checkers, and
// gives it to every checker it builds and every node it checks.
package env

import (
	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/carrier"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/directive/spawner"
	"github.com/mpyw/goroutinectx/internal/flow"
)

// Env is what is known about one pass, the same for every node of it.
type Env struct {
	// Pass is the analysis pass.
	Pass *analysis.Pass
	// SSA is the package's SSA program.
	SSA *flow.Program
	// Carriers are -context-carriers.
	Carriers []carrier.Carrier
	// Derivers is -goroutine-deriver, nil when it is not set.
	Derivers *deriver.Matcher
	// Spawners are the functions marked as spawners, and -external-spawner.
	Spawners *spawner.Map
	// Module is the path of the analyzed package's module, or "" when the
	// driver does not give it.
	Module string
}

// New builds the Env of pass.
func New(pass *analysis.Pass, prog *flow.Program, carriers []carrier.Carrier, derivers *deriver.Matcher, spawners *spawner.Map) *Env {
	e := &Env{Pass: pass, SSA: prog, Carriers: carriers, Derivers: derivers, Spawners: spawners}
	if pass != nil && pass.Module != nil {
		e.Module = pass.Module.Path
	}
	return e
}
