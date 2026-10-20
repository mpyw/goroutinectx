package checkers

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/checker"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/funcspec"
	"github.com/mpyw/goroutinectx/internal/probe"
	"github.com/mpyw/goroutinectx/internal/ssa"
)

// SpawnCallbackChecker checks function calls that take callbacks spawned as goroutines.
type SpawnCallbackChecker struct {
	spawnClosureCheck
	checkerName checker.Name
	entries     []SpawnCallbackEntry
}

// SpawnCallbackEntry defines a function that spawns its callback argument as a goroutine.
type SpawnCallbackEntry struct {
	Spec           funcspec.Spec
	CallbackArgIdx int
}

// spawnClosureCheck holds the closure checks that SpawnCallbackChecker and
// SpawnerChecker share.
type spawnClosureCheck struct {
	derivers *deriver.Matcher
}

// newSpawnCallbackChecker creates a new SpawnCallbackChecker.
func newSpawnCallbackChecker(name checker.Name, entries []SpawnCallbackEntry, derivers *deriver.Matcher) *SpawnCallbackChecker {
	return &SpawnCallbackChecker{
		spawnClosureCheck: spawnClosureCheck{derivers: derivers},
		checkerName:       name,
		entries:           entries,
	}
}

// Name returns the checker name for ignore directive matching.
func (c *SpawnCallbackChecker) Name() checker.Name {
	return c.checkerName
}

// MatchCall returns true if this checker should handle the call.
func (c *SpawnCallbackChecker) MatchCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	fn := funcspec.ExtractFunc(pass, call)
	if fn == nil {
		return false
	}

	return slices.ContainsFunc(c.entries, func(entry SpawnCallbackEntry) bool { return entry.Spec.Matches(fn) })
}

// CheckCall checks the call expression.
func (c *SpawnCallbackChecker) CheckCall(cctx *probe.Context, call *ast.CallExpr) []analysis.Diagnostic {
	fn := funcspec.ExtractFunc(cctx.Pass, call)
	if fn == nil {
		return nil
	}

	for _, entry := range c.entries {
		if !entry.Spec.Matches(fn) {
			continue
		}
		return c.checkSingleArg(cctx, call, entry)
	}

	return nil
}

func (c *SpawnCallbackChecker) checkSingleArg(cctx *probe.Context, call *ast.CallExpr, entry SpawnCallbackEntry) []analysis.Diagnostic {
	if entry.CallbackArgIdx >= len(call.Args) {
		return nil
	}

	arg := call.Args[entry.CallbackArgIdx]
	if c.checkArg(cctx, arg) {
		return nil
	}

	ctxName := cctx.CtxNames[0]

	// Format error message based on whether deriver is configured
	msg := fmt.Sprintf("%s() closure should use context %q", entry.Spec.FullName(), ctxName)
	if !c.derivers.IsEmpty() {
		msg = fmt.Sprintf("%s() closure should use context %q or call goroutine deriver", entry.Spec.FullName(), ctxName)
	}
	return []analysis.Diagnostic{{Pos: spawnReportPos(call), Message: msg}}
}

// spawnReportPos gives the position to report a call at: its method name
// when it has one.
func spawnReportPos(call *ast.CallExpr) token.Pos {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		return sel.Sel.Pos()
	}
	return call.Pos()
}

func (c *SpawnCallbackChecker) checkArg(cctx *probe.Context, arg ast.Expr) bool {
	// Try SSA-based check first
	if lit, ok := arg.(*ast.FuncLit); ok {
		if result, ok := c.checkFuncLitSSA(cctx, lit); ok {
			return result
		}
	}

	// Fall back to AST-based check
	return c.checkArgFromAST(cctx, arg)
}

// checkFuncLitSSA checks a func literal using SSA analysis.
// Returns (result, true) if SSA succeeded, or (false, false) if SSA failed.
func (c *spawnClosureCheck) checkFuncLitSSA(cctx *probe.Context, lit *ast.FuncLit) (bool, bool) {
	// If func lit has context param, it's OK
	if cctx.FuncLitHasContextParam(lit) {
		return true, true
	}

	ssaFn := cctx.SSAProg.FindFuncLit(lit)
	if ssaFn == nil {
		return false, false
	}

	// Check if closure captures context
	if ssa.ClosureCapturesContext(ssaFn, cctx.Carriers) {
		return true, true
	}

	// If derivers configured, also check if deriver is called
	if !c.derivers.IsEmpty() {
		result := ssa.ClosureCallsDeriver(ssaFn, c.derivers)
		if result.FoundAtStart {
			return true, true
		}
	}

	return false, true
}

