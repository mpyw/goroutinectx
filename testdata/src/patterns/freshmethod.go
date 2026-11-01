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
