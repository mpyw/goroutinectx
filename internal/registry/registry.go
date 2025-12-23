package registry

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/funcspec"
	"github.com/mpyw/goroutinectx/internal/patterns"
)

// CallArgEntry represents a registered API with CallArgPatterns.
// Used for: errgroup.Go, DoAllFns, DoAll (callback in call arguments).
type CallArgEntry struct {
	Spec            funcspec.Spec
	CallbackArgIdx  int
	Variadic        bool
	TaskConstructor *patterns.TaskConstructorConfig
	Patterns        []patterns.CallArgPattern
}

// TaskSourceEntry represents a registered API with TaskSourcePatterns.
// Used for: task.DoAsync (callback in constructor, task is receiver).
type TaskSourceEntry struct {
	Spec            funcspec.Spec
	TaskConstructor *patterns.TaskConstructorConfig
	Patterns        []patterns.TaskSourcePattern
}

// FuncMatch contains information about a matched function.
// Used by spawnerlabel to determine if a function is a spawner.
type FuncMatch struct {
	FullName       string
	CallbackArgIdx int
	AlwaysSpawns   bool // true for TaskSource APIs (method receiver is task)
}

// Registry holds registered APIs and their patterns.
type Registry struct {
	goStmtPatterns    []patterns.GoStmtPattern
	callArgEntries    []CallArgEntry
	taskSourceEntries []TaskSourceEntry
}

// New creates a new empty registry.
func New() *Registry {
	return &Registry{}
}

// RegisterGoStmt adds GoStmtPatterns to the registry.
func (r *Registry) RegisterGoStmt(patterns ...patterns.GoStmtPattern) {
	r.goStmtPatterns = append(r.goStmtPatterns, patterns...)
}

// GoStmtPatterns returns all registered GoStmtPatterns.
func (r *Registry) GoStmtPatterns() []patterns.GoStmtPattern {
	return r.goStmtPatterns
}

// RegisterCallArg adds a CallArgEntry to the registry.
func (r *Registry) RegisterCallArg(entry CallArgEntry) {
	r.callArgEntries = append(r.callArgEntries, entry)
}

// RegisterTaskSource adds a TaskSourceEntry to the registry.
func (r *Registry) RegisterTaskSource(entry TaskSourceEntry) {
	r.taskSourceEntries = append(r.taskSourceEntries, entry)
}

// CallArgEntries returns all registered CallArgEntries.
func (r *Registry) CallArgEntries() []CallArgEntry {
	return r.callArgEntries
}

// TaskSourceEntries returns all registered TaskSourceEntries.
func (r *Registry) TaskSourceEntries() []TaskSourceEntry {
	return r.taskSourceEntries
}

// MatchCallArg attempts to match a call expression against registered CallArg APIs.
// Returns the matched entry and callback argument, or nil if no match.
func (r *Registry) MatchCallArg(pass *analysis.Pass, call *ast.CallExpr) (*CallArgEntry, ast.Expr) {
	fn := funcspec.ExtractFunc(pass, call)
	if fn == nil {
		return nil, nil
	}

	for i := range r.callArgEntries {
		entry := &r.callArgEntries[i]
		if entry.Spec.Matches(fn) {
			return entry, getCallbackArg(call, entry.CallbackArgIdx)
		}
	}

	return nil, nil
}

// MatchTaskSource attempts to match a call expression against registered TaskSource APIs.
// Returns the matched entry, or nil if no match.
func (r *Registry) MatchTaskSource(pass *analysis.Pass, call *ast.CallExpr) *TaskSourceEntry {
	fn := funcspec.ExtractFunc(pass, call)
	if fn == nil {
		return nil
	}

	for i := range r.taskSourceEntries {
		entry := &r.taskSourceEntries[i]
		if entry.Spec.Matches(fn) {
			return entry
		}
	}

	return nil
}

// getCallbackArg extracts the callback argument at the given index.
func getCallbackArg(call *ast.CallExpr, idx int) ast.Expr {
	if idx < 0 || idx >= len(call.Args) {
		return nil
	}
	return call.Args[idx]
}

// MatchFunc attempts to match a types.Func against registered APIs.
// Returns FuncMatch for spawnerlabel detection, or nil if no match.
func (r *Registry) MatchFunc(fn *types.Func) *FuncMatch {
	for i := range r.callArgEntries {
		entry := &r.callArgEntries[i]
		if entry.Spec.Matches(fn) {
			return &FuncMatch{
				FullName:       entry.Spec.FullName(),
				CallbackArgIdx: entry.CallbackArgIdx,
				AlwaysSpawns:   false,
			}
		}
	}

	for i := range r.taskSourceEntries {
		entry := &r.taskSourceEntries[i]
		if entry.Spec.Matches(fn) {
			return &FuncMatch{
				FullName:       entry.Spec.FullName(),
				CallbackArgIdx: 0,
				AlwaysSpawns:   true,
			}
		}
	}

	return nil
}
