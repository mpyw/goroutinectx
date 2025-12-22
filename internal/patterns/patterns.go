// Package patterns defines pattern interfaces and types for goroutinectx.
package patterns

import (
	"go/ast"

	"github.com/mpyw/goroutinectx/internal/context"
)

// CheckContext is an alias for context.CheckContext.
type CheckContext = context.CheckContext

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

// GoStmtResult represents the result of a go statement pattern check.
type GoStmtResult struct {
	// OK indicates the pattern is satisfied (no error).
	OK bool
	// DeferOnly indicates the deriver was found but only in defer statements.
	// This is only relevant for deriver patterns.
	DeferOnly bool
}

// GoStmtPattern defines the interface for go statement patterns.
type GoStmtPattern interface {
	// Name returns a human-readable name for the pattern.
	Name() string

	// CheckGoStmt checks if the pattern is satisfied for the given go statement.
	CheckGoStmt(cctx *CheckContext, stmt *ast.GoStmt) GoStmtResult

	// Message returns the diagnostic message when the pattern is violated.
	Message(ctxName string) string

	// DeferMessage returns the diagnostic message when deriver is only in defer.
	// Returns empty string if not applicable.
	DeferMessage(ctxName string) string
}
