// Package spawnerlabel checks that functions are properly labeled with
// //goroutinectx:spawner when they spawn goroutines.
package spawnerlabel

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/directives/spawner"
)

// spawnCallInfo contains information about a detected spawn call.
type spawnCallInfo struct {
	methodName string // e.g., "errgroup.Group.Go", "sync.WaitGroup.Go", "gotask.DoAll"
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
