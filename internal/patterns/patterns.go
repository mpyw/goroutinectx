// Package patterns defines pattern interfaces and types for goroutinectx.
package patterns

import (
	"go/ast"
	"strings"

	"github.com/mpyw/goroutinectx/internal/context"
)

// TaskReceiverIdx is a special index value indicating the receiver of a method call.
// Used in TaskConstructor or API definitions when the task/callback comes from
// the method receiver (e.g., task.DoAsync() where task is the receiver).
const TaskReceiverIdx = -1

// TaskConstructor defines how tasks are created for task-based APIs.
// Used to trace back from executor APIs (e.g., DoAsync) to find the actual callback.
//
// Example: gotask.NewTask(fn) creates a Task, which is later executed via task.DoAsync(ctx).
// The TaskConstructor for DoAsync would point to NewTask so we can find fn.
type TaskConstructor struct {
	// Pkg is the package path (e.g., "github.com/siketyan/gotask")
	Pkg string

	// Type is the receiver type name (empty for package-level functions)
	Type string

	// Name is the constructor function/method name (e.g., "NewTask")
	Name string

	// CallbackArgIdx is the index of the callback argument in the constructor.
	// Default is 0 (first argument).
	CallbackArgIdx int
}

// FullName returns a human-readable name for the task constructor.
func (c TaskConstructor) FullName() string {
	pkgName := shortPkgName(c.Pkg)
	if c.Type == "" {
		return pkgName + "." + c.Name
	}
	return pkgName + "." + c.Type + "." + c.Name
}

// shortPkgName returns the last component of a package path.
func shortPkgName(pkgPath string) string {
	if idx := strings.LastIndex(pkgPath, "/"); idx >= 0 {
		return pkgPath[idx+1:]
	}
	return pkgPath
}

// Pattern defines the interface for context propagation patterns.
type Pattern interface {
	// Name returns a human-readable name for the pattern.
	Name() string

	// Check checks if the pattern is satisfied for the given call.
	// Returns true if the pattern is satisfied (no error).
	// taskConstructor may be nil if the API doesn't use a task constructor pattern.
	// taskSourceIdx indicates where the task object comes from:
	//   - TaskReceiverIdx (-1): task is the method receiver
	//   - 0+: task is the argument at that index
	Check(cctx *context.CheckContext, call *ast.CallExpr, callbackArg ast.Expr, taskConstructor *TaskConstructor, taskSourceIdx int) bool

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
	CheckGoStmt(cctx *context.CheckContext, stmt *ast.GoStmt) GoStmtResult

	// Message returns the diagnostic message when the pattern is violated.
	Message(ctxName string) string

	// DeferMessage returns the diagnostic message when deriver is only in defer.
	// Returns empty string if not applicable.
	DeferMessage(ctxName string) string
}
