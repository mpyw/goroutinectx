// Package spawnerfact tests that //goroutinectx:spawner applies to calls
// from another package (#89), and that the directive may sit on any line of
// the doc comment (#85).
package spawnerfact

import (
	"context"

	"spawnerfactlib"
)

// ===== SHOULD REPORT =====

// [BAD]: A spawner of another package is checked (#89).
func badImportedSpawner(ctx context.Context) {
	spawnerfactlib.Spawn(func() {}) // want `Spawn\(\) func argument should use context "ctx"`
}

// [BAD]: The same for a method.
func badImportedSpawnerMethod(ctx context.Context) {
	p := &spawnerfactlib.Pool{}
	p.Submit(func() {}) // want `Submit\(\) func argument should use context "ctx"`
}

//goroutinectx:spawner
// spawnDoc runs fn.
func spawnDoc(fn func()) { go fn() }

// [BAD]: The directive above another doc line still marks a spawner (#85).
func badDirectiveAboveDocLine(ctx context.Context) {
	spawnDoc(func() {}) // want `spawnDoc\(\) func argument should use context "ctx"`
}

// ===== SHOULD NOT REPORT =====

// [GOOD]: A spawner of another package given a closure that uses ctx.
func goodImportedSpawner(ctx context.Context) {
	spawnerfactlib.Spawn(func() { _ = ctx })
}

// [GOOD]: A function of another package without the directive.
func goodImportedNotSpawner(ctx context.Context) {
	spawnerfactlib.Run(func() {})
}
