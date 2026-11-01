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

// ===== Senders that share a buffer =====

// [GOOD]: Two senders, a buffer of two
//
// Each send has a slot, so neither blocks.
func goodTwoSendersBufferTwoGoroutine(ctx context.Context, a, b *exec.Cmd) error {
	errc := make(chan error, 2)
	go func() {
		errc <- a.Wait()
	}()
	go func() {
		errc <- b.Wait()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: Two senders, a buffer of two
//
// Each send has a slot, so neither blocks.
func goodTwoSendersBufferTwoErrgroup(ctx context.Context, a, b *exec.Cmd) error {
	g := new(errgroup.Group)
	errc := make(chan error, 2)
	g.Go(func() error {
		errc <- a.Wait()
		return nil
	})
	g.Go(func() error {
		errc <- b.Wait()
		return nil
	})
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: A sender and a receiver that share a buffer of one
//
// A goroutine that only receives takes no slot.
func goodSenderAndReceiverGoroutine(ctx context.Context, cmd *exec.Cmd) {
	errc := make(chan error, 1)
	go func() {
		errc <- cmd.Wait()
	}()
	go func() {
		select {
		case <-errc:
		case <-ctx.Done():
		}
	}()
}

// [GOOD]: Two senders, one of them in a select with a default
//
// The send in the select cannot block.
func goodSelectSendNeighborGoroutine(ctx context.Context, a, b *exec.Cmd) error {
	errc := make(chan error, 2)
	go func() {
		errc <- a.Wait()
	}()
	go func() {
		err := b.Wait()
		select {
		case errc <- err:
		default:
		}
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: One sender, received without a select
//
// The caller waits for the one send.
func goodOneSenderReceiveGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	go func() {
		errc <- cmd.Wait()
	}()
	return <-errc
}

// [BAD]: Two senders, a buffer of one
//
// After the first receive, or on ctx.Done(), the second send blocks for ever
// (#242).
func badTwoSendersBufferOneGoroutine(ctx context.Context, a, b *exec.Cmd) error {
	errc := make(chan error, 1)
	go func() { // want `goroutine does not propagate context "ctx"`
		errc <- a.Wait()
	}()
	go func() { // want `goroutine does not propagate context "ctx"`
		errc <- b.Wait()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Two senders, a buffer of one
//
// After the first receive, or on ctx.Done(), the second send blocks for ever
// (#242).
func badTwoSendersBufferOneErrgroup(ctx context.Context, a, b *exec.Cmd) error {
	g := new(errgroup.Group)
	errc := make(chan error, 1)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		errc <- a.Wait()
		return nil
	})
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		errc <- b.Wait()
		return nil
	})
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Two senders, a buffer of one
//
// After the first receive, or on ctx.Done(), the second send blocks for ever
// (#242).
func badTwoSendersBufferOneWaitGroup(ctx context.Context, a, b *exec.Cmd) error {
	var wg sync.WaitGroup
	errc := make(chan error, 1)
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		errc <- a.Wait()
	})
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		errc <- b.Wait()
	})
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Two senders, a buffer of one
//
// After the first receive, or on ctx.Done(), the second send blocks for ever
// (#242).
func badTwoSendersBufferOneSpawner(ctx context.Context, a, b *exec.Cmd) error {
	g := new(errgroup.Group)
	errc := make(chan error, 1)
	runWithGroup(g, func() error { // want `runWithGroup\(\) func argument should use context "ctx"`
		errc <- a.Wait()
		return nil
	})
	runWithGroup(g, func() error { // want `runWithGroup\(\) func argument should use context "ctx"`
		errc <- b.Wait()
		return nil
	})
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: One sender that sends twice
//
// The second send blocks for ever when the caller has stopped receiving
// (#242).
func badSendTwiceGoroutine(ctx context.Context, a, b *exec.Cmd) error {
	errc := make(chan error, 1)
	go func() { // want `goroutine does not propagate context "ctx"`
		errc <- a.Wait()
		errc <- b.Wait()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Senders spawned in a loop
//
// There is one sender for each command, and one slot (#242).
func badLoopSendersGoroutine(ctx context.Context, cmds []*exec.Cmd) error {
	errc := make(chan error, 1)
	for _, cmd := range cmds {
		go func() { // want `goroutine does not propagate context "ctx"`
			errc <- cmd.Wait()
		}()
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Senders spawned in a loop
//
// There is one sender for each command, and one slot (#242).
func badLoopSendersErrgroup(ctx context.Context, cmds []*exec.Cmd) error {
	g := new(errgroup.Group)
	errc := make(chan error, 1)
	for _, cmd := range cmds {
		g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
			errc <- cmd.Wait()
			return nil
		})
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: One closure spawned in a loop
//
// The closure is made once but run for each command.
func badLoopSpawnedClosureGoroutine(ctx context.Context, cmd *exec.Cmd, n int) error {
	errc := make(chan error, 1)
	send := func() {
		errc <- cmd.Wait()
	}
	for range n {
		go send() // want `goroutine does not propagate context "ctx"`
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: A closure called once starts the sender
//
// The closure that makes the goroutine runs once, so there is one sender for
// the one slot.
func goodClosureCalledOnceGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	start := func() {
		go func() {
			errc <- cmd.Wait()
		}()
	}
	start()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A closure called twice starts the sender
//
// Two senders share the one slot.
func badClosureCalledTwiceGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	start := func() {
		go func() { // want `goroutine does not propagate context "ctx"`
			errc <- cmd.Wait()
		}()
	}
	start()
	start()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: A deferred closure starts the sender
//
// A deferred call runs the closure once.
func goodClosureDeferredGoroutine(ctx context.Context, cmd *exec.Cmd) {
	errc := make(chan error, 1)
	defer func() {
		select {
		case <-errc:
		case <-ctx.Done():
		}
	}()
	defer func() {
		go func() {
			errc <- cmd.Wait()
		}()
	}()
}

// [BAD]: A closure held as an interface starts the sender
//
// The closure is not followed through the interface, so it may run more
// than once.
func badClosureAsInterfaceGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	var start any = func() {
		go func() { // want `goroutine does not propagate context "ctx"`
			errc <- cmd.Wait()
		}()
	}
	start.(func())()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// waiterRunTwice runs f twice.
func waiterRunTwice(f func()) { f(); f() }

// [BAD]: A closure given to a call starts the sender
//
// The function called may run the closure more than once, as waiterRunTwice
// does, so two senders may share the one slot (#255).
func badClosureGivenToCallGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	waiterRunTwice(func() {
		go func() { // want `goroutine does not propagate context "ctx"`
			errc <- cmd.Wait()
		}()
	})
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: One sender given to a spawner
//
// A spawner may run the func it is given more than once, so it is not known
// to have one sender for the one slot (#255). errgroup.Group.Go, by
// contrast, runs it once.
func badOneSenderSpawner(ctx context.Context, cmd *exec.Cmd) error {
	g := new(errgroup.Group)
	errc := make(chan error, 1)
	runWithGroup(g, func() error { // want `runWithGroup\(\) func argument should use context "ctx"`
		errc <- cmd.Wait()
		return nil
	})
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: One sender given to errgroup.Group.Go
//
// errgroup.Group.Go runs the func once.
func goodOneSenderErrgroup(ctx context.Context, cmd *exec.Cmd) error {
	g := new(errgroup.Group)
	errc := make(chan error, 1)
	g.Go(func() error {
		errc <- cmd.Wait()
		return nil
	})
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: The channel is given to a call
//
// The function called may send on it too, so the slot may be taken.
func badChanEscapesGoroutine(ctx context.Context, cmd *exec.Cmd, fill func(chan<- error)) error {
	errc := make(chan error, 1)
	fill(errc)
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

// [BAD]: A neighbor that sends in a loop
//
// The neighbor's sends in a select can fill the slot first.
func badLoopSelectSendNeighborGoroutine(ctx context.Context, cmd *exec.Cmd, errs []error) error {
	errc := make(chan error, 1)
	go func() { // want `goroutine does not propagate context "ctx"`
		errc <- cmd.Wait()
	}()
	go func() { // want `goroutine does not propagate context "ctx"`
		for _, err := range errs {
			select {
			case errc <- err:
			default:
			}
		}
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: The variable of the channel is given to a call
//
// The function called may send on the channel, or set the variable to
// another one.
func badChanVarEscapesGoroutine(ctx context.Context, cmd *exec.Cmd, fill func(*chan error)) error {
	errc := make(chan error, 1)
	fill(&errc)
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

// [BAD]: The channel is sent on another channel
//
// Whoever receives it may send on it too.
func badChanSentGoroutine(ctx context.Context, cmd *exec.Cmd, out chan<- chan error) error {
	errc := make(chan error, 1)
	select {
	case out <- errc:
	default:
	}
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

// ===== Body in a function literal called in place (#282) =====

// [GOOD]: WaitGroup.Wait, then close, in a function literal called in place
//
// The literal is part of the goroutine, so its body is judged as the
// goroutine's own.
func goodWaitCloseInPlaceGoroutine(ctx context.Context, wg *sync.WaitGroup) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		func() {
			wg.Wait()
			close(done)
		}()
	}()
	return done
}

// [GOOD]: WaitGroup.Wait, then close, in a function literal called in place
//
// The literal is part of the callback, so its body is judged as the
// callback's own.
func goodWaitCloseInPlaceErrgroup(ctx context.Context, wg *sync.WaitGroup) error {
	g := new(errgroup.Group)
	done := make(chan struct{})
	g.Go(func() error {
		func() {
			wg.Wait()
			close(done)
		}()
		return nil
	})
	select {
	case <-done:
	case <-ctx.Done():
	}
	return g.Wait()
}

// [GOOD]: Cmd.Wait, then send on a buffered channel, in a literal called in place
//
// The one send cannot block, as the goroutine's own would not.
func goodCmdWaitSendInPlaceGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	go func() {
		func() {
			errc <- cmd.Wait()
		}()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: WaitGroup.Wait, with a close in a deferred literal
//
// A deferred literal is part of the goroutine too.
func goodWaitDeferredCloseInPlaceGoroutine(ctx context.Context, wg *sync.WaitGroup) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer func() {
			close(done)
		}()
		func() {
			wg.Wait()
		}()
	}()
	return done
}

// [BAD]: WaitGroup.Wait, then more work, in a literal called in place
//
// The literal does more than wait and signal.
func badWaitMoreWorkInPlaceGoroutine(ctx context.Context, wg *sync.WaitGroup) {
	go func() { // want `goroutine does not propagate context "ctx"`
		func() {
			wg.Wait()
			fmt.Println("done")
		}()
	}()
}

// [BAD]: Wait in a loop, in a literal called in place
//
// The literal waits again and again.
func badWaitLoopInPlaceGoroutine(ctx context.Context, wg *sync.WaitGroup, done chan<- struct{}) {
	go func() { // want `goroutine does not propagate context "ctx"`
		func() {
			for {
				wg.Wait()
				select {
				case done <- struct{}{}:
				default:
				}
			}
		}()
	}()
}

// [BAD]: Wait, then send on an unbuffered channel, in a literal called in place
//
// The send blocks for ever once the caller has returned on ctx.Done().
func badWaitUnbufferedSendInPlaceGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error)
	go func() { // want `goroutine does not propagate context "ctx"`
		func() {
			errc <- cmd.Wait()
		}()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: One send in the goroutine and one in a literal called in place
//
// The second send may find the slot the first one filled (#242).
func badTwoSendsInPlaceGoroutine(ctx context.Context, a, b *exec.Cmd) error {
	errc := make(chan error, 1)
	go func() { // want `goroutine does not propagate context "ctx"`
		errc <- a.Wait()
		func() {
			errc <- b.Wait()
		}()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A literal in the goroutine called twice
//
// A literal kept in a variable and called twice is not called in place.
func badLiteralCalledTwiceGoroutine(ctx context.Context, wg *sync.WaitGroup) <-chan struct{} {
	done := make(chan struct{})
	go func() { // want `goroutine does not propagate context "ctx"`
		wait := func() {
			wg.Wait()
		}
		wait()
		wait()
		close(done)
	}()
	return done
}

// [BAD]: A literal in the goroutine started in a goroutine of its own
//
// The outer goroutine does not wait for the literal, so the literal is not
// part of it. The inner goroutine is a waiter of its own, and passes.
func badLiteralInGoroutineGoroutine(ctx context.Context, wg *sync.WaitGroup) <-chan struct{} {
	done := make(chan struct{})
	go func() { // want `goroutine does not propagate context "ctx"`
		go func() {
			wg.Wait()
			close(done)
		}()
	}()
	return done
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
