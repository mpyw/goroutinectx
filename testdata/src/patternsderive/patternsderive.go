// Package patternsderive holds the cases that the goroutine, errgroup,
// waitgroup and spawner checkers share, run with -goroutine-deriver. A go
// statement must call the deriver. A callback passes when it either captures
// ctx or calls the deriver. Each case's variants sit side by side under one
// heading.
package patternsderive

import (
	"sync"

	"golang.org/x/sync/errgroup"
)

//goroutinectx:spawner
func runWithGroup(g *errgroup.Group, fn func() error) {
	g.Go(fn)
}

//goroutinectx:spawner
func runWithWaitGroup(wg *sync.WaitGroup, fn func()) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		fn()
	}()
}
