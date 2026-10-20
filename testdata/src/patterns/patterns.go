// Package patterns holds the cases that the goroutine, errgroup, waitgroup
// and spawner checkers share. Each case is written once per checker, and the
// variants sit side by side under one heading, so a case missing for one
// checker shows. Cases that only one checker has stay in its own directory.
package patterns

import (
	"golang.org/x/sync/errgroup"
)

//goroutinectx:spawner
func runWithGroup(g *errgroup.Group, fn func() error) {
	g.Go(fn)
}
