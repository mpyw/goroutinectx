// Package errgroup checks errgroup.Group.Go() calls for context propagation.
package errgroup

import (
	"go/ast"

	"github.com/mpyw/goroutinectx/internal/context"
	"github.com/mpyw/goroutinectx/internal/directives/ignore"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

const pkgPath = "golang.org/x/sync/errgroup"

// Checker checks errgroup.Group.Go() calls for context propagation.
type Checker struct{}

// New creates a new errgroup checker.
func New() *Checker {
	return &Checker{}
}

// CheckCall implements checkers.CallChecker.
func (c *Checker) CheckCall(cctx *context.CheckContext, call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	// Check for .Go() or .TryGo() method
	methodName := sel.Sel.Name
	if methodName != "Go" && methodName != "TryGo" {
		return
	}

	// Check if receiver is errgroup.Group
	if !typeutil.IsNamedType(cctx.Pass, sel.X, pkgPath, "Group") {
		return
	}

	// errgroup.Group.Go() takes a func() error
	if len(call.Args) != 1 {
		return
	}

	if !cctx.CheckFuncArgUsesContext(call.Args[0]) {
		cctx.Reportf(ignore.Errgroup, call.Pos(), "errgroup.Group.%s() closure should use context %q", methodName, cctx.Scope.Name)
	}
}