func (c *SpawnCallbackChecker) checkArgFromAST(cctx *probe.Context, arg ast.Expr) bool {
	if lit, ok := arg.(*ast.FuncLit); ok {
		return c.checkFuncLitAST(cctx, lit)
	}

	if ident, ok := arg.(*ast.Ident); ok {
		assigns := cctx.FuncLitAssignmentsOfIdent(ident)
		if len(assigns) == 0 {
			return true
		}
		return c.checkFuncLitAssignments(cctx, assigns)
	}

	if call, ok := arg.(*ast.CallExpr); ok {
		return cctx.FactoryCallReturnsContextUsingFunc(call)
	}

	if sel, ok := arg.(*ast.SelectorExpr); ok {
		return cctx.SelectorExprCapturesContext(sel)
	}

	if idx, ok := arg.(*ast.IndexExpr); ok {
		return cctx.IndexExprCapturesContext(idx)
	}

	return true
}

// checkFuncLitAssignments checks all func literal assignments from last unconditional onwards.
// ALL must pass for the check to succeed.
func (c *spawnClosureCheck) checkFuncLitAssignments(cctx *probe.Context, assigns []probe.FuncLitAssignment) bool {
	// ALL must pass (because conditional assignments may override)
	for _, assign := range probe.EffectiveFuncLitAssignments(assigns) {
		if !c.checkFuncLitAST(cctx, assign.Lit) {
			return false
		}
	}
	return true
}

// checkFuncLitAST checks a func literal using AST-based analysis.
func (c *spawnClosureCheck) checkFuncLitAST(cctx *probe.Context, lit *ast.FuncLit) bool {
	// Check context capture
	if cctx.FuncLitCapturesContext(lit) {
		return true
	}

	// If derivers configured, also check if deriver is called
	if !c.derivers.IsEmpty() {
		if c.derivers.SatisfiesAnyGroup(cctx.Pass, lit.Body) {
			return true
		}
	}

	return false
}

// =============================================================================
// Specific Checker Factories
// =============================================================================

// NewErrgroupSpawnChecker creates the errgroup checker.
func NewErrgroupSpawnChecker(derivers *deriver.Matcher) *SpawnCallbackChecker {
	return newSpawnCallbackChecker(checker.Errgroup, []SpawnCallbackEntry{
		{Spec: funcspec.Spec{PkgPath: "golang.org/x/sync/errgroup", TypeName: "Group", FuncName: "Go"}, CallbackArgIdx: 0},
		{Spec: funcspec.Spec{PkgPath: "golang.org/x/sync/errgroup", TypeName: "Group", FuncName: "TryGo"}, CallbackArgIdx: 0},
	}, derivers)
}

// NewWaitgroupSpawnChecker creates the waitgroup checker (Go 1.25+).
func NewWaitgroupSpawnChecker(derivers *deriver.Matcher) *SpawnCallbackChecker {
	return newSpawnCallbackChecker(checker.Waitgroup, []SpawnCallbackEntry{
		{Spec: funcspec.Spec{PkgPath: "sync", TypeName: "WaitGroup", FuncName: "Go"}, CallbackArgIdx: 0},
	}, derivers)
}

