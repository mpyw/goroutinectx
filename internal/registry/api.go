// Package registry provides API registration for goroutinectx patterns.
package registry

import (
	"strings"

	"github.com/mpyw/goroutinectx/internal/patterns"
)

// APIKind represents the kind of API (method or function).
type APIKind int

const (
	// KindMethod represents a method call: receiver.Method()
	KindMethod APIKind = iota
	// KindFunc represents a package-level function call: pkg.Func()
	KindFunc
)

// API defines a goroutine-spawning API to check.
type API struct {
	// Pkg is the package path (e.g., "golang.org/x/sync/errgroup")
	Pkg string

	// Type is the receiver type name (empty for package-level functions)
	Type string

	// Name is the method or function name (e.g., "Go", "Submit")
	Name string

	// Kind is KindMethod or KindFunc
	Kind APIKind

	// CallbackArgIdx is the index of the callback argument (0-based).
	CallbackArgIdx int

	// Variadic indicates that all arguments from CallbackArgIdx onwards should be checked.
	// Used for APIs like DoAllFns(ctx, fn1, fn2, ...) where multiple callbacks are passed.
	Variadic bool

	// TaskConstructor defines how to trace back to find the actual callback.
	// If set, TaskSourceIdx indicates where the task object comes from,
	// and we trace into the constructor's callback argument to find the callback body.
	//
	// Example: For task.DoAsync(ctx), TaskSourceIdx is TaskReceiverIdx (-1),
	// meaning the task comes from the receiver. We trace task -> NewTask(fn) -> fn.
	TaskConstructor *patterns.TaskConstructor

	// TaskSourceIdx indicates where the task object comes from when TaskConstructor is set.
	// Use patterns.TaskReceiverIdx (-1) for method receiver (e.g., task.DoAsync(ctx)).
	// Use 0+ for argument index (e.g., executor.Run(ctx, task) where task is at index 1).
	// Default (0) means first argument, so set explicitly when needed.
	TaskSourceIdx int
}

// FullName returns a human-readable name for the API.
// For methods: "errgroup.Group.Go"
// For functions: "gotask.Do"
func (a API) FullName() string {
	pkgName := shortPkgName(a.Pkg)
	if a.Type == "" {
		return pkgName + "." + a.Name
	}
	return pkgName + "." + a.Type + "." + a.Name
}

// shortPkgName returns the last component of a package path.
func shortPkgName(pkgPath string) string {
	if idx := strings.LastIndex(pkgPath, "/"); idx >= 0 {
		return pkgPath[idx+1:]
	}
	return pkgPath
}
