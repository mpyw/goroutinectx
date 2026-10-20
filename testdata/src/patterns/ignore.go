// //goroutinectx:ignore on the line of a report and on the line before it.

package patterns

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ===== Ignore directive - same line =====

// [GOOD]: Ignore directive - same line
//
// The //goroutinectx:ignore directive suppresses the warning.
func goodIgnoredSameLineGoroutine(ctx context.Context) {
	go func() { //goroutinectx:ignore
		fmt.Println("ignored")
	}()
}

// [GOOD]: Ignore directive - same line
//
// The //goroutinectx:ignore directive suppresses the warning.
func goodIgnoredSameLineErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { //goroutinectx:ignore
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Ignore directive - same line
//
// The //goroutinectx:ignore directive suppresses the warning.
func goodIgnoredSameLineWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { //goroutinectx:ignore
	})
	wg.Wait()
}

// ===== Ignore directive - previous line =====

// [GOOD]: Ignore directive - previous line
//
// The //goroutinectx:ignore directive suppresses the warning.
func goodIgnoredPreviousLineGoroutine(ctx context.Context) {
	//goroutinectx:ignore
	go func() {
		fmt.Println("ignored")
	}()
}

// [GOOD]: Ignore directive - previous line
//
// The //goroutinectx:ignore directive suppresses the warning.
func goodIgnoredPreviousLineErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	//goroutinectx:ignore
	g.Go(func() error {
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Ignore directive - previous line
//
// The //goroutinectx:ignore directive suppresses the warning.
func goodIgnoredPreviousLineWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	//goroutinectx:ignore
	wg.Go(func() {
	})
	wg.Wait()
}
