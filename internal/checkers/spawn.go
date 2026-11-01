package checkers

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/checkername"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/env"
	"github.com/mpyw/goroutinectx/internal/registry"
	"github.com/mpyw/goroutinectx/internal/site"
)

// SpawnCallbackChecker checks calls to the supported APIs of one checker
// (errgroup, waitgroup or conc), which spawn their callback argument as a
// goroutine.
type SpawnCallbackChecker struct {
	spawnClosureCheck
	checkerName checkername.Name
}

// spawnClosureCheck holds the closure checks that SpawnCallbackChecker and
// SpawnerChecker share.
type spawnClosureCheck struct {
	derivers *deriver.Matcher
}

// newSpawnCallbackChecker creates the checker of the registry entries of
// name.
//
//declscope:shared // Defs in checkers.go builds the checker with it
func newSpawnCallbackChecker(name checkername.Name, derivers *deriver.Matcher) *SpawnCallbackChecker {
	return &SpawnCallbackChecker{
		spawnClosureCheck: spawnClosureCheck{derivers: derivers},
		checkerName:       name,
	}
}

// Name returns the checker name for ignore directive matching.
func (c *SpawnCallbackChecker) Name() checkername.Name {
	return c.checkerName
}

// CheckCall checks the callback argument of a call to one of the
// checker's APIs.
func (c *SpawnCallbackChecker) CheckCall(e *env.Env, ctxName string, s *site.Site) []analysis.Diagnostic {
	if s.Callee == nil {
		return nil
	}
	entry, ok := registry.MatchFor(c.checkerName, s.Callee)
	if !ok || entry.CallbackArgIdx >= len(s.Params()) {
		return nil
	}
	arg := s.Params()[entry.CallbackArgIdx]
	if e.Index.Judge(e.Flow, newSpawnedContextPolicy(e, c.derivers), arg).Passes() {
		return nil
	}
	// Report at the callback, as the spawner checker reports at each
	// argument, so that an ignore above the callback silences it.
	format := messageContextFormat(c.derivers, messageCallback, messageCallbackDeriver)
	return []analysis.Diagnostic{{Pos: arg.Expr.Pos(), Message: fmt.Sprintf(format, entry.Spec.FullName(), ctxName)}}
}

// =============================================================================
// Spawner Checker
// =============================================================================

// SpawnerChecker checks calls to spawner-marked functions.
type SpawnerChecker struct {
	spawnClosureCheck
	spawners SpawnerMap
}

// SpawnerMap interface for checking if a function is a spawner.
type SpawnerMap interface {
	IsSpawner(fn *types.Func) bool
}

// newSpawnerChecker creates a spawner checker.
//
//declscope:shared // Defs in checkers.go builds the checker with it
func newSpawnerChecker(spawners SpawnerMap, derivers *deriver.Matcher) *SpawnerChecker {
	return &SpawnerChecker{
		spawnClosureCheck: spawnClosureCheck{derivers: derivers},
		spawners:          spawners,
	}
}

// Name returns the checker name.
func (*SpawnerChecker) Name() checkername.Name {
	return checkername.Spawner
}

// CheckCall checks a call to a spawner. It reports each failing func
// argument at its own position.
func (c *SpawnerChecker) CheckCall(e *env.Env, ctxName string, s *site.Site) []analysis.Diagnostic {
	if s.Callee == nil || !c.spawners.IsSpawner(s.Callee) {
		return nil
	}
	format := messageContextFormat(c.derivers, messageSpawner, messageSpawnerDeriver)
	p := newSpawnedContextPolicy(e, c.derivers)
	var diags []analysis.Diagnostic
	for _, arg := range s.Args {
		if spawnIsFuncArg(e.Pass, arg.Expr) && !e.Index.Judge(e.Flow, p, arg).Passes() {
			diags = append(diags, analysis.Diagnostic{Pos: arg.Expr.Pos(), Message: fmt.Sprintf(format, s.Callee.Name(), ctxName)})
		}
	}
	return diags
}

// spawnIsFuncArg reports whether arg is of a func type.
func spawnIsFuncArg(pass *analysis.Pass, arg ast.Expr) bool {
	tv, ok := pass.TypesInfo.Types[arg]
	if !ok {
		return false
	}
	_, isFunc := tv.Type.Underlying().(*types.Signature)
	return isFunc
}
