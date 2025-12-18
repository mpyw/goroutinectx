// Package spawner checks calls to functions marked with
// //goroutinectx:spawner for context propagation.
package spawner

import (
	"go/ast"

	"github.com/mpyw/goroutinectx/internal/context"
	"github.com/mpyw/goroutinectx/internal/directives/spawner"
)

// Checker checks calls to spawner functions.
type Checker struct {
	spawners spawner.Map
}

// New creates a new spawner checker.
func New(spawners spawner.Map) *Checker {
	return &Checker{spawners: spawners}
}

// CheckCall implements checkers.CallChecker.
func (c *Checker) CheckCall(cctx *context.CheckContext, call *ast.CallExpr) {
	if len(c.spawners) == 0 {
		return
	}

	// Get the function being called
	fn := spawner.GetFuncFromCall(cctx.Pass, call)
	if fn == nil {
		return
	}

	// Check if it's a spawner
	if !c.spawners.IsSpawner(fn) {
		return
	}

	// Find func arguments and check each one for context usage
	funcArgs := spawner.FindFuncArgs(cctx.Pass, call)
	for _, arg := range funcArgs {
		if !cctx.CheckFuncArgUsesContext(arg) {
			cctx.Reportf(arg.Pos(), "%s() func argument should use context %q", fn.Name(), cctx.Scope.Name)
		}
	}
}
