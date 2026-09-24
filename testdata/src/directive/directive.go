// Package directive tests which comments are read as goroutinectx directives.
package directive

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

//goroutinectx:spawner
func runCanonical(g *errgroup.Group, fn func() error) {
	g.Go(fn)
}

// goroutinectx:spawner // want `malformed goroutinectx directive: write it as //goroutinectx:name`
func runSpaced(g *errgroup.Group, fn func() error) {
	g.Go(fn)
}

/* goroutinectx:spawner */ // want `malformed goroutinectx directive: write it as //goroutinectx:name`
func runBlock(g *errgroup.Group, fn func() error) {
	g.Go(fn)
}

//goroutinectx:spawnerX
func runLookalike(g *errgroup.Group, fn func() error) {
	g.Go(fn)
}

// canonical spawner directive marks the function
func badCanonicalSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runCanonical(g, func() error { // want `runCanonical\(\) func argument should use context "ctx"`
		return nil
	})
	_ = g.Wait()
}

// malformed and lookalike spawner directives mark nothing
func noncanonicalSpawners(ctx context.Context) {
	g := new(errgroup.Group)
	runSpaced(g, func() error { return nil })
	runBlock(g, func() error { return nil })
	runLookalike(g, func() error { return nil })
	_ = g.Wait()
}

// canonical ignore directive suppresses the report
func goodCanonicalIgnore(ctx context.Context) {
	//goroutinectx:ignore
	go func() {
		fmt.Println("background task")
	}()
}

// canonical ignore directive with a checker and a reason
func goodCanonicalIgnoreWithArgs(ctx context.Context) {
	//goroutinectx:ignore goroutine - fire-and-forget
	go func() {
		fmt.Println("background task")
	}()
}

// space after the comment marker
func badSpacedIgnore(ctx context.Context) {
	// goroutinectx:ignore // want `malformed goroutinectx directive: write it as //goroutinectx:name`
	go func() { // want `goroutine does not propagate context "ctx"`
		fmt.Println("background task")
	}()
}

// space after the colon
func badSpaceAfterColonIgnore(ctx context.Context) {
	//goroutinectx: ignore // want `malformed goroutinectx directive: write it as //goroutinectx:name`
	go func() { // want `goroutine does not propagate context "ctx"`
		fmt.Println("background task")
	}()
}

// block comment
func badBlockIgnore(ctx context.Context) {
	/* goroutinectx:ignore */ // want `malformed goroutinectx directive: write it as //goroutinectx:name`
	go func() { // want `goroutine does not propagate context "ctx"`
		fmt.Println("background task")
	}()
}

// a longer name is not the ignore directive
func badLookalikeIgnore(ctx context.Context) {
	//goroutinectx:ignored
	go func() { // want `goroutine does not propagate context "ctx"`
		fmt.Println("background task")
	}()
}

// uppercase name is not a directive
func badUppercaseIgnore(ctx context.Context) {
	//goroutinectx:Ignore // want `malformed goroutinectx directive: write it as //goroutinectx:name`
	go func() { // want `goroutine does not propagate context "ctx"`
		fmt.Println("background task")
	}()
}
