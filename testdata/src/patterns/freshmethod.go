// A method given only a fresh context, judged by whether its receiver can
// hold a context (#274).

package patterns

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/example/ctxworker"
)

// ===== Method given a fresh context, receiver that holds none =====

// [BAD]: Method of an empty struct given a fresh context
//
// The receiver can hold no context, so the method does not get ctx.
func badEmptyRecvMethodFreshCtxGoroutine(ctx context.Context) {
	go emptyRecv{}.wait(context.Background()) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Method of a pointer to an empty struct given a fresh context
//
// The receiver can hold no context, so the method does not get ctx.
func badEmptyRecvPtrMethodFreshCtxGoroutine(ctx context.Context) {
	r := &emptyRecv{}
	go r.wait(context.TODO()) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Method of a struct of basic types given a fresh context
//
// The receiver can hold no context, so the method does not get ctx.
func badBasicRecvMethodFreshCtxGoroutine(ctx context.Context) {
	go basicRecv{name: "a", n: 1}.wait(context.Background()) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Factory method of an empty struct given a fresh context
//
// The receiver can hold no context, so the func the method returns does not
// get ctx.
func badEmptyRecvFactoryMethodFreshCtxGoroutine(ctx context.Context) {
	go emptyRecv{}.make(context.Background())() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Factory method of an empty struct given a fresh context
//
// The receiver can hold no context, so the func the method returns does not
// get ctx.
func badEmptyRecvFactoryMethodFreshCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(emptyRecv{}.makeErr(context.Background())) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Factory method of an empty struct given a fresh context
//
// The receiver can hold no context, so the func the method returns does not
// get ctx.
func badEmptyRecvFactoryMethodFreshCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(emptyRecv{}.make(context.TODO())) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Factory method of an empty struct given a fresh context
//
// The receiver can hold no context, so the func the method returns does not
// get ctx.
func badEmptyRecvFactoryMethodFreshCtxSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, emptyRecv{}.makeErr(context.Background())) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Method of another package, of an empty struct, given a fresh context
//
// The receiver can hold no context, so the method does not get ctx.
func badOtherPkgEmptyRecvMethodFreshCtxGoroutine(ctx context.Context) {
	go ctxworker.Plain{}.Run(context.Background()) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Factory method of another package, of an empty struct, given a fresh context
//
// The receiver can hold no context, so the func the method returns does not
// get ctx.
func badOtherPkgEmptyRecvFactoryMethodFreshCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(ctxworker.Plain{}.MakeErr(context.Background())) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [GOOD]: Method of an empty struct given ctx
//
// The method is given ctx.
func goodEmptyRecvMethodCtxGoroutine(ctx context.Context) {
	go emptyRecv{}.wait(ctx)
}

// [GOOD]: Factory method of an empty struct given ctx
//
// The method is given ctx.
func goodEmptyRecvFactoryMethodCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(emptyRecv{}.makeErr(ctx))
	_ = g.Wait()
}

// ===== Method given a fresh context, receiver that may hold one =====

// [GOOD]: Method of a struct with a context field given a fresh context
//
// The method may get ctx from its receiver.
func goodCtxRecvMethodFreshCtxGoroutine(ctx context.Context) {
	r := &ctxRecv{ctx: ctx}
	go r.wait(context.Background())
}

// [GOOD]: Factory method of a struct with a context field given a fresh context
//
// The method may get ctx from its receiver.
func goodCtxRecvFactoryMethodFreshCtxErrgroup(ctx context.Context) {
	r := ctxRecv{ctx: ctx}
	g := new(errgroup.Group)
	g.Go(r.makeErr(context.Background()))
	_ = g.Wait()
}

// [GOOD]: Method of a struct with an interface field given a fresh context
//
// An interface may hold a context, so the method may get ctx from its
// receiver.
func goodAnyRecvMethodFreshCtxGoroutine(ctx context.Context) {
	go anyRecv{v: ctx}.wait(context.Background())
}

// ===== Method value given a fresh context (#323) =====

// [BAD]: Method value of an empty struct given a fresh context
//
// The method value holds a receiver that can hold no context, so the method
// does not get ctx, the same as when it is called directly.
func badEmptyRecvMethodValueFreshCtxGoroutine(ctx context.Context) {
	run := emptyRecv{}.wait
	go run(context.Background()) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Method value of a pointer to an empty struct through a Phi given a fresh context
//
// Each value the Phi joins is a method value whose receiver can hold no
// context.
func badEmptyRecvPtrMethodValuePhiFreshCtxGoroutine(ctx context.Context) {
	run := (&emptyRecv{}).wait
	if freshMethodFlag {
		run = basicRecv{name: "a"}.wait
	}
	go run(context.TODO()) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Factory method value of an empty struct given a fresh context
//
// The receiver can hold no context, so the func the method returns does not
// get ctx.
func badEmptyRecvFactoryMethodValueFreshCtxErrgroup(ctx context.Context) {
	mk := emptyRecv{}.makeErr
	g := new(errgroup.Group)
	g.Go(mk(context.Background())) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [GOOD]: Method value of an empty struct given ctx
//
// The method is given ctx.
func goodEmptyRecvMethodValueCtxGoroutine(ctx context.Context) {
	run := emptyRecv{}.wait
	go run(ctx)
}

// [GOOD]: Method value of a struct with a context field given a fresh context
//
// The method may get ctx from the receiver the method value holds.
func goodCtxRecvMethodValueFreshCtxGoroutine(ctx context.Context) {
	run := (&ctxRecv{ctx: ctx}).wait
	go run(context.Background())
}

// [GOOD]: Method value of an empty struct or a struct with a context field through a Phi given a fresh context
//
// One of the receivers the Phi joins may hold ctx.
func goodMixedRecvMethodValuePhiFreshCtxGoroutine(ctx context.Context) {
	run := emptyRecv{}.wait
	if freshMethodFlag {
		run = (&ctxRecv{ctx: ctx}).wait
	}
	go run(context.Background())
}

// [GOOD]: Method value of an interface given a fresh context
//
// The value behind the interface may hold ctx.
func goodIfaceMethodValueFreshCtxGoroutine(ctx context.Context) {
	var w interface{ wait(context.Context) } = &ctxRecv{ctx: ctx}
	run := w.wait
	go run(context.Background())
}

// ===== Method expression, or captured method value, given a fresh context (#337) =====

// [BAD]: Method expression of an empty struct given a fresh context
//
// A method expression takes its receiver as the first argument, which can
// hold no context, so the method does not get ctx.
func badEmptyRecvMethodExprFreshCtxGoroutine(ctx context.Context) {
	go emptyRecv.wait(emptyRecv{}, context.Background()) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Method expression of an empty struct in a variable given a fresh context
//
// The variable holds only the method expression.
func badEmptyRecvMethodExprVarFreshCtxGoroutine(ctx context.Context) {
	m := emptyRecv.wait
	go m(emptyRecv{}, context.TODO()) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Method expression of a pointer to an empty struct given a fresh context
//
// The receiver can hold no context, so the method does not get ctx.
func badEmptyRecvPtrMethodExprFreshCtxGoroutine(ctx context.Context) {
	go (*emptyRecv).wait(&emptyRecv{}, context.Background()) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Captured method value of an empty struct called with a fresh context
//
// The method value holds a receiver that can hold no context, so the method
// does not get ctx, the same as when the closure calls it directly.
func badEmptyRecvCapturedMethodValueFreshCtxGoroutine(ctx context.Context) {
	run := emptyRecv{}.wait
	go func() { // want `goroutine does not propagate context "ctx"`
		run(context.Background())
	}()
}

// [BAD]: Captured method expression of an empty struct called with a fresh context
//
// The receiver the closure gives can hold no context.
func badEmptyRecvCapturedMethodExprFreshCtxErrgroup(ctx context.Context) {
	m := basicRecv.wait
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		m(basicRecv{name: "a"}, context.Background())
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Method expression of an empty struct given ctx
//
// The method is given ctx.
func goodEmptyRecvMethodExprCtxGoroutine(ctx context.Context) {
	go emptyRecv.wait(emptyRecv{}, ctx)
}

// [GOOD]: Method expression of a struct with a context field given a fresh context
//
// The method may get ctx from the receiver given first.
func goodCtxRecvMethodExprFreshCtxGoroutine(ctx context.Context) {
	go (*ctxRecv).wait(&ctxRecv{ctx: ctx}, context.Background())
}

// [GOOD]: Method expression of an interface given a fresh context
//
// The value behind the interface may hold ctx.
func goodIfaceMethodExprFreshCtxGoroutine(ctx context.Context) {
	var w freshMethodWaiter = &ctxRecv{ctx: ctx}
	go freshMethodWaiter.wait(w, context.Background())
}

// [GOOD]: Captured method value of a struct with a context field called with a fresh context
//
// The method may get ctx from the receiver the method value holds.
func goodCtxRecvCapturedMethodValueFreshCtxGoroutine(ctx context.Context) {
	run := (&ctxRecv{ctx: ctx}).wait
	go func() {
		run(context.Background())
	}()
}

// [GOOD]: Captured method value of an empty struct called with ctx
//
// The method is given ctx.
func goodEmptyRecvCapturedMethodValueCtxGoroutine(ctx context.Context) {
	run := emptyRecv{}.wait
	go func() {
		run(ctx)
	}()
}

type freshMethodWaiter interface{ wait(context.Context) }

var freshMethodFlag bool

type emptyRecv struct{}

func (emptyRecv) wait(ctx context.Context) { <-ctx.Done() }

func (emptyRecv) make(ctx context.Context) func() {
	return func() { <-ctx.Done() }
}

func (emptyRecv) makeErr(ctx context.Context) func() error {
	return func() error { return ctx.Err() }
}

type basicRecv struct {
	name string
	n    int
}

func (r basicRecv) wait(ctx context.Context) {
	_, _ = r.name, r.n
	<-ctx.Done()
}

type ctxRecv struct{ ctx context.Context }

func (r *ctxRecv) wait(_ context.Context) { <-r.ctx.Done() }

func (r ctxRecv) makeErr(_ context.Context) func() error {
	return func() error { return r.ctx.Err() }
}

type anyRecv struct{ v any }

func (r anyRecv) wait(_ context.Context) { _ = r.v }
