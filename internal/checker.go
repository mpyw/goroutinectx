package internal

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/check"
	"github.com/mpyw/goroutinectx/internal/directive/ignore"
)

// Checker is the unified interface for all checkers.
// Each checker may implement one or more check methods.
type Checker interface {
	// Name returns the checker name for ignore directive matching.
	Name() ignore.CheckerName
}

// GoStmtChecker checks go statements (go func()...).
type GoStmtChecker interface {
	Checker
	CheckGoStmt(cctx *check.Context, stmt *ast.GoStmt) *Result
}

// CallChecker checks function call expressions.
type CallChecker interface {
	Checker
	// MatchCall returns true if this checker should handle the call.
	MatchCall(pass *analysis.Pass, call *ast.CallExpr) bool
	// CheckCall checks the call expression.
	CheckCall(cctx *check.Context, call *ast.CallExpr) *Result
}

// Result represents the outcome of a check.
type Result struct {
	OK       bool   // Check passed
	Message  string // Error message if not OK
	DeferMsg string // Alternative message if only defer has the check
}

// OK returns a passing result.
func OK() *Result {
	return &Result{OK: true}
}

// Fail returns a failing result with message.
func Fail(msg string) *Result {
	return &Result{OK: false, Message: msg}
}

// FailWithDefer returns a failing result with defer-specific message.
func FailWithDefer(msg, deferMsg string) *Result {
	return &Result{OK: false, Message: msg, DeferMsg: deferMsg}
}
