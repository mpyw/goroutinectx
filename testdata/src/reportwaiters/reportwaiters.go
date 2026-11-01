// Package reportwaiters holds the goroutines that only wait and signal of
// patterns/waiter.go, reported with -report-waiters (#230).
package reportwaiters

import (
	"context"
	"os/exec"
	"sync"

	"golang.org/x/sync/errgroup"
)

//goroutinectx:spawner
func runWithGroup(g *errgroup.Group, fn func() error) {
	g.Go(fn)
}

// [BAD]: WaitGroup.Wait, then close
//
// -report-waiters reports it.
func badWaitCloseGoroutine(ctx context.Context) {
	var wg sync.WaitGroup
	done := make(chan struct{})
	go func() { // want `goroutine does not propagate context "ctx"`
		wg.Wait()
		close(done)
	}()
	<-done
}

// [BAD]: WaitGroup.Wait, then close
//
// -report-waiters reports it.
func badWaitCloseErrgroup(ctx context.Context) error {
	var wg sync.WaitGroup
	g := new(errgroup.Group)
	done := make(chan struct{})
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		wg.Wait()
		close(done)
		return nil
	})
	return g.Wait()
}

// [BAD]: WaitGroup.Wait, then close
//
// -report-waiters reports it.
func badWaitCloseWaitGroup(ctx context.Context) {
	var inner, outer sync.WaitGroup
	done := make(chan struct{})
	outer.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		inner.Wait()
		close(done)
	})
	<-done
}

// [BAD]: WaitGroup.Wait, then close
//
// -report-waiters reports it.
func badWaitCloseSpawner(ctx context.Context) {
	var wg sync.WaitGroup
	g := new(errgroup.Group)
	done := make(chan struct{})
	runWithGroup(g, func() error { // want `runWithGroup\(\) func argument should use context "ctx"`
		wg.Wait()
		close(done)
		return nil
	})
	<-done
}

// [BAD]: Cmd.Wait, then send on a buffered channel
//
// -report-waiters reports it.
func badCmdWaitSendGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	go func() { // want `goroutine does not propagate context "ctx"`
		errc <- cmd.Wait()
	}()
	return <-errc
}

// [BAD]: Two senders, a buffer of two
//
// -report-waiters reports it.
func badTwoSendersBufferTwoGoroutine(ctx context.Context, a, b *exec.Cmd) error {
	errc := make(chan error, 2)
	go func() { // want `goroutine does not propagate context "ctx"`
		errc <- a.Wait()
	}()
	go func() { // want `goroutine does not propagate context "ctx"`
		errc <- b.Wait()
	}()
	return <-errc
}

// [BAD]: errgroup.Group.Wait, then return the error
//
// -report-waiters reports it.
func badGroupWaitReturnErrgroup(ctx context.Context) error {
	inner, outer := new(errgroup.Group), new(errgroup.Group)
	outer.Go(func() error { return inner.Wait() }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	return outer.Wait()
}

// [BAD]: WaitGroup.Wait, then close, in a function literal called in place
//
// -report-waiters reports it.
func badWaitCloseInPlaceGoroutine(ctx context.Context) {
	var wg sync.WaitGroup
	done := make(chan struct{})
	go func() { // want `goroutine does not propagate context "ctx"`
		func() {
			wg.Wait()
			close(done)
		}()
	}()
	<-done
}

// pickFlag picks a branch the analyzer cannot predict.
var pickFlag bool

// [BAD]: Two senders joined by a Phi, a buffer of one
//
// -report-waiters reports it (#298).
func badPhiTwoSendersGoroutine(ctx context.Context, a, b *exec.Cmd) error {
	errc := make(chan error, 1)
	run := func() { errc <- a.Wait() }
	if pickFlag {
		run = func() { errc <- b.Wait() }
	}
	go run() // want `goroutine does not propagate context "ctx"`
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Two senders joined by a Phi, given to errgroup.Group.Go
//
// -report-waiters reports it (#298).
func badPhiTwoSendersErrgroup(ctx context.Context, a, b *exec.Cmd) error {
	g := new(errgroup.Group)
	errc := make(chan error, 1)
	run := func() error { errc <- a.Wait(); return nil }
	if pickFlag {
		run = func() error { errc <- b.Wait(); return nil }
	}
	g.Go(run) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
