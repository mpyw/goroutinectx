// The channel of ctx.Done(), or a func that cancels a context made from
// ctx, taken by a goroutine instead of ctx itself (#229).

package patterns

import (
	"context"
	"errors"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// ===== Channel of ctx.Done() =====

// [GOOD]: Channel of ctx.Done()
//
// The goroutine ends when ctx is done.
func goodCapturedDoneGoroutine(ctx context.Context) {
	done := ctx.Done()
	go func() { <-done }()
}

// [GOOD]: Channel of ctx.Done()
//
// The goroutine ends when ctx is done.
func goodCapturedDoneErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	done := ctx.Done()
	g.Go(func() error { <-done; return nil })
	_ = g.Wait()
}

// [GOOD]: Channel of ctx.Done()
//
// The goroutine ends when ctx is done.
func goodCapturedDoneWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	done := ctx.Done()
	wg.Go(func() { <-done })
	wg.Wait()
}

// [GOOD]: Channel of ctx.Done()
//
// The goroutine ends when ctx is done.
func goodCapturedDoneSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	done := ctx.Done()
	runWithGroup(g, func() error { <-done; return nil })
	_ = g.Wait()
}

// [GOOD]: Channel of ctx.Done() given as an argument
//
// The goroutine ends when ctx is done.
func goodDoneArgGoroutine(ctx context.Context) {
	go func(done <-chan struct{}) { <-done }(ctx.Done())
}

// [GOOD]: Channel of ctx.Done() in a field of a captured struct
//
// The channel is read from a part of a captured value.
func goodDoneFieldGoroutine(ctx context.Context) {
	w := &doneWaiter{done: ctx.Done()}
	go func() { <-w.done }()
}

// [GOOD]: Channel of ctx.Done() set on one branch
//
// The goroutine may end when ctx is done.
func goodDoneOneBranchGoroutine(ctx context.Context, stop chan struct{}) {
	var done <-chan struct{} = stop
	if conditionFlag {
		done = ctx.Done()
	}
	go func() { <-done }()
}

// [GOOD]: Channel of the Done method of a struct that embeds ctx
//
// The struct is a context made from ctx.
func goodWrapperDoneGoroutine(ctx context.Context) {
	w := ctxWrapper{Context: ctx, user: "u"}
	done := w.Done()
	go func() { <-done }()
}

// [NOTCHECKED]: Channel from a method value of ctx.Done
//
// The method value is not followed to its receiver, so the channel does not
// count.
func notCheckedDoneMethodValueGoroutine(ctx context.Context) {
	doneOf := ctx.Done
	done := doneOf()
	go func() { <-done }() // want `goroutine does not propagate context "ctx"`
}

// ===== Channel that is not of ctx =====

// [BAD]: Channel that is not of ctx
//
// A channel the function makes does not tie the goroutine to ctx.
func badCapturedChanGoroutine(ctx context.Context) {
	ch := make(chan struct{})
	go func() { <-ch }() // want `goroutine does not propagate context "ctx"`
	close(ch)
}

// [BAD]: Channel that is not of ctx
//
// A channel the function makes does not tie the goroutine to ctx.
func badCapturedChanErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan struct{})
	g.Go(func() error { <-ch; return nil }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	close(ch)
	_ = g.Wait()
}

// [BAD]: Channel that is not of ctx
//
// A channel the function makes does not tie the goroutine to ctx.
func badCapturedChanWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	ch := make(chan struct{})
	wg.Go(func() { <-ch }) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	close(ch)
	wg.Wait()
}

// [BAD]: Channel that is not of ctx
//
// A channel the function makes does not tie the goroutine to ctx.
func badCapturedChanSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan struct{})
	runWithGroup(g, func() error { <-ch; return nil }) // want `runWithGroup\(\) func argument should use context "ctx"`
	close(ch)
	_ = g.Wait()
}

