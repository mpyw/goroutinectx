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

// CallArgChecker checks function calls that take callback arguments.
type CallArgChecker struct {
	checkerName ignore.CheckerName
	entries     []CallArgEntry
	derivers    *deriver.Matcher
}

// CallArgEntry defines a function that takes a callback argument.
type CallArgEntry struct {
	Spec           funcspec.Spec
	CallbackArgIdx int
}

// NewCallArgChecker creates a new CallArgChecker.
func NewCallArgChecker(name ignore.CheckerName, entries []CallArgEntry, derivers *deriver.Matcher) *CallArgChecker {
	return &CallArgChecker{
		checkerName: name,
		entries:     entries,
		derivers:    derivers,
	}
}

// Name returns the checker name for ignore directive matching.
func (c *CallArgChecker) Name() ignore.CheckerName {
	return c.checkerName
}

// MatchCall returns true if this checker should handle the call.
func (c *CallArgChecker) MatchCall(pass *analysis.Pass, call *ast.CallExpr) bool {
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
func (c *CallArgChecker) CheckCall(cctx *probe.Context, call *ast.CallExpr) *internal.Result {
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

func (c *CallArgChecker) checkSingleArg(cctx *probe.Context, call *ast.CallExpr, entry CallArgEntry) *internal.Result {
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
	return internal.Fail(fmt.Sprintf("%s() closure should use context %q", entry.Spec.FullName(), ctxName))
}

func (c *CallArgChecker) checkArg(cctx *probe.Context, arg ast.Expr) bool {
	if len(cctx.CtxNames) == 0 {
		return true
	}

	// Try SSA-based check first
	if lit, ok := arg.(*ast.FuncLit); ok {
		if result, ok := cctx.FuncLitCapturesContextSSA(lit); ok {
			return result
		}
	}

	// Fall back to AST-based check
	return c.checkArgFromAST(cctx, arg)
}

func (c *CallArgChecker) checkArgFromAST(cctx *probe.Context, arg ast.Expr) bool {
	if lit, ok := arg.(*ast.FuncLit); ok {
		return cctx.FuncLitCapturesContext(lit)
	}

	if ident, ok := arg.(*ast.Ident); ok {
		funcLit := cctx.FuncLitOfIdent(ident)
		if funcLit == nil {
			return true
		}
		return cctx.FuncLitCapturesContext(funcLit)
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

// =============================================================================
// Specific Checker Factories
// =============================================================================

// NewErrgroupChecker creates the errgroup checker.
func NewErrgroupChecker(derivers *deriver.Matcher) *CallArgChecker {
	return NewCallArgChecker(ignore.Errgroup, []CallArgEntry{
		{Spec: funcspec.Spec{PkgPath: "golang.org/x/sync/errgroup", TypeName: "Group", FuncName: "Go"}, CallbackArgIdx: 0},
		{Spec: funcspec.Spec{PkgPath: "golang.org/x/sync/errgroup", TypeName: "Group", FuncName: "TryGo"}, CallbackArgIdx: 0},
	}, derivers)
}

// NewWaitgroupChecker creates the waitgroup checker (Go 1.25+).
func NewWaitgroupChecker(derivers *deriver.Matcher) *CallArgChecker {
	return NewCallArgChecker(ignore.Waitgroup, []CallArgEntry{
		{Spec: funcspec.Spec{PkgPath: "sync", TypeName: "WaitGroup", FuncName: "Go"}, CallbackArgIdx: 0},
	}, derivers)
}

// NewConcChecker creates the conc checker.
func NewConcChecker(derivers *deriver.Matcher) *CallArgChecker {
	return NewCallArgChecker(ignore.Errgroup, []CallArgEntry{
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

	// Report each failing argument at its position
	for _, arg := range funcArgs {
		if !c.checkFuncArg(cctx, arg) {
			cctx.Pass.Reportf(arg.Pos(), "%s() func argument should use context %q", fn.Name(), ctxName)
		}
	}

	// Return OK because we handled reporting ourselves
	return internal.OK()
}

func (c *SpawnerChecker) checkFuncArg(cctx *probe.Context, arg ast.Expr) bool {
	// Try SSA-based check first
	if lit, ok := arg.(*ast.FuncLit); ok {
		if result, ok := cctx.FuncLitCapturesContextSSA(lit); ok {
			return result
		}
		return cctx.FuncLitCapturesContext(lit)
	}

	if ident, ok := arg.(*ast.Ident); ok {
		funcLit := cctx.FuncLitOfIdent(ident)
		if funcLit == nil {
			return true
		}
		return cctx.FuncLitCapturesContext(funcLit)
	}

	if call, ok := arg.(*ast.CallExpr); ok {
		return cctx.FactoryCallReturnsContextUsingFunc(call)
	}

	return true
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
