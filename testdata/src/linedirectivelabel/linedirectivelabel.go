// Package linedirectivelabel tests that //line directives do not move
// spawnerlabel ignore directives.
package linedirectivelabel

import "golang.org/x/sync/errgroup"

//line fake.tmpl:1
//goroutinectx:ignore spawnerlabel
func goodIgnoredBelowLine(g *errgroup.Group) {
	g.Go(func() error { return nil })
}

//line a.tmpl:10
//goroutinectx:ignore spawnerlabel // want `unused goroutinectx:ignore directive for checker\(s\): spawnerlabel`
var ignoreTarget = 1

// The ignore is at adjusted line 10 and the function at adjusted line 11, but
// the ignore belongs to ignoreTarget.
//
//line b.tmpl:11
func badIgnoreCollision(g *errgroup.Group) { // want `function "badIgnoreCollision" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g.Go(func() error { return nil })
}
