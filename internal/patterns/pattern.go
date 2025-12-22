// Package patterns defines pattern interfaces and types for goroutinectx.
package patterns

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/directives/carrier"
	internalssa "github.com/mpyw/goroutinectx/internal/ssa"
)

// CheckContext provides context for pattern checking.
type CheckContext struct {
	Pass    *analysis.Pass
	Tracer  *internalssa.Tracer
	SSAProg *internalssa.Program
	// CtxNames holds the context variable names from the enclosing scope (AST-based).
	// This is used when SSA-based context detection fails.
	CtxNames []string
	// Carriers holds the configured context carrier types.
	Carriers []carrier.Carrier
}

// Report reports a diagnostic at the given position.
func (c *CheckContext) Report(pos token.Pos, msg string) {
	c.Pass.Reportf(pos, "%s", msg)
}

// isContextType checks if a type is context.Context.
func isContextType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "context" && obj.Name() == "Context"
}

// Pattern defines the interface for context propagation patterns.
type Pattern interface {
	// Name returns a human-readable name for the pattern.
	Name() string

	// Check checks if the pattern is satisfied for the given call.
	// Returns true if the pattern is satisfied (no error).
	Check(cctx *CheckContext, call *ast.CallExpr, callbackArg ast.Expr) bool

	// Message returns the diagnostic message when the pattern is violated.
	Message(apiName string, ctxName string) string
}
