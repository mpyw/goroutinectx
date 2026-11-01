// Package reportcalleewaiters holds the named functions that only wait and
// signal of reportcallee, run with -report-callee-without-context and
// -report-waiters both set.
package reportcalleewaiters

import (
	"context"
	"sync"
)

// waitAll only waits and signals.
func waitAll(wg *sync.WaitGroup, done chan struct{}) {
	wg.Wait()
	close(done)
}

// waitGroup only waits and signals in its method.
type waitGroup struct {
	wg   sync.WaitGroup
	done chan struct{}
}

func (w *waitGroup) waitAll() {
	w.wg.Wait()
	close(w.done)
}

// [BAD]: A named function that only waits and signals
//
// -report-waiters reports it too.
func badWaiter(ctx context.Context, wg *sync.WaitGroup) {
	done := make(chan struct{})
	go waitAll(wg, done) // want `goroutine does not propagate context "ctx"`
	<-done
}

// [BAD]: A method value that only waits and signals
//
// -report-waiters reports it too.
func badWaiterMethodValue(ctx context.Context, w *waitGroup) {
	f := w.waitAll
	go f() // want `goroutine does not propagate context "ctx"`
	<-w.done
}
