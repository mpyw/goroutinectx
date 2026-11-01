// Package spawnerlabelown tests -spawnerlabel with -external-spawner naming
// Pool.Submit and Start of this package, which is checked (#383). Neither
// has the directive, so there is none to call unnecessary.
package spawnerlabelown

// Pool is named by -external-spawner.
type Pool struct{ queue chan func() }

// [GOOD]: Named by -external-spawner, and only calls its param in place
func (p *Pool) Submit(f func()) { f() }

// Options is what Start is given.
type Options struct{ Task func() }

// [GOOD]: Named by -external-spawner, with no func param
func Start(Options) {}

// [BAD]: Has the directive, and only calls its param in place
//
//goroutinectx:spawner
func unnecessaryOwnDirective(f func()) { f() } // want `function "unnecessaryOwnDirective" has unnecessary //goroutinectx:spawner directive`
