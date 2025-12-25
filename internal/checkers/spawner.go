package checkers

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/directive/ignore"
	"github.com/mpyw/goroutinectx/internal/funcspec"
	"github.com/mpyw/goroutinectx/internal/probe"
)

// SpawnCallbackChecker checks function calls that take callbacks spawned as goroutines.
type SpawnCallbackChecker struct {
	checkerName ignore.CheckerName
	entries     []SpawnCallbackEntry
	derivers    *deriver.Matcher
}

// SpawnCallbackEntry defines a function that spawns its callback argument as a goroutine.
type SpawnCallbackEntry struct {
	Spec           funcspec.Spec
	CallbackArgIdx int
}

// NewSpawnCallbackChecker creates a new SpawnCallbackChecker.
func NewSpawnCallbackChecker(name ignore.CheckerName, entries []SpawnCallbackEntry, derivers *deriver.Matcher) *SpawnCallbackChecker {
	return &SpawnCallbackChecker{
		checkerName: name,
		entries:     entries,
		derivers:    derivers,
	}
}

// Name returns the checker name for ignore directive matching.
func (c *SpawnCallbackChecker) Name() ignore.CheckerName {
	return c.checkerName
}

// MatchCall returns true if this checker should handle the call.
func (c *SpawnCallbackChecker) MatchCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	fn := funcspec.ExtractFunc(pass, call)
	if fn == nil {
		return false
	}

	for _, entry := range c.entries {
		if entry.Spec.Matches(fn) {
			return true
		}
	}
	return false
}

// CheckCall checks the call expression.
func (c *SpawnCallbackChecker) CheckCall(cctx *probe.Context, call *ast.CallExpr) *internal.Result {
	fn := funcspec.ExtractFunc(cctx.Pass, call)
	if fn == nil {
		return internal.OK()
	}

	for _, entry := range c.entries {
		if !entry.Spec.Matches(fn) {
			continue
		}
		return c.checkSingleArg(cctx, call, entry)
	}

	return internal.OK()
}

func (c *SpawnCallbackChecker) checkSingleArg(cctx *probe.Context, call *ast.CallExpr, entry SpawnCallbackEntry) *internal.Result {
	if entry.CallbackArgIdx >= len(call.Args) {
		return internal.OK()
	}

	arg := call.Args[entry.CallbackArgIdx]
	if c.checkArg(cctx, arg) {
		return internal.OK()
	}

	ctxName := "ctx"
	if len(cctx.CtxNames) > 0 {
		ctxName = cctx.CtxNames[0]
	}

	// Format error message based on whether deriver is configured
	if c.derivers != nil && !c.derivers.IsEmpty() {
		return internal.Fail(fmt.Sprintf("%s() closure should use context %q or call goroutine deriver", entry.Spec.FullName(), ctxName))
	}
	return internal.Fail(fmt.Sprintf("%s() closure should use context %q", entry.Spec.FullName(), ctxName))
}

