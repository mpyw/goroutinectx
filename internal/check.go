package internal

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/directive/ignore"
	"github.com/mpyw/goroutinectx/internal/probe"
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
	CheckGoStmt(cctx *probe.Context, stmt *ast.GoStmt) *CheckResult
}

// CallChecker checks function call expressions.
type CallChecker interface {
	Checker
	// MatchCall returns true if this checker should handle the call.
	MatchCall(pass *analysis.Pass, call *ast.CallExpr) bool
	// CheckCall checks the call expression.
	CheckCall(cctx *probe.Context, call *ast.CallExpr) *CheckResult
}

// CheckResult represents the outcome of a check.
type CheckResult struct {
	OK       bool   // Check passed
	Message  string // Error message if not OK
	DeferMsg string // Alternative message if only defer has the check
}

// CheckPassed returns a passing result.
func CheckPassed() *CheckResult {
	return &CheckResult{OK: true}
}

// CheckFailed returns a failing result with message.
func CheckFailed(msg string) *CheckResult {
	return &CheckResult{OK: false, Message: msg}
}

// CheckFailedWithDefer returns a failing result with defer-specific message.
func CheckFailedWithDefer(msg, deferMsg string) *CheckResult {
	return &CheckResult{OK: false, Message: msg, DeferMsg: deferMsg}
}
