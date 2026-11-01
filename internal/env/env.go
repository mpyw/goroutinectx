// Package env holds what the checkers read about the package being analyzed.
// Run builds one [Env] for each pass, before it builds the checkers, and
// gives it to every checker it builds and every node it checks.
package env

import (
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/carrier"
	"github.com/mpyw/goroutinectx/internal/ctxkind"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/directive/spawner"
	"github.com/mpyw/goroutinectx/internal/flow"
	"github.com/mpyw/goroutinectx/internal/site"
)

// Env is what is known about one pass, the same for every node of it.
type Env struct {
	// Pass is the analysis pass.
	Pass *analysis.Pass
	// SSA is the package's SSA program.
	SSA *flow.Program
	// Index gives each go statement and call as a site, and each function
	// literal's SSA function.
	Index *site.Index
	// Kinds tells what counts as a context, with -context-carriers, the
	// analyzed package and its module.
	Kinds *ctxkind.Classifier
	// Derivers is -goroutine-deriver, nil when it is not set.
	Derivers *deriver.Matcher
	// Spawners are the functions marked as spawners, and -external-spawner.
	Spawners *spawner.Map
	// ReportWaiters is -report-waiters: a goroutine that only waits and
	// signals is judged as any other (#230).
	ReportWaiters bool
	// Flow follows SSA values back to where they come from, with one bound
	// on the work of each question and what it found remembered for the
	// pass.
	Flow *flow.Flow
}

// New builds the Env of pass. carriers are -context-carriers, and
// reportWaiters is -report-waiters.
func New(pass *analysis.Pass, prog *flow.Program, carriers []carrier.Carrier, derivers *deriver.Matcher, spawners *spawner.Map, reportWaiters bool) *Env {
	// module is the path of the analyzed package's module, or "" when the
	// driver does not give it.
	module := ""
	var pkg *types.Package
	if pass != nil {
		pkg = pass.Pkg
		if pass.Module != nil {
			module = pass.Module.Path
		}
	}
	return &Env{
		Pass:     pass,
		SSA:      prog,
		Index:    site.NewIndex(pass, prog),
		Kinds:    ctxkind.New(prog.Program, carriers, module, pkg),
		Derivers: derivers,
		Spawners: spawners,
		Flow:     flow.NewFlow(),

		ReportWaiters: reportWaiters,
	}
}
