// Package linedirective tests that //line directives do not move directives.
// Directives match the physical source lines, not the adjusted ones.
package linedirective

import "context"

// ===== WITHOUT //line (CONTROL) =====

// [GOOD]: ignore on the line above the go statement
func goodIgnoreControl(ctx context.Context) {
	//goroutinectx:ignore
	go func() {}()
}

// [BAD]: no ignore
func badNoIgnoreControl(ctx context.Context) {
	go func() {}() // want `goroutine does not propagate context "ctx"`
}

// [GOOD]: spawner directive on the line above the function
//
//goroutinectx:spawner
func runMarked(f func()) { go f() }

// [BAD]: calls a spawner without context
func badMarkedSpawner(ctx context.Context) {
	runMarked(func() {}) // want `runMarked\(\) func argument should use context "ctx"`
}

// ===== BELOW //line =====

//line fake.tmpl:1
func goodIgnoreBelowLine(ctx context.Context) {
	//goroutinectx:ignore
	go func() {}()
}

// [BAD]: no ignore
func badNoIgnoreBelowLine(ctx context.Context) {
	go func() {}() // want `goroutine does not propagate context "ctx"`
}

// [GOOD]: ignore above a go statement that starts with a /*line*/ directive
func goodIgnoreBlockLine(ctx context.Context) {
	//goroutinectx:ignore
	/*line block.tmpl:30:1*/ go func() {}()
}

// ===== ADJUSTED LINES THAT COLLIDE =====

//line a.tmpl:10
//goroutinectx:spawner
var spawnerTarget = 1

// The spawner directive is at adjusted line 10 and runUnmarked at adjusted
// line 11, but the directive belongs to spawnerTarget. runUnmarked is not a
// spawner.
//
//line b.tmpl:11
func runUnmarked(f func()) { go f() }

// [GOOD]: runUnmarked is not a spawner
func goodUnmarkedSpawner(ctx context.Context) {
	runUnmarked(func() {})
}

//line linedirective.go:20
//goroutinectx:ignore // belongs to ignoreTarget // want `unused goroutinectx:ignore directive`
var ignoreTarget = 2

// [BAD]: both regions name this file. The ignore is at adjusted line 20 and
// the go statement at adjusted line 21, but the ignore belongs to ignoreTarget.
func badIgnoreCollision(ctx context.Context) {
//line linedirective.go:21
	go func() {}() // want `goroutine does not propagate context "ctx"`
}
