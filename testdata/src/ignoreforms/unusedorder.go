package ignoreforms

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// [BAD]: Several unused ignores, in a second file of the package. They are
// reported in the order of their positions, on every run (#294).
func badUnusedInOrder(ctx context.Context) {
	//goroutinectx:ignore // want `^unused goroutinectx:ignore directive$`
	go func() {
		_ = ctx
	}()
	g := new(errgroup.Group)
	//goroutinectx:ignore errgroup,goroutine // want `unused goroutinectx:ignore directive for checker\(s\): errgroup, goroutine`
	g.Go(func() error {
		_ = ctx
		return nil
	})
	//goroutinectx:ignore goroutine // want `unused goroutinectx:ignore directive for checker\(s\): goroutine`
	go func() {
		_ = ctx
	}()
	_ = g.Wait()
}