// [BAD]: Channel of the Done method of a fresh context
//
// context.Background() carries nothing of ctx.
func badFreshDoneGoroutine(ctx context.Context) {
	done := context.Background().Done()
	go func() { <-done }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Channel that is not of ctx, given as an argument
//
// A channel the function makes does not tie the goroutine to ctx.
func badChanArgGoroutine(ctx context.Context) {
	ch := make(chan struct{})
	go func(done <-chan struct{}) { <-done }(ch) // want `goroutine does not propagate context "ctx"`
	close(ch)
}

// [NOTCHECKED]: Channel from a parameter
//
// Where the channel comes from cannot be followed, so it does not count.
func notCheckedParamChanGoroutine(ctx context.Context, done <-chan struct{}) {
	go func() { <-done }() // want `goroutine does not propagate context "ctx"`
}

// ===== Channel of ctx.Done() given to a named function =====

// [GOOD]: Channel of ctx.Done() given to a named function with a fresh context
//
// The function ends when ctx is done, as a function literal given the same
// arguments does (#241).
func goodDoneToNamedGoroutine(ctx context.Context) {
	go waitDoneOrContext(context.Background(), ctx.Done())
}

// [GOOD]: Channel of ctx.Done() in a variable, given to a named function with a fresh context
//
// The variable is followed to ctx.Done() (#241).
func goodDoneVarToNamedGoroutine(ctx context.Context) {
	done := ctx.Done()
	go waitDoneOrContext(context.Background(), done)
}

// [GOOD]: Channel of ctx.Done() given to a factory with a fresh context
//
// The func the factory returns ends when ctx is done (#241).
func goodDoneToFactoryGoroutine(ctx context.Context) {
	go makeDoneWaiter(context.Background(), ctx.Done())()
}

// [GOOD]: Channel of ctx.Done() given to a factory with a fresh context
//
// The func the factory returns ends when ctx is done (#241).
func goodDoneToFactoryErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(makeDoneWaiterErr(context.Background(), ctx.Done()))
	_ = g.Wait()
}

// [GOOD]: Channel of ctx.Done() given to a factory with a fresh context
//
// The func the factory returns ends when ctx is done (#241).
func goodDoneToFactoryWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(makeDoneWaiter(context.Background(), ctx.Done()))
	wg.Wait()
}

// [GOOD]: Channel of ctx.Done() given to a factory with a fresh context
//
// The func the factory returns ends when ctx is done (#241).
func goodDoneToFactorySpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, makeDoneWaiterErr(context.Background(), ctx.Done()))
	_ = g.Wait()
}

// [BAD]: Channel that is not of ctx, given to a named function with a fresh context
//
// Neither the channel nor context.Background() carries anything of ctx.
func badChanToNamedGoroutine(ctx context.Context) {
	ch := make(chan struct{})
	go waitDoneOrContext(context.Background(), ch) // want `goroutine does not propagate context "ctx"`
	close(ch)
}

// [BAD]: Channel of a fresh context, given to a named function with a fresh context
//
// context.Background().Done() carries nothing of ctx.
func badFreshDoneToNamedGoroutine(ctx context.Context) {
	go waitDoneOrContext(context.Background(), context.Background().Done()) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Channel that is not of ctx, given to a factory with a fresh context
//
// Neither the channel nor context.Background() carries anything of ctx.
func badChanToFactoryGoroutine(ctx context.Context) {
	ch := make(chan struct{})
	go makeDoneWaiter(context.Background(), ch)() // want `goroutine does not propagate context "ctx"`
	close(ch)
}

// [BAD]: Channel that is not of ctx, given to a factory with a fresh context
//
// Neither the channel nor context.Background() carries anything of ctx.
func badChanToFactoryErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan struct{})
	g.Go(makeDoneWaiterErr(context.Background(), ch)) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	close(ch)
	_ = g.Wait()
}

// [BAD]: Channel that is not of ctx, given to a factory with a fresh context
//
// Neither the channel nor context.Background() carries anything of ctx.
func badChanToFactoryWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	ch := make(chan struct{})
	wg.Go(makeDoneWaiter(context.Background(), ch)) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	close(ch)
	wg.Wait()
}

// [BAD]: Channel that is not of ctx, given to a factory with a fresh context
//
// Neither the channel nor context.Background() carries anything of ctx.
func badChanToFactorySpawner(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan struct{})
	runWithGroup(g, makeDoneWaiterErr(context.Background(), ch)) // want `runWithGroup\(\) func argument should use context "ctx"`
	close(ch)
	_ = g.Wait()
}

// ===== Cancel func of a context made from ctx =====

// [GOOD]: Cancel func of a context made from ctx
//
// The goroutine cancels a context made from ctx.
func goodCapturedCancelGoroutine(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { waitSignal(); cancel() }()
	_ = ctx
}

// [GOOD]: Cancel func of a context made from ctx
//
// The goroutine cancels a context made from ctx.
func goodCapturedCancelErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	_, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	g.Go(func() error { waitSignal(); cancel(); return nil })
	_ = g.Wait()
}

