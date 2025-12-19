// Package waitgroup checks sync.WaitGroup.Go() calls for context propagation.
// Note: sync.WaitGroup.Go() was added in Go 1.25.
package waitgroup

import (
	"go/ast"

	"github.com/mpyw/goroutinectx/internal/context"
	"github.com/mpyw/goroutinectx/internal/directives/ignore"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

const pkgPath = "sync"

// Checker checks sync.WaitGroup.Go() calls for context propagation.
type Checker struct{}

// New creates a new waitgroup checker.
func New() *Checker {
	return &Checker{}
}

// CheckCall implements checkers.CallChecker.
func (c *Checker) CheckCall(cctx *context.CheckContext, call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	// Check for .Go() method (Go 1.25+)
	if sel.Sel.Name != "Go" {
		return
	}

	// Check if receiver is sync.WaitGroup
	if !typeutil.IsNamedType(cctx.Pass, sel.X, pkgPath, "WaitGroup") {
		return
	}

	// sync.WaitGroup.Go() takes a func()
	if len(call.Args) != 1 {
		return
	}

	if !cctx.CheckFuncArgUsesContext(call.Args[0]) {
		cctx.Reportf(ignore.Waitgroup, call.Pos(), "sync.WaitGroup.Go() closure should use context %q", cctx.Scope.Name)
	}
}
