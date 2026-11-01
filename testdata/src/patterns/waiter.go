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

// ===== Senders joined by a Phi (#298) =====

// [GOOD]: One sender run through a Phi
//
// Each value the Phi joins is the one sender, run once.
func goodPhiSenderGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	run := send
	if conditionFlag {
		run = send
	}
	go run()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: Two senders joined by a Phi, a buffer of one
//
// The go statement runs one of them, so there is one sender for the one
// slot.
func goodPhiTwoSendersGoroutine(ctx context.Context, a, b *exec.Cmd) error {
	errc := make(chan error, 1)
	run := func() { errc <- a.Wait() }
	if conditionFlag {
		run = func() { errc <- b.Wait() }
	}
	go run()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: Two senders joined by a Phi, given to errgroup.Group.Go
//
// errgroup.Group.Go runs one of them once.
func goodPhiTwoSendersErrgroup(ctx context.Context, a, b *exec.Cmd) error {
	g := new(errgroup.Group)
	errc := make(chan error, 1)
	run := func() error { errc <- a.Wait(); return nil }
	if conditionFlag {
		run = func() error { errc <- b.Wait(); return nil }
	}
	g.Go(run)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: Two senders joined by a Phi, given to sync.WaitGroup.Go
//
// sync.WaitGroup.Go runs one of them once.
func goodPhiTwoSendersWaitGroup(ctx context.Context, a, b *exec.Cmd) error {
	var wg sync.WaitGroup
	errc := make(chan error, 1)
	run := func() { errc <- a.Wait() }
	if conditionFlag {
		run = func() { errc <- b.Wait() }
	}
	wg.Go(run)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: A closure called once through two Phis starts the sender
//
// Each Phi is used once, so the closure that makes the goroutine runs once.
func goodPhiClosureCalledOnceGoroutine(ctx context.Context, cmd *exec.Cmd, other bool) error {
	errc := make(chan error, 1)
	start := func() {
		go func() {
			errc <- cmd.Wait()
		}()
	}
	s := start
	if conditionFlag {
		s = start
	}
	t := s
	if other {
		t = s
	}
	t()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: A sender only made, a buffer of two
//
// The closure only made is counted as a sender too, though nothing runs it,
// and the buffer has room for both.
func goodSenderOnlyMadeBufferTwoGoroutine(ctx context.Context, a, b *exec.Cmd) error {
	errc := make(chan error, 2)
	go func() {
		errc <- a.Wait()
	}()
	_ = func() { errc <- b.Wait() }
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Senders joined by a Phi and one more, a buffer of one
//
// The go statement of the Phi runs one sender, and the other go statement
// one more, so two senders share the one slot.
func badPhiSendersAndOneMoreGoroutine(ctx context.Context, a, b *exec.Cmd) error {
	errc := make(chan error, 1)
	run := func() { errc <- a.Wait() }
	if conditionFlag {
		run = func() { errc <- a.Wait() }
	}
	go run() // want `goroutine does not propagate context "ctx"`
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

// [BAD]: A sender through a Phi spawned in a loop
//
// The go statement runs the sender for each pass of the loop.
func badPhiSenderInLoopGoroutine(ctx context.Context, cmd *exec.Cmd, n int) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	run := send
	if conditionFlag {
		run = send
	}
	for range n {
		go run() // want `goroutine does not propagate context "ctx"`
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A closure through a Phi called twice starts the sender
//
// Two senders share the one slot.
func badPhiClosureCalledTwiceGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	start := func() {
		go func() { // want `goroutine does not propagate context "ctx"`
			errc <- cmd.Wait()
		}()
	}
	s := start
	if conditionFlag {
		s = start
	}
	s()
	s()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: One sender through a Phi given to a spawner
//
// A spawner may run the func it is given more than once (#255).
func badPhiSenderSpawner(ctx context.Context, cmd *exec.Cmd) error {
	g := new(errgroup.Group)
	errc := make(chan error, 1)
	send := func() error { errc <- cmd.Wait(); return nil }
	run := send
	if conditionFlag {
		run = send
	}
	runWithGroup(g, run) // want `runWithGroup\(\) func argument should use context "ctx"`
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

// ===== Sender in a variable a closure captures (#321) =====

// [GOOD]: A goroutine calls the sender from a variable it captures
//
// The goroutine runs the one sender once, as go run() does.
func goodCapturedSenderCalledGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	run := func() { errc <- cmd.Wait() }
	go func() { run() }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: A closure called in place starts the sender from a variable it captures
//
// The closure runs once, so the go statement in it runs the sender once.
func goodCapturedSenderInClosureGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	run := func() { errc <- cmd.Wait() }
	func() { go run() }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: A closure called in place starts one sender stored twice
//
// The variable holds the one sender either way.
func goodCapturedSenderStoredTwiceGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	run := send
	if conditionFlag {
		run = send
	}
	func() { go run() }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: A closure called in place starts one of two senders, a buffer of one
//
// The go statement runs one of the senders the variable holds, so there is
// one sender for the one slot.
func goodCapturedTwoSendersInClosureGoroutine(ctx context.Context, a, b *exec.Cmd) error {
	errc := make(chan error, 1)
	run := func() { errc <- a.Wait() }
	if conditionFlag {
		run = func() { errc <- b.Wait() }
	}
	func() { go run() }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: A closure called in place gives the sender to errgroup.Group.Go
//
// errgroup.Group.Go runs it once.
func goodCapturedSenderInClosureErrgroup(ctx context.Context, cmd *exec.Cmd) error {
	g := new(errgroup.Group)
	errc := make(chan error, 1)
	run := func() error { errc <- cmd.Wait(); return nil }
	func() { g.Go(run) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: A closure called in place gives the sender to sync.WaitGroup.Go
//
// sync.WaitGroup.Go runs it once.
func goodCapturedSenderInClosureWaitGroup(ctx context.Context, cmd *exec.Cmd) error {
	var wg sync.WaitGroup
	errc := make(chan error, 1)
	run := func() { errc <- cmd.Wait() }
	func() { wg.Go(run) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A goroutine calls the sender from a variable twice
//
// Two sends share the one slot.
func badCapturedSenderCalledTwiceGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	run := func() { errc <- cmd.Wait() }
	go func() { // want `goroutine does not propagate context "ctx"`
		run()
		run()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Goroutines that call the sender from a variable spawned in a loop
//
// The sender runs for each pass of the loop.
func badCapturedSenderCalledInLoopGoroutine(ctx context.Context, cmd *exec.Cmd, n int) error {
	errc := make(chan error, 1)
	run := func() { errc <- cmd.Wait() }
	for range n {
		go func() { run() }() // want `goroutine does not propagate context "ctx"`
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Two closures start the sender from a variable they capture
//
// Each go statement runs the sender, so two senders share the one slot.
func badCapturedSenderTwoClosuresGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	run := func() { errc <- cmd.Wait() }
	func() { go run() }() // want `goroutine does not propagate context "ctx"`
	func() { go run() }() // want `goroutine does not propagate context "ctx"`
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A closure called twice starts the sender from a variable it captures
//
// The go statement runs for each call, so two senders share the one slot.
func badCapturedSenderClosureCalledTwiceGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	run := func() { errc <- cmd.Wait() }
	start := func() { go run() } // want `goroutine does not propagate context "ctx"`
	start()
	start()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A closure given to a call starts the sender from a variable it captures
//
// The function called may run the closure more than once (#255).
func badCapturedSenderClosureGivenToCallGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	run := func() { errc <- cmd.Wait() }
	waiterRunTwice(func() { go run() }) // want `goroutine does not propagate context "ctx"`
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A closure gives the sender from a variable it captures to a spawner
//
// A spawner may run the func it is given more than once (#255).
func badCapturedSenderInClosureSpawner(ctx context.Context, cmd *exec.Cmd) error {
	g := new(errgroup.Group)
	errc := make(chan error, 1)
	run := func() error { errc <- cmd.Wait(); return nil }
	func() {
		runWithGroup(g, run) // want `runWithGroup\(\) func argument should use context "ctx"`
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: A goroutine calls one of two senders from a variable it captures, a buffer of one
//
// The variable holds one of them, so there is one sender for the one slot.
func goodCapturedTwoSendersCalledGoroutine(ctx context.Context, a, b *exec.Cmd) error {
	errc := make(chan error, 1)
	run := func() { errc <- a.Wait() }
	if conditionFlag {
		run = func() { errc <- b.Wait() }
	}
	go func() { run() }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: A goroutine defers the sender from a variable it captures
//
// The deferred call runs the sender once.
func goodCapturedSenderDeferredGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	run := func() { errc <- cmd.Wait() }
	go func() { defer run() }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A goroutine calls a sender that calls itself through its variable
//
// The sender may run any number of times.
func badCapturedSenderRecursiveGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	var run func()
	run = func() {
		errc <- cmd.Wait()
		run()
	}
	go func() { run() }() // want `goroutine does not propagate context "ctx"`
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A goroutine calls one of two senders from a variable, and sends itself
//
// The goroutine sends once more, so two sends share the one slot.
func badCapturedTwoSendersCalledAndSendGoroutine(ctx context.Context, a, b *exec.Cmd) error {
	errc := make(chan error, 1)
	run := func() { errc <- a.Wait() }
	if conditionFlag {
		run = func() { errc <- b.Wait() }
	}
	go func() { // want `goroutine does not propagate context "ctx"`
		run()
		errc <- a.Wait()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ===== Callers of a captured sender joined by a Phi (#345) =====

// [GOOD]: Two closures joined by a Phi call the sender from a variable they capture
//
// Only one of the closures runs, once, so the sender runs once.
func goodCapturedSenderPhiCallersGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	run := func() { send() }
	if conditionFlag {
		run = func() { send() }
	}
	go run()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: A closure called in place starts one of two callers of a captured sender
//
// The go statement runs one of the callers the variable holds, once.
func goodCapturedSenderPhiCallersInClosureGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	run := func() { send() }
	if conditionFlag {
		run = func() { send() }
	}
	func() { go run() }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: Two closures joined by a Phi defer the sender from a variable they capture
//
// The deferred call runs the sender once, in the one closure that runs.
func goodCapturedSenderPhiDeferringGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	run := func() { defer send() }
	if conditionFlag {
		run = func() { send() }
	}
	go run()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Two closures joined by a Phi call the sender, an unbuffered channel
//
// The send blocks for good once the caller stops receiving (#242).
func badCapturedSenderPhiCallersUnbufferedGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error)
	send := func() { errc <- cmd.Wait() }
	run := func() { send() }
	if conditionFlag {
		run = func() { send() }
	}
	go run() // want `goroutine does not propagate context "ctx"`
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: One of two closures joined by a Phi calls the sender twice
//
// Two sends share the one slot.
func badCapturedSenderPhiCallerTwiceGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	run := func() { send() }
	if conditionFlag {
		run = func() {
			send()
			send()
		}
	}
	go run() // want `goroutine does not propagate context "ctx"`
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Two closures joined by a Phi call the sender, which the function calls too
//
// The function sends once more, so two sends share the one slot.
func badCapturedSenderPhiCallersAndCalledGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	run := func() { send() }
	if conditionFlag {
		run = func() { send() }
	}
	go run() // want `goroutine does not propagate context "ctx"`
	send()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Two closures joined by a Phi call the sender, spawned in a loop
//
// The sender runs for each pass of the loop.
func badCapturedSenderPhiCallersInLoopGoroutine(ctx context.Context, cmd *exec.Cmd, n int) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	run := func() { send() }
	if conditionFlag {
		run = func() { send() }
	}
	for range n {
		go run() // want `goroutine does not propagate context "ctx"`
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Two closures joined by a Phi call the sender, each spawned
//
// Each go statement runs one of them, so two senders share the one slot.
func badCapturedSenderPhiCallersSpawnedTwiceGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	run := func() { send() }
	if conditionFlag {
		run = func() { send() }
	}
	go run() // want `goroutine does not propagate context "ctx"`
	go run() // want `goroutine does not propagate context "ctx"`
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Two closures not joined call the sender from a variable they capture
//
// Each closure runs at its own site, so two senders share the one slot.
func badCapturedSenderTwoCallersGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	a := func() { send() }
	b := func() { send() }
	go a() // want `goroutine does not propagate context "ctx"`
	go b() // want `goroutine does not propagate context "ctx"`
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: One of two closures joined by a Phi gives the sender to a call
//
// The function called may run the sender more than once (#255).
func badCapturedSenderPhiCallerGivesToCallGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	run := func() { send() }
	if conditionFlag {
		run = func() { waiterRunTwice(send) }
	}
	go run() // want `goroutine does not propagate context "ctx"`
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A goroutine calls the sender from a variable the function calls in a loop
//
// The sender runs for each pass of the loop as well.
func badCapturedSenderAlsoCalledInLoopGoroutine(ctx context.Context, cmd *exec.Cmd, n int) error {
	errc := make(chan error, 1)
	run := func() { errc <- cmd.Wait() }
	go func() { run() }() // want `goroutine does not propagate context "ctx"`
	for range n {
		run()
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A goroutine sends on a channel a sender that calls itself also sends on
//
// The sender may run any number of times once called, so its sends cannot
// be counted. This once overflowed the stack, following the sender to
// itself.
func badCapturedRecursiveSenderOtherGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	var run func()
	run = func() {
		errc <- cmd.Wait()
		run()
	}
	go func() { // want `goroutine does not propagate context "ctx"`
		_ = cmd.Wait()
		errc <- nil
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ===== Senders started in branches that exclude each other (#350) =====

// [GOOD]: Each branch of an if starts a caller of a captured sender
//
// Only one of the branches runs, so the sender runs once.
func goodCapturedSenderBranchCallersGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	if conditionFlag {
		go func() { send() }()
	} else {
		go func() { send() }()
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: Each branch of an if starts a sender
//
// Only one of the branches runs, so one sender shares the slot.
func goodBranchSendersGoroutine(ctx context.Context, a, b *exec.Cmd) error {
	errc := make(chan error, 1)
	if conditionFlag {
		go func() { errc <- a.Wait() }()
	} else {
		go func() { errc <- b.Wait() }()
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: Each case of a switch starts a sender
//
// Only one of the cases runs, so one sender shares the slot.
func goodSwitchSendersGoroutine(ctx context.Context, a, b, c *exec.Cmd, n int) error {
	errc := make(chan error, 1)
	switch n {
	case 0:
		go func() { errc <- a.Wait() }()
	case 1:
		go func() { errc <- b.Wait() }()
	default:
		go func() { errc <- c.Wait() }()
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A sender started in a branch, and another after the if
//
// Both run when the branch is taken, so two senders share the one slot.
func badBranchSenderAndAfterGoroutine(ctx context.Context, a, b *exec.Cmd) error {
	errc := make(chan error, 1)
	if conditionFlag {
		go func() { errc <- a.Wait() }() // want `goroutine does not propagate context "ctx"`
	}
	go func() { errc <- b.Wait() }() // want `goroutine does not propagate context "ctx"`
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Each branch of an if starts a caller of a captured sender, and so does the code after it
//
// Two of the callers run, so two senders share the one slot.
func badCapturedSenderBranchCallersAndAfterGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	if conditionFlag {
		go func() { send() }() // want `goroutine does not propagate context "ctx"`
	} else {
		go func() { send() }() // want `goroutine does not propagate context "ctx"`
	}
	go func() { send() }() // want `goroutine does not propagate context "ctx"`
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: One branch of an if starts two senders
//
// Both senders of that branch run, so two share the one slot.
func badBranchTwoSendersGoroutine(ctx context.Context, a, b, c *exec.Cmd) error {
	errc := make(chan error, 1)
	if conditionFlag {
		go func() { errc <- a.Wait() }() // want `goroutine does not propagate context "ctx"`
		go func() { errc <- b.Wait() }() // want `goroutine does not propagate context "ctx"`
	} else {
		go func() { errc <- c.Wait() }() // want `goroutine does not propagate context "ctx"`
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Each branch of an if starts a sender, in a loop
//
// The branches run for each pass of the loop, so senders pile up.
func badBranchSendersInLoopGoroutine(ctx context.Context, a, b *exec.Cmd, n int) error {
	errc := make(chan error, 1)
	for i := range n {
		if i%2 == 0 {
			go func() { errc <- a.Wait() }() // want `goroutine does not propagate context "ctx"`
		} else {
			go func() { errc <- b.Wait() }() // want `goroutine does not propagate context "ctx"`
		}
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Each branch of an if starts a sender, on an unbuffered channel
//
// The send blocks for good once the caller stops receiving (#242).
func badBranchSendersUnbufferedGoroutine(ctx context.Context, a, b *exec.Cmd) error {
	errc := make(chan error)
	if conditionFlag {
		go func() { errc <- a.Wait() }() // want `goroutine does not propagate context "ctx"`
	} else {
		go func() { errc <- b.Wait() }() // want `goroutine does not propagate context "ctx"`
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A caller of a captured sender is started, and another in a closure called in place
//
// Both callers run, so two senders share the one slot.
func badCapturedSenderCallerAndInClosureGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	go func() { send() }() // want `goroutine does not propagate context "ctx"`
	func() {
		go func() { send() }() // want `goroutine does not propagate context "ctx"`
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ===== Senders at sites that exclude each other, in closures or used twice (#354, #355, #356) =====

// [GOOD]: Each branch of an if starts a caller of a captured sender, in a closure called in place
//
// Only one of the closures runs, so the sender runs once.
func goodCapturedSenderBranchCallersInClosureGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	if conditionFlag {
		func() { go func() { send() }() }()
	} else {
		func() { go func() { send() }() }()
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: Each branch of an if starts a goroutine that calls a captured sender from a closure called in place
//
// Only one of the goroutines runs, so the sender runs once.
func goodCapturedSenderBranchNestedCallersGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	if conditionFlag {
		go func() { func() { send() }() }()
	} else {
		go func() { func() { send() }() }()
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: Each branch of an if starts a caller of a captured sender, one through another variable
//
// Only one of the branches runs, so the sender runs once.
func goodCapturedSenderBranchCallersRespilledGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	if conditionFlag {
		again := send
		go func() { again() }()
	} else {
		go func() { send() }()
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A caller of a captured sender through another variable, and another caller
//
// Both callers run, so two senders share the one slot.
func badCapturedSenderRespilledAndCallerGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	again := send
	go func() { again() }() // want `goroutine does not propagate context "ctx"`
	go func() { send() }()  // want `goroutine does not propagate context "ctx"`
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A closure called in place starts two callers of a captured sender
//
// Both callers run, so two senders share the one slot.
func badCapturedSenderTwoCallersInClosureGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	func() {
		go func() { send() }() // want `goroutine does not propagate context "ctx"`
		go func() { send() }() // want `goroutine does not propagate context "ctx"`
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A caller of a captured sender in a closure called in a branch, and another after the if
//
// Both run when the branch is taken, so two senders share the one slot.
func badCapturedSenderBranchCallerInClosureAndAfterGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	if conditionFlag {
		func() { go func() { send() }() }() // want `goroutine does not propagate context "ctx"`
	}
	func() { go func() { send() }() }() // want `goroutine does not propagate context "ctx"`
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A goroutine calls a captured sender from two closures called in place
//
// Both closures run, so the sender runs twice on the one slot.
func badCapturedSenderNestedTwiceGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	go func() { // want `goroutine does not propagate context "ctx"`
		func() { send() }()
		func() { send() }()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Each branch of an if starts a caller of a captured sender in a closure called in place, in a loop
//
// The branches run for each pass of the loop, so senders pile up.
func badCapturedSenderBranchCallersInClosureInLoopGoroutine(ctx context.Context, cmd *exec.Cmd, n int) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	for i := range n {
		if i%2 == 0 {
			func() { go func() { send() }() }() // want `goroutine does not propagate context "ctx"`
		} else {
			func() { go func() { send() }() }() // want `goroutine does not propagate context "ctx"`
		}
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: Each branch of an if starts one sender closure
//
// Only one of the branches runs, so the sender runs once.
func goodSenderBranchStartsGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	if conditionFlag {
		go send()
	} else {
		go send()
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: One sender closure started in a branch, and again after the if
//
// Both run when the branch is taken, so two senders share the one slot.
func badSenderBranchStartAndAfterGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	if conditionFlag {
		go send() // want `goroutine does not propagate context "ctx"`
	}
	go send() // want `goroutine does not propagate context "ctx"`
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Each branch of an if starts one sender closure, in a loop
//
// The branches run for each pass of the loop, so senders pile up.
func badSenderBranchStartsInLoopGoroutine(ctx context.Context, cmd *exec.Cmd, n int) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	for i := range n {
		if i%2 == 0 {
			go send() // want `goroutine does not propagate context "ctx"`
		} else {
			go send() // want `goroutine does not propagate context "ctx"`
		}
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: Each branch of an if starts one sender closure, on an unbuffered channel
//
// The send blocks for good once the caller stops receiving (#242).
func badSenderBranchStartsUnbufferedGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error)
	send := func() { errc <- cmd.Wait() }
	if conditionFlag {
		go send() // want `goroutine does not propagate context "ctx"`
	} else {
		go send() // want `goroutine does not propagate context "ctx"`
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: One branch of an if starts one sender closure twice
//
// Both starts run when the branch is taken, so the sender runs twice.
func badSenderBranchStartsTwiceGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	if conditionFlag {
		go send() // want `goroutine does not propagate context "ctx"`
		go send() // want `goroutine does not propagate context "ctx"`
	} else {
		go send() // want `goroutine does not propagate context "ctx"`
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: A closure called in place starts a caller of a captured sender loaded twice and joined by a Phi
//
// The Phi holds the caller either way, and it runs once.
func goodCapturedSenderPhiLoadsInClosureGoroutine(ctx context.Context, cmd *exec.Cmd, again bool) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	run := func() { send() }
	if conditionFlag {
		run = func() { send() }
	}
	func() {
		p := run
		if again {
			p = run
		}
		go p()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [GOOD]: A closure called in place starts a caller of a captured sender in each branch of an if
//
// Only one of the branches runs, so the caller runs once.
func goodCapturedSenderBranchLoadsInClosureGoroutine(ctx context.Context, cmd *exec.Cmd, again bool) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	run := func() { send() }
	if conditionFlag {
		run = func() { send() }
	}
	func() {
		if again {
			go run()
		} else {
			go run()
		}
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A closure called in place starts a caller of a captured sender joined by a Phi, and again
//
// Both starts run, so two senders share the one slot.
func badCapturedSenderPhiLoadsInClosureAndAgainGoroutine(ctx context.Context, cmd *exec.Cmd, again bool) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	run := func() { send() }
	if conditionFlag {
		run = func() { send() }
	}
	func() {
		p := run
		if again {
			p = run
		}
		go p()   // want `goroutine does not propagate context "ctx"`
		go run() // want `goroutine does not propagate context "ctx"`
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// [BAD]: A caller of a captured sender started from one variable, then from the same variable set again
//
// The two starts both run, so two senders share the one slot.
func badCapturedSenderStartedAgainGoroutine(ctx context.Context, cmd *exec.Cmd) error {
	errc := make(chan error, 1)
	send := func() { errc <- cmd.Wait() }
	run := func() { send() }
	func() { go run() }() // want `goroutine does not propagate context "ctx"`
	run = func() { send() }
	func() { go run() }() // want `goroutine does not propagate context "ctx"`
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
