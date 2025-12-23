// Package patterns defines pattern interfaces and types for goroutinectx.
package patterns

import (
	"go/ast"

	"github.com/mpyw/goroutinectx/internal/context"
	"github.com/mpyw/goroutinectx/internal/directives/ignore"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// TaskConstructorConfig defines how tasks are created for task-based APIs.
// Used to trace back from executor APIs (e.g., DoAsync) to find the actual callback.
//
// Example: gotask.NewTask(fn) creates a Task, which is later executed via task.DoAsync(ctx).
// The TaskConstructorConfig for DoAsync would point to NewTask so we can find fn.
type TaskConstructorConfig struct {
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
func (c TaskConstructorConfig) FullName() string {
	pkgName := typeutil.ShortPkgName(c.Pkg)
	if c.Type == "" {
		return pkgName + "." + c.Name
	}
	return pkgName + "." + c.Type + "." + c.Name
}

// TaskCheckContext provides context for task-source pattern checks.
// Embeds CheckContext and adds task constructor configuration.
type TaskCheckContext struct {
	*context.CheckContext
	// Constructor defines how to trace back to the task's callback (e.g., NewTask).
	Constructor *TaskConstructorConfig
}

// CallArgPattern checks callback arguments passed directly to APIs.
// Used for: errgroup.Go(fn), DoAllFns(ctx, fn1, fn2, ...), DoAll(ctx, task1, task2, ...)
type CallArgPattern interface {
	// Name returns a human-readable name for the pattern.
	Name() string

	// CheckerName returns the ignore checker name for this pattern.
	CheckerName() ignore.CheckerName

	// Check checks if the callback argument satisfies the pattern.
	// constructor is optional - nil for direct fn args (errgroup.Go, DoAllFns),
	// non-nil for task args that need tracing (DoAll).
	Check(cctx *context.CheckContext, arg ast.Expr, constructor *TaskConstructorConfig) bool

	// Message returns the diagnostic message when the pattern is violated.
	Message(apiName string, ctxName string) string
}

// TaskSourcePattern checks APIs where the callback is in a task constructor.
// Used for: task.DoAsync(ctx) where task = NewTask(fn)
// The pattern traces the receiver back to the constructor to find the callback.
type TaskSourcePattern interface {
	// Name returns a human-readable name for the pattern.
	Name() string

	// CheckerName returns the ignore checker name for this pattern.
	CheckerName() ignore.CheckerName

	// Check checks if the task's callback (from constructor) satisfies the pattern.
	// The task is always the method receiver.
	Check(tcctx *TaskCheckContext, call *ast.CallExpr) bool

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

	// CheckerName returns the ignore checker name for this pattern.
	CheckerName() ignore.CheckerName

	// Check checks if the pattern is satisfied for the given go statement.
	Check(cctx *context.CheckContext, stmt *ast.GoStmt) GoStmtResult

	// Message returns the diagnostic message when the pattern is violated.
	Message(ctxName string) string

	// DeferMessage returns the diagnostic message when deriver is only in defer.
	// Returns empty string if not applicable.
	DeferMessage(ctxName string) string
}
