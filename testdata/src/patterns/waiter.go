// Goroutines that only wait and then signal, which have no use for ctx
// (#230), and their neighbors that are still reported. The reportwaiters
// fixture holds the same shapes, reported with -report-waiters.

package patterns

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ===== WaitGroup.Wait, then close =====

// [GOOD]: WaitGroup.Wait, then close
//
// The caller selects on the channel and ctx.Done().
func goodWaitCloseGoroutine(ctx context.Context) error {
	var wg sync.WaitGroup
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: WaitGroup.Wait, then close
//
// The caller selects on the channel and ctx.Done().
func goodWaitCloseErrgroup(ctx context.Context) error {
	var wg sync.WaitGroup
	g := new(errgroup.Group)
	done := make(chan struct{})
	g.Go(func() error {
		wg.Wait()
		close(done)
		return nil
	})
	select {
	case <-done:
	case <-ctx.Done():
	}
	return g.Wait()
}

// [GOOD]: WaitGroup.Wait, then close
//
// The caller selects on the channel and ctx.Done().
func goodWaitCloseWaitGroup(ctx context.Context) {
	var inner, outer sync.WaitGroup
	done := make(chan struct{})
	outer.Go(func() {
		inner.Wait()
		close(done)
	})
	select {
	case <-done:
	case <-ctx.Done():
	}
}

// [GOOD]: WaitGroup.Wait, then close
//
// The caller selects on the channel and ctx.Done().
func goodWaitCloseSpawner(ctx context.Context) {
	var wg sync.WaitGroup
	g := new(errgroup.Group)
	done := make(chan struct{})
	runWithGroup(g, func() error {
		wg.Wait()
		close(done)
		return nil
	})
	select {
	case <-done:
	case <-ctx.Done():
	}
}

// [GOOD]: WaitGroup.Wait with a deferred close
//
// The channel is closed in defer.
func goodWaitDeferCloseGoroutine(ctx context.Context) {
	var wg sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

// ===== Cmd.Wait, then send on a buffered channel =====

// [GOOD]: Cmd.Wait, then send on a buffered channel
//
// The send cannot block, so the goroutine ends when the process does.
func goodCmdWaitSendGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	go func() {
		errc <- cmd.Wait()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: Cmd.Wait, then store the error and close
//
// The error is stored to a variable the caller reads after the close.
func goodCmdWaitStoreGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	var err error
	done := make(chan struct{})
	go func() {
		err = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: Cmd.Wait alone
//
// The goroutine only reaps the process.
func goodCmdWaitOnlyGoroutine(ctx context.Context, cmd *exec.Cmd) {
	go func() { _ = cmd.Wait() }()
	<-ctx.Done()
}

// [GOOD]: Cmd.Wait, then send in a select with a default
//
// The send cannot block.
func goodCmdWaitSelectSendGoroutine(ctx context.Context, cmd *exec.Cmd, errc chan error) {
	go func() {
		err := cmd.Wait()
		select {
		case errc <- err:
		default:
		}
	}()
	<-ctx.Done()
}

// [GOOD]: Cmd.Wait, then send the error only when there is one
//
// A branch without a loop is fine.
func goodCmdWaitIfSendGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	go func() {
		if err := cmd.Wait(); err != nil {
			errc <- err
		}
		close(errc)
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ===== errgroup.Group.Wait, then send the error =====

// [GOOD]: errgroup.Group.Wait, then send the error
//
// The channel is buffered.
func goodGroupWaitSendGoroutine(ctx context.Context) error {
	g := new(errgroup.Group)
	errc := make(chan error, 1)
	go func() {
		errc <- g.Wait()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: errgroup.Group.Wait, then return the error
//
// The callback gives the error of the inner group to the outer one.
func goodGroupWaitReturnErrgroup(ctx context.Context) error {
	inner, outer := new(errgroup.Group), new(errgroup.Group)
	outer.Go(func() error { return inner.Wait() })
	<-ctx.Done()
	return outer.Wait()
}

// [GOOD]: errgroup.Group.Wait, then return the error
//
// The callback gives the error of the inner group to the outer one.
func goodGroupWaitReturnSpawner(ctx context.Context) error {
	inner, outer := new(errgroup.Group), new(errgroup.Group)
	runWithGroup(outer, func() error { return inner.Wait() })
	<-ctx.Done()
	return outer.Wait()
}

// ===== Wait, then more work =====

// [BAD]: WaitGroup.Wait, then more work
//
// The goroutine does more than signal.
func badWaitThenWorkGoroutine(ctx context.Context) {
	var wg sync.WaitGroup
	done := make(chan struct{})
	go func() { // want `goroutine does not propagate context "ctx"`
		wg.Wait()
		fmt.Println("all done")
		close(done)
	}()
	<-done
}

// [BAD]: WaitGroup.Wait, then more work
//
// The goroutine does more than signal.
func badWaitThenWorkErrgroup(ctx context.Context) error {
	var wg sync.WaitGroup
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		wg.Wait()
		return exec.Command("true").Run()
	})
	return g.Wait()
}

// [BAD]: WaitGroup.Wait, then more work
//
// The goroutine does more than signal.
func badWaitThenWorkWaitGroup(ctx context.Context) {
	var inner, outer sync.WaitGroup
	outer.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		inner.Wait()
		fmt.Println("all done")
	})
	outer.Wait()
}

// [BAD]: WaitGroup.Wait, then more work
//
// The goroutine does more than signal.
func badWaitThenWorkSpawner(ctx context.Context) {
	var wg sync.WaitGroup
	g := new(errgroup.Group)
	runWithGroup(g, func() error { // want `runWithGroup\(\) func argument should use context "ctx"`
		wg.Wait()
		fmt.Println("all done")
		return nil
	})
}

// ===== Wait in a loop =====

// [BAD]: Wait in a loop
//
// A loop may never end.
func badWaitLoopGoroutine(ctx context.Context, cmds []*exec.Cmd) {
	done := make(chan struct{})
	go func() { // want `goroutine does not propagate context "ctx"`
		for _, cmd := range cmds {
			_ = cmd.Wait()
		}
		close(done)
	}()
	<-done
}

// ===== Wait, then send on an unbuffered channel =====

// [BAD]: Wait, then send on an unbuffered channel
//
// When ctx is done the caller stops receiving, and the send blocks for ever.
func badWaitUnbufferedSendGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error)
	go func() { // want `goroutine does not propagate context "ctx"`
		errc <- cmd.Wait()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Wait, then send on a channel that is not followed
//
// The channel is a parameter, which may be unbuffered.
func badWaitParamSendGoroutine(ctx context.Context, cmd *exec.Cmd, errc chan error) {
	go func() { // want `goroutine does not propagate context "ctx"`
		errc <- cmd.Wait()
	}()
}

// ===== Receive, then signal =====

// [BAD]: Receive, then close
//
// A receive is not a wait: it could select on ctx.Done(), and it leaks when
// nothing is sent.
func badReceiveCloseGoroutine(ctx context.Context, in <-chan struct{}) {
	done := make(chan struct{})
	go func() { // want `goroutine does not propagate context "ctx"`
		<-in
		close(done)
	}()
	<-done
}

// ===== Server loop =====

// [BAD]: Server loop
//
// Serving is not waiting.
func badServeGoroutine(ctx context.Context, srv *http.Server, l net.Listener) {
	errc := make(chan error, 1)
	go func() { // want `goroutine does not propagate context "ctx"`
		errc <- srv.Serve(l)
	}()
	<-errc
}