func (c *SpawnCallbackChecker) checkArg(cctx *probe.Context, arg ast.Expr) bool {
	if len(cctx.CtxNames) == 0 {
		return true
	}

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
func (c *SpawnCallbackChecker) checkFuncLitSSA(cctx *probe.Context, lit *ast.FuncLit) (bool, bool) {
	if cctx.SSAProg == nil || cctx.Tracer == nil {
		return false, false
	}

	// If func lit has context param, it's OK
	if cctx.FuncLitHasContextParam(lit) {
		return true, true
	}

	ssaFn := cctx.SSAProg.FindFuncLit(lit)
	if ssaFn == nil {
		return false, false
	}

	// Check if closure captures context
	if cctx.Tracer.ClosureCapturesContext(ssaFn, cctx.Carriers) {
		return true, true
	}

	// If derivers configured, also check if deriver is called
	if c.derivers != nil && !c.derivers.IsEmpty() {
		result := cctx.Tracer.ClosureCallsDeriver(ssaFn, c.derivers)
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
		funcLit := cctx.FuncLitOfIdent(ident)
		if funcLit == nil {
			return true
		}
		return c.checkFuncLitAST(cctx, funcLit)
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

// checkFuncLitAST checks a func literal using AST-based analysis.
func (c *SpawnCallbackChecker) checkFuncLitAST(cctx *probe.Context, lit *ast.FuncLit) bool {
	// Check context capture
	if cctx.FuncLitCapturesContext(lit) {
		return true
	}

	// If derivers configured, also check if deriver is called
	if c.derivers != nil && !c.derivers.IsEmpty() {
		if c.derivers.SatisfiesAnyGroup(cctx.Pass, lit.Body) {
			return true
		}
	}

	return false
}

// =============================================================================
// Specific Checker Factories
// =============================================================================

// NewErrgroupChecker creates the errgroup checker.
func NewErrgroupChecker(derivers *deriver.Matcher) *SpawnCallbackChecker {
	return NewSpawnCallbackChecker(ignore.Errgroup, []SpawnCallbackEntry{
		{Spec: funcspec.Spec{PkgPath: "golang.org/x/sync/errgroup", TypeName: "Group", FuncName: "Go"}, CallbackArgIdx: 0},
		{Spec: funcspec.Spec{PkgPath: "golang.org/x/sync/errgroup", TypeName: "Group", FuncName: "TryGo"}, CallbackArgIdx: 0},
	}, derivers)
}

// NewWaitgroupChecker creates the waitgroup checker (Go 1.25+).
func NewWaitgroupChecker(derivers *deriver.Matcher) *SpawnCallbackChecker {
	return NewSpawnCallbackChecker(ignore.Waitgroup, []SpawnCallbackEntry{
		{Spec: funcspec.Spec{PkgPath: "sync", TypeName: "WaitGroup", FuncName: "Go"}, CallbackArgIdx: 0},
	}, derivers)
}

// NewConcChecker creates the conc checker.
func NewConcChecker(derivers *deriver.Matcher) *SpawnCallbackChecker {
	return NewSpawnCallbackChecker(ignore.Errgroup, []SpawnCallbackEntry{
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
	spawners SpawnerMap
	derivers *deriver.Matcher
}

// SpawnerMap interface for checking if a function is a spawner.
type SpawnerMap interface {
	IsSpawner(fn *types.Func) bool
}

// NewSpawnerChecker creates a spawner checker.
func NewSpawnerChecker(spawners SpawnerMap, derivers *deriver.Matcher) *SpawnerChecker {
	return &SpawnerChecker{
		spawners: spawners,
		derivers: derivers,
	}
}

// Name returns the checker name.
func (*SpawnerChecker) Name() ignore.CheckerName {
	return ignore.Spawner
}

// MatchCall returns true if this checker should handle the call.
func (c *SpawnerChecker) MatchCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	fn := funcspec.ExtractFunc(pass, call)
	return fn != nil && c.spawners.IsSpawner(fn)
}

// CheckCall checks the call expression.
// Note: This checker reports directly to pass because it may have multiple failing arguments.
func (c *SpawnerChecker) CheckCall(cctx *probe.Context, call *ast.CallExpr) *internal.Result {
	if len(cctx.CtxNames) == 0 {
		return internal.OK()
	}

	// Get the function being called
	fn := funcspec.ExtractFunc(cctx.Pass, call)
	if fn == nil {
		return internal.OK()
	}

	// Find func-typed arguments
	funcArgs := findFuncArgs(cctx.Pass, call)
	if len(funcArgs) == 0 {
		return internal.OK()
	}

	ctxName := "ctx"
	if len(cctx.CtxNames) > 0 {
		ctxName = cctx.CtxNames[0]
	}

	// Format error message based on whether deriver is configured
	msgFormat := "%s() func argument should use context %q"
	if c.derivers != nil && !c.derivers.IsEmpty() {
		msgFormat = "%s() func argument should use context %q or call goroutine deriver"
	}

	// Report each failing argument at its position
	for _, arg := range funcArgs {
		if !c.checkFuncArg(cctx, arg) {
			cctx.Pass.Reportf(arg.Pos(), msgFormat, fn.Name(), ctxName)
		}
	}

	// Return OK because we handled reporting ourselves
	return internal.OK()
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
		funcLit := cctx.FuncLitOfIdent(ident)
		if funcLit == nil {
			return true
		}
		return c.checkFuncLitAST(cctx, funcLit)
	}

	if call, ok := arg.(*ast.CallExpr); ok {
		return cctx.FactoryCallReturnsContextUsingFunc(call)
	}

	return true
}

// checkFuncLitSSA checks a func literal using SSA analysis for SpawnerChecker.
func (c *SpawnerChecker) checkFuncLitSSA(cctx *probe.Context, lit *ast.FuncLit) (bool, bool) {
	if cctx.SSAProg == nil || cctx.Tracer == nil {
		return false, false
	}

	if cctx.FuncLitHasContextParam(lit) {
		return true, true
	}

	ssaFn := cctx.SSAProg.FindFuncLit(lit)
	if ssaFn == nil {
		return false, false
	}

	if cctx.Tracer.ClosureCapturesContext(ssaFn, cctx.Carriers) {
		return true, true
	}

	if c.derivers != nil && !c.derivers.IsEmpty() {
		result := cctx.Tracer.ClosureCallsDeriver(ssaFn, c.derivers)
		if result.FoundAtStart {
			return true, true
		}
	}

	return false, true
}

// checkFuncLitAST checks a func literal using AST analysis for SpawnerChecker.
func (c *SpawnerChecker) checkFuncLitAST(cctx *probe.Context, lit *ast.FuncLit) bool {
	if cctx.FuncLitCapturesContext(lit) {
		return true
	}

	if c.derivers != nil && !c.derivers.IsEmpty() {
		if c.derivers.SatisfiesAnyGroup(cctx.Pass, lit.Body) {
			return true
		}
	}

	return false
}

// findFuncArgs finds all arguments in a call that are func types.
func findFuncArgs(pass *analysis.Pass, call *ast.CallExpr) []ast.Expr {
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
