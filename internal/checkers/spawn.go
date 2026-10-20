package checkers

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	gossa "golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/checker"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/probe"
	"github.com/mpyw/goroutinectx/internal/registry"
)

// SpawnCallbackChecker checks calls to the supported APIs of one checker
// (errgroup, waitgroup or conc), which spawn their callback argument as a
// goroutine.
type SpawnCallbackChecker struct {
	spawnClosureCheck
	checkerName checker.Name
}

// spawnClosureCheck holds the closure checks that SpawnCallbackChecker and
// SpawnerChecker share.
type spawnClosureCheck struct {
	derivers *deriver.Matcher
}

// NewSpawnCallbackChecker creates the checker of the registry entries of
// name.
func NewSpawnCallbackChecker(name checker.Name, derivers *deriver.Matcher) *SpawnCallbackChecker {
	return &SpawnCallbackChecker{
		spawnClosureCheck: spawnClosureCheck{derivers: derivers},
		checkerName:       name,
	}
}

// Name returns the checker name for ignore directive matching.
func (c *SpawnCallbackChecker) Name() checker.Name {
	return c.checkerName
}

// CheckCall checks the call expression.
func (c *SpawnCallbackChecker) CheckCall(cctx *probe.Context, call *ast.CallExpr) []analysis.Diagnostic {
	fn, recvArgs := spawnedCallee(cctx, call)
	if fn == nil {
		return nil
	}

	entry, ok := registry.MatchFor(c.checkerName, fn)
	if !ok {
		return nil
	}
	return c.checkSingleArg(cctx, call, entry, entry.CallbackArgIdx+recvArgs)
}

// checkSingleArg checks the callback, call's argument at idx.
func (c *SpawnCallbackChecker) checkSingleArg(cctx *probe.Context, call *ast.CallExpr, entry registry.Entry, idx int) []analysis.Diagnostic {
	if idx >= len(call.Args) {
		return nil
	}

	var v gossa.Value
	if args := spawnedCallArgs(cctx, call); args != nil {
		v = args[idx]
	}
	if spawnedFuncPasses(cctx, call.Args[idx], v, spawnedContextResolver(cctx, c.derivers)) {
		return nil
	}

	ctxName := cctx.CtxNames[0]

	// Format error message based on whether deriver is configured
	msg := fmt.Sprintf("%s() closure should use context %q", entry.Spec.FullName(), ctxName)
	if !c.derivers.IsEmpty() {
		msg = fmt.Sprintf("%s() closure should use context %q or call goroutine deriver", entry.Spec.FullName(), ctxName)
	}
	// Report at the callback, as the spawner checker reports at each
	// argument, so that an ignore above the callback silences it.
	return []analysis.Diagnostic{{Pos: call.Args[idx].Pos(), Message: msg}}
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

// NewSpawnerChecker creates a spawner checker.
func NewSpawnerChecker(spawners SpawnerMap, derivers *deriver.Matcher) *SpawnerChecker {
	return &SpawnerChecker{
		spawnClosureCheck: spawnClosureCheck{derivers: derivers},
		spawners:          spawners,
	}
}

// Name returns the checker name.
func (*SpawnerChecker) Name() checker.Name {
	return checker.Spawner
}

// CheckCall checks the call expression. It reports each failing argument at
// its own position.
func (c *SpawnerChecker) CheckCall(cctx *probe.Context, call *ast.CallExpr) []analysis.Diagnostic {
	// Get the function being called
	fn, _ := spawnedCallee(cctx, call)
	if fn == nil || !c.spawners.IsSpawner(fn) {
		return nil
	}

	ctxName := cctx.CtxNames[0]

	// Format error message based on whether deriver is configured
	msgFormat := "%s() func argument should use context %q"
	if !c.derivers.IsEmpty() {
		msgFormat = "%s() func argument should use context %q or call goroutine deriver"
	}

	// Report each failing func argument at its position.
	args := spawnedCallArgs(cctx, call)
	r := spawnedContextResolver(cctx, c.derivers)
	var diags []analysis.Diagnostic
	for i, arg := range call.Args {
		if !spawnIsFuncArg(cctx.Pass, arg) {
			continue
		}
		var v gossa.Value
		if args != nil {
			v = args[i]
		}
		if !spawnedFuncPasses(cctx, arg, v, r) {
			diags = append(diags, analysis.Diagnostic{Pos: arg.Pos(), Message: fmt.Sprintf(msgFormat, fn.Name(), ctxName)})
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
