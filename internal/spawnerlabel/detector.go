// Package spawnerlabel checks that functions are properly labeled with
// //goroutinectx:spawner when they spawn goroutines.
package spawnerlabel

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/directives/spawner"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Package paths for known spawn methods.
const (
	errgroupPkgPath = "golang.org/x/sync/errgroup"
	syncPkgPath     = "sync"
	gotaskPkgPath   = "github.com/siketyan/gotask"
)

// spawnCallInfo contains information about a detected spawn call.
type spawnCallInfo struct {
	methodName string // e.g., "errgroup.Group.Go", "sync.WaitGroup.Go", "gotask.DoAll"
}

// isSpawnCall checks if a call expression is a spawn call that takes func arguments.
// Returns the spawn info if it's a spawn call with func arguments, nil otherwise.
func isSpawnCall(pass *analysis.Pass, call *ast.CallExpr, spawners *spawner.Map) *spawnCallInfo {
	// Check known spawn methods first
	if info := isKnownSpawnMethod(pass, call); info != nil {
		return info
	}

	// Check spawner-marked functions
	if info := isSpawnerMarkedCall(pass, call, spawners); info != nil {
		return info
	}

	return nil
}

// isKnownSpawnMethod checks for built-in spawn methods (errgroup, waitgroup, gotask).
func isKnownSpawnMethod(pass *analysis.Pass, call *ast.CallExpr) *spawnCallInfo {
	// Check errgroup.Group.Go/TryGo
	if info := isErrgroupSpawnCall(pass, call); info != nil {
		return info
	}

	// Check sync.WaitGroup.Go
	if info := isWaitgroupSpawnCall(pass, call); info != nil {
		return info
	}

	// Check gotask.Do* and DoAsync
	if info := isGotaskSpawnCall(pass, call); info != nil {
		return info
	}

	return nil
}

// isErrgroupSpawnCall checks for errgroup.Group.Go() or TryGo() calls.
func isErrgroupSpawnCall(pass *analysis.Pass, call *ast.CallExpr) *spawnCallInfo {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	methodName := sel.Sel.Name
	if methodName != "Go" && methodName != "TryGo" {
		return nil
	}

	if !typeutil.IsNamedType(pass, sel.X, errgroupPkgPath, "Group") {
		return nil
	}

	// Must have func argument
	if len(call.Args) != 1 {
		return nil
	}

	if !hasFuncArg(pass, call) {
		return nil
	}

	return &spawnCallInfo{methodName: "errgroup.Group." + methodName}
}

// isWaitgroupSpawnCall checks for sync.WaitGroup.Go() calls.
func isWaitgroupSpawnCall(pass *analysis.Pass, call *ast.CallExpr) *spawnCallInfo {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	if sel.Sel.Name != "Go" {
		return nil
	}

	if !typeutil.IsNamedType(pass, sel.X, syncPkgPath, "WaitGroup") {
		return nil
	}

	if len(call.Args) != 1 {
		return nil
	}

	if !hasFuncArg(pass, call) {
		return nil
	}

	return &spawnCallInfo{methodName: "sync.WaitGroup.Go"}
}

// isGotaskSpawnCall checks for gotask.Do* and Task.DoAsync/CancelableTask.DoAsync calls.
func isGotaskSpawnCall(pass *analysis.Pass, call *ast.CallExpr) *spawnCallInfo {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	// Check for gotask.Do* package-level functions
	if strings.HasPrefix(sel.Sel.Name, "Do") {
		if isGotaskPackageCall(pass, sel) {
			// Do* functions need at least 2 args (ctx + task)
			if len(call.Args) >= 2 && hasFuncArgAtIndex(pass, call, 1) {
				return &spawnCallInfo{methodName: "gotask." + sel.Sel.Name}
			}
		}
	}

	// Check for Task.DoAsync / CancelableTask.DoAsync
	if sel.Sel.Name == "DoAsync" {
		if isGotaskTaskType(pass, sel.X) {
			return &spawnCallInfo{methodName: "gotask.Task.DoAsync"}
		}
	}

	return nil
}

// isGotaskPackageCall checks if the selector is a call to gotask package.
func isGotaskPackageCall(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	obj := pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return false
	}

	pkgName, ok := obj.(*types.PkgName)
	if !ok {
		return false
	}

	return strings.HasPrefix(pkgName.Imported().Path(), gotaskPkgPath)
}

// isGotaskTaskType checks if the expression is of type gotask.Task or gotask.CancelableTask.
func isGotaskTaskType(pass *analysis.Pass, expr ast.Expr) bool {
	typ := pass.TypesInfo.TypeOf(expr)
	if typ == nil {
		return false
	}

	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}

	named, ok := typ.(*types.Named)
	if !ok {
		return false
	}

	// Handle generic types
	if origin := named.Origin(); origin != nil {
		named = origin
	}

	pkg := named.Obj().Pkg()
	if pkg == nil || !strings.HasPrefix(pkg.Path(), gotaskPkgPath) {
		return false
	}

	typeName := named.Obj().Name()
	return typeName == "Task" || typeName == "CancelableTask"
}

// isSpawnerMarkedCall checks if the call is to a spawner-marked function with func args.
func isSpawnerMarkedCall(pass *analysis.Pass, call *ast.CallExpr, spawners *spawner.Map) *spawnCallInfo {
	if spawners.Len() == 0 {
		return nil
	}

	fn := spawner.GetFuncFromCall(pass, call)
	if fn == nil {
		return nil
	}

	if !spawners.IsSpawner(fn) {
		return nil
	}

	// Check if there are func arguments
	funcArgs := spawner.FindFuncArgs(pass, call)
	if len(funcArgs) == 0 {
		return nil
	}

	return &spawnCallInfo{methodName: fn.Name()}
}

// hasFuncArg checks if the call has any func-typed argument.
func hasFuncArg(pass *analysis.Pass, call *ast.CallExpr) bool {
	return len(spawner.FindFuncArgs(pass, call)) > 0
}

// hasFuncArgAtIndex checks if the call has a func-typed argument at the given index.
func hasFuncArgAtIndex(pass *analysis.Pass, call *ast.CallExpr, index int) bool {
	if index >= len(call.Args) {
		return false
	}

	tv, ok := pass.TypesInfo.Types[call.Args[index]]
	if !ok {
		return false
	}

	_, isFunc := tv.Type.Underlying().(*types.Signature)
	return isFunc
}

// hasFuncParams checks if a function has func-typed parameters.
func hasFuncParams(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return false
	}

	params := sig.Params()
	for i := 0; i < params.Len(); i++ {
		param := params.At(i)
		paramType := param.Type()

		// Handle variadic parameters: ...func() is represented as []func()
		if slice, ok := paramType.(*types.Slice); ok {
			paramType = slice.Elem()
		}

		if _, isFunc := paramType.Underlying().(*types.Signature); isFunc {
			return true
		}
	}

	return false
}
