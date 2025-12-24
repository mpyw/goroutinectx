package registry

import (
	"go/types"

	"github.com/mpyw/goroutinectx/internal/funcspec"
)

// Entry represents a registered spawner API.
type Entry struct {
	Spec           funcspec.Spec
	CallbackArgIdx int
	AlwaysSpawns   bool // true for TaskSource APIs (method receiver is task)
}

// FuncMatch contains information about a matched function.
// Used by spawnerlabel to determine if a function is a spawner.
type FuncMatch struct {
	FullName       string
	CallbackArgIdx int
	AlwaysSpawns   bool
}

// Registry holds registered spawner APIs for spawnerlabel detection.
type Registry struct {
	entries []Entry
}

// New creates a new empty registry.
func New() *Registry {
	return &Registry{}
}

// Register adds an entry to the registry.
func (r *Registry) Register(entry Entry) {
	r.entries = append(r.entries, entry)
}

// MatchFunc attempts to match a types.Func against registered APIs.
// Returns FuncMatch for spawnerlabel detection, or nil if no match.
func (r *Registry) MatchFunc(fn *types.Func) *FuncMatch {
	for i := range r.entries {
		entry := &r.entries[i]
		if entry.Spec.Matches(fn) {
			return &FuncMatch{
				FullName:       entry.Spec.FullName(),
				CallbackArgIdx: entry.CallbackArgIdx,
				AlwaysSpawns:   entry.AlwaysSpawns,
			}
		}
	}
	return nil
}