// NewConcSpawnChecker creates the conc checker.
func NewConcSpawnChecker(derivers *deriver.Matcher) *SpawnCallbackChecker {
	return newSpawnCallbackChecker(checker.Conc, []SpawnCallbackEntry{
		// conc.Pool.Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc", TypeName: "Pool", FuncName: "Go"}, CallbackArgIdx: 0},
		// conc.WaitGroup.Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc", TypeName: "WaitGroup", FuncName: "Go"}, CallbackArgIdx: 0},
		// pool.Pool.Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "Pool", FuncName: "Go"}, CallbackArgIdx: 0},
		// pool.ResultPool[T].Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultPool", FuncName: "Go"}, CallbackArgIdx: 0},
		// pool.ContextPool.Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ContextPool", FuncName: "Go"}, CallbackArgIdx: 0},
		// pool.ResultContextPool[T].Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultContextPool", FuncName: "Go"}, CallbackArgIdx: 0},
		// pool.ErrorPool.Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ErrorPool", FuncName: "Go"}, CallbackArgIdx: 0},
		// pool.ResultErrorPool[T].Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultErrorPool", FuncName: "Go"}, CallbackArgIdx: 0},
		// stream.Stream.Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/stream", TypeName: "Stream", FuncName: "Go"}, CallbackArgIdx: 0},
		// iter.ForEach
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", FuncName: "ForEach"}, CallbackArgIdx: 1},
		// iter.ForEachIdx
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", FuncName: "ForEachIdx"}, CallbackArgIdx: 1},
		// iter.Map
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", FuncName: "Map"}, CallbackArgIdx: 1},
		// iter.MapErr
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", FuncName: "MapErr"}, CallbackArgIdx: 1},
		// iter.Iterator.ForEach
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", TypeName: "Iterator", FuncName: "ForEach"}, CallbackArgIdx: 1},
		// iter.Iterator.ForEachIdx
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", TypeName: "Iterator", FuncName: "ForEachIdx"}, CallbackArgIdx: 1},
		// iter.Mapper.Map
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", TypeName: "Mapper", FuncName: "Map"}, CallbackArgIdx: 1},
		// iter.Mapper.MapErr
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", TypeName: "Mapper", FuncName: "MapErr"}, CallbackArgIdx: 1},
	}, derivers)
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

// MatchCall returns true if this checker should handle the call.
func (c *SpawnerChecker) MatchCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	fn := funcspec.ExtractFunc(pass, call)
	return fn != nil && c.spawners.IsSpawner(fn)
}

// CheckCall checks the call expression. It reports each failing argument at
// its own position.
func (c *SpawnerChecker) CheckCall(cctx *probe.Context, call *ast.CallExpr) []analysis.Diagnostic {
	// Get the function being called
	fn := funcspec.ExtractFunc(cctx.Pass, call)
	if fn == nil {
		return nil
	}

	// Find func-typed arguments
	funcArgs := findSpawnableFuncArgs(cctx.Pass, call)
	if len(funcArgs) == 0 {
		return nil
	}

	ctxName := cctx.CtxNames[0]

	// Format error message based on whether deriver is configured
	msgFormat := "%s() func argument should use context %q"
	if !c.derivers.IsEmpty() {
		msgFormat = "%s() func argument should use context %q or call goroutine deriver"
	}

	// Report each failing argument at its position
	var diags []analysis.Diagnostic
	for _, arg := range funcArgs {
		if !c.checkFuncArg(cctx, arg) {
			diags = append(diags, analysis.Diagnostic{Pos: arg.Pos(), Message: fmt.Sprintf(msgFormat, fn.Name(), ctxName)})
		}
	}
	return diags
}

func (c *SpawnerChecker) checkFuncArg(cctx *probe.Context, arg ast.Expr) bool {
	// Try SSA-based check first
	if lit, ok := arg.(*ast.FuncLit); ok {
		if result, ok := c.checkFuncLitSSA(cctx, lit); ok {
			return result
		}
		return c.checkFuncLitAST(cctx, lit)
	}

	if ident, ok := arg.(*ast.Ident); ok {
		assigns := cctx.FuncLitAssignmentsOfIdent(ident)
		if len(assigns) == 0 {
			return true
		}
		return c.checkFuncLitAssignments(cctx, assigns)
	}

	if call, ok := arg.(*ast.CallExpr); ok {
		return cctx.FactoryCallReturnsContextUsingFunc(call)
	}

	return true
}

// findSpawnableFuncArgs finds all arguments in a call that are func types.
func findSpawnableFuncArgs(pass *analysis.Pass, call *ast.CallExpr) []ast.Expr {
	var funcArgs []ast.Expr

	for _, arg := range call.Args {
		tv, ok := pass.TypesInfo.Types[arg]
		if !ok {
			continue
		}

		if _, isFunc := tv.Type.Underlying().(*types.Signature); isFunc {
			funcArgs = append(funcArgs, arg)
		}
	}

	return funcArgs
}