// [GOOD]: Cancel func of a context made from ctx
//
// The goroutine cancels a context made from ctx.
func goodCapturedCancelWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	_, cancel := context.WithDeadline(ctx, time.Now().Add(time.Second))
	defer cancel()
	wg.Go(func() { waitSignal(); cancel() })
	wg.Wait()
}

// [GOOD]: Cancel func of a context made from ctx
//
// The goroutine cancels a context made from ctx.
func goodCapturedCancelSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	_, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	runWithGroup(g, func() error { waitSignal(); cancel(errors.New("stop")); return nil })
	_ = g.Wait()
}

// [GOOD]: Cancel func of the Cause forms
//
// WithTimeoutCause and WithDeadlineCause give a cancel func too.
func goodCapturedCancelCauseFormsGoroutine(ctx context.Context) {
	_, cancelT := context.WithTimeoutCause(ctx, time.Second, errors.New("timeout"))
	_, cancelD := context.WithDeadlineCause(ctx, time.Now(), errors.New("deadline"))
	go func() { cancelT() }()
	go func() { cancelD() }()
}

// [GOOD]: Cancel func given as an argument
//
// The goroutine cancels a context made from ctx.
func goodCancelArgGoroutine(ctx context.Context) {
	_, cancel := context.WithCancel(ctx)
	go func(stop context.CancelFunc) { waitSignal(); stop() }(cancel)
}

// [GOOD]: Cancel func spawned directly
//
// The cancel func of a context made from ctx is spawned itself.
func goodCancelSpawnedGoroutine(ctx context.Context) {
	_, cancel := context.WithCancel(ctx)
	go cancel()
}

// ===== Cancel func of a fresh context =====

// [BAD]: Cancel func of a fresh context
//
// The context it cancels is made from context.Background(), not ctx.
func badCapturedFreshCancelGoroutine(ctx context.Context) {
	_, cancel := context.WithCancel(context.Background())
	go func() { waitSignal(); cancel() }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Cancel func of a fresh context
//
// The context it cancels is made from context.Background(), not ctx.
func badCapturedFreshCancelErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	_, cancel := context.WithTimeout(context.Background(), time.Second)
	g.Go(func() error { waitSignal(); cancel(); return nil }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Cancel func of a fresh context
//
// The context it cancels is made from context.Background(), not ctx.
func badCapturedFreshCancelWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	_, cancel := context.WithCancel(context.TODO())
	wg.Go(func() { waitSignal(); cancel() }) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Cancel func of a fresh context
//
// The context it cancels is made from context.Background(), not ctx.
func badCapturedFreshCancelSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	_, cancel := context.WithCancel(context.Background())
	runWithGroup(g, func() error { waitSignal(); cancel(); return nil }) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Cancel func of a fresh context, given as an argument
//
// The context it cancels is made from context.Background(), not ctx.
func badFreshCancelArgGoroutine(ctx context.Context) {
	_, cancel := context.WithCancel(context.Background())
	go func(stop context.CancelFunc) { waitSignal(); stop() }(cancel) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Cancel func of a fresh context, spawned directly
//
// The context it cancels is made from context.Background(), not ctx.
func badFreshCancelSpawnedGoroutine(ctx context.Context) {
	_, cancel := context.WithCancel(context.Background())
	go cancel() // want `goroutine does not propagate context "ctx"`
}

// [NOTCHECKED]: Cancel func from a parameter
//
// Where the func comes from cannot be followed, so it passes, as go stop()
// does.
func notCheckedParamCancelGoroutine(ctx context.Context, stop context.CancelFunc) {
	go func() { waitSignal(); stop() }()
}

// doneWaiter holds a channel to wait on.
type doneWaiter struct{ done <-chan struct{} }

// waitSignal waits for something other than ctx.
func waitSignal() {}

// waitDoneOrContext waits until done is closed or bg is done.
func waitDoneOrContext(bg context.Context, done <-chan struct{}) {
	select {
	case <-done:
	case <-bg.Done():
	}
}

// makeDoneWaiter gives a func that waits as waitDoneOrContext does.
func makeDoneWaiter(bg context.Context, done <-chan struct{}) func() {
	return func() { waitDoneOrContext(bg, done) }
}

// makeDoneWaiterErr gives a func that waits as waitDoneOrContext does.
func makeDoneWaiterErr(bg context.Context, done <-chan struct{}) func() error {
	return func() error { waitDoneOrContext(bg, done); return nil }
}
