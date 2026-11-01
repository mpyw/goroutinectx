// Package spawnerlabelexternal tests -spawnerlabel with
// -external-spawner=github.com/example/workerpool.Run (#377).
package spawnerlabelexternal

import "github.com/example/workerpool"

// [GOOD]: External spawner, literal (row 8a)
func goodExtLit() { workerpool.Run(func() {}) }

// [BAD]: External spawner, forward (row 8b)
func badExtFwd(f func()) { workerpool.Run(f) } // want `function "badExtFwd" should have //goroutinectx:spawner directive \(calls Run with func argument\)`

// [GOOD]: External spawner, an unrelated literal beside a func param
func goodExtParamUnrelated(f func()) {
	workerpool.Run(func() {})
	f()
}

// [BAD]: External spawner, a literal that calls its param
func badExtParamInLiteral(f func()) { workerpool.Run(func() { f() }) } // want `function "badExtParamInLiteral" should have //goroutinectx:spawner directive \(calls Run with func argument\)`

// [GOOD]: Pool.Submit is not given as an external spawner here
func goodExtNotGiven(p *workerpool.Pool, f func()) { p.Submit(f) }
