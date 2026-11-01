// A context written as an alias (#181), as golang.org/x/net/context, as a
// type parameter (#186), or as an interface that embeds context.Context
// (#194).

package patterns

import (
	"context"
	"sync"
	"time"

	netcontext "golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
)

// Ctx is an alias of context.Context.
type Ctx = context.Context

func workCtx(ctx Ctx) { _ = ctx }

// ===== Context through an alias =====

// [BAD]: Alias ctx param without use
//
// A parameter of an alias of context.Context is a context.
func badAliasCtxParamGoroutine(ctx Ctx) {
	go func() { // want `goroutine does not propagate context "ctx"`
	}()
}

// [BAD]: Alias ctx param without use
//
// A parameter of an alias of context.Context is a context.
func badAliasCtxParamErrgroup(ctx Ctx) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Alias ctx param without use
//
// A parameter of an alias of context.Context is a context.
func badAliasCtxParamWaitGroup(ctx Ctx) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	})
	wg.Wait()
}

// [BAD]: Alias ctx param without use
//
// A parameter of an alias of context.Context is a context.
func badAliasCtxParamSpawner(ctx Ctx) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error { // want `runWithGroup\(\) func argument should use context "ctx"`
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Alias ctx param used
//
// The closure uses the alias-typed context.
func goodAliasCtxParamGoroutine(ctx Ctx) {
	go func() {
		workCtx(ctx)
	}()
}

// [GOOD]: Alias ctx param used
//
// The closure uses the alias-typed context.
func goodAliasCtxParamErrgroup(ctx Ctx) {
	g := new(errgroup.Group)
	g.Go(func() error {
		workCtx(ctx)
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Alias ctx param used
//
// The closure uses the alias-typed context.
func goodAliasCtxParamWaitGroup(ctx Ctx) {
	var wg sync.WaitGroup
	wg.Go(func() {
		workCtx(ctx)
	})
	wg.Wait()
}

// [GOOD]: Alias ctx param used
//
// The closure uses the alias-typed context.
func goodAliasCtxParamSpawner(ctx Ctx) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error {
		workCtx(ctx)
		return nil
	})
	_ = g.Wait()
}

// ===== Context held in an alias-typed variable =====

// [GOOD]: Alias-typed variable captured
//
// The closure captures a variable of an alias of context.Context.
func goodAliasCtxVariableGoroutine(ctx context.Context) {
	var c Ctx = ctx
	go func() {
		workCtx(c)
	}()
}

// [GOOD]: Alias-typed variable captured
//
// The closure captures a variable of an alias of context.Context.
func goodAliasCtxVariableErrgroup(ctx context.Context) {
	var c Ctx = ctx
	g := new(errgroup.Group)
	g.Go(func() error {
		workCtx(c)
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Alias-typed variable captured
//
// The closure captures a variable of an alias of context.Context.
func goodAliasCtxVariableWaitGroup(ctx context.Context) {
	var c Ctx = ctx
	var wg sync.WaitGroup
	wg.Go(func() {
		workCtx(c)
	})
	wg.Wait()
}

// [GOOD]: Alias-typed variable captured
//
// The closure captures a variable of an alias of context.Context.
func goodAliasCtxVariableSpawner(ctx context.Context) {
	var c Ctx = ctx
	g := new(errgroup.Group)
	runWithGroup(g, func() error {
		workCtx(c)
		return nil
	})
	_ = g.Wait()
}

// ===== golang.org/x/net/context =====

// [BAD]: x/net/context param without use
//
// golang.org/x/net/context.Context is an alias of context.Context.
func badNetContextGoroutine(ctx netcontext.Context) {
	go func() { // want `goroutine does not propagate context "ctx"`
	}()
}

// [BAD]: x/net/context param without use
//
// golang.org/x/net/context.Context is an alias of context.Context.
func badNetContextErrgroup(ctx netcontext.Context) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		return nil
	})
	_ = g.Wait()
}

// [BAD]: x/net/context param without use
//
// golang.org/x/net/context.Context is an alias of context.Context.
func badNetContextWaitGroup(ctx netcontext.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	})
	wg.Wait()
}

// [BAD]: x/net/context param without use
//
// golang.org/x/net/context.Context is an alias of context.Context.
func badNetContextSpawner(ctx netcontext.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error { // want `runWithGroup\(\) func argument should use context "ctx"`
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: x/net/context param used
//
// The closure uses the golang.org/x/net/context context.
func goodNetContextGoroutine(ctx netcontext.Context) {
	go func() {
		workCtx(ctx)
	}()
}

// [GOOD]: x/net/context param used
//
// The closure uses the golang.org/x/net/context context.
func goodNetContextErrgroup(ctx netcontext.Context) {
	g := new(errgroup.Group)
	g.Go(func() error {
		workCtx(ctx)
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: x/net/context param used
//
// The closure uses the golang.org/x/net/context context.
func goodNetContextWaitGroup(ctx netcontext.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		workCtx(ctx)
	})
	wg.Wait()
}

// [GOOD]: x/net/context param used
//
// The closure uses the golang.org/x/net/context context.
func goodNetContextSpawner(ctx netcontext.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error {
		workCtx(ctx)
		return nil
	})
	_ = g.Wait()
}

// ===== Fresh golang.org/x/net/context =====

// [BAD]: Named function given a fresh x/net/context
//
// golang.org/x/net/context.Background() is as fresh as
// context.Background() (#195).
func badNetContextFreshNamedFuncGoroutine(ctx context.Context) {
	go workCtx(netcontext.Background()) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Captured fresh x/net/context
//
// golang.org/x/net/context.TODO() is as fresh as context.TODO() (#195).
func badNetContextFreshCaptureGoroutine(ctx context.Context) {
	bg := netcontext.TODO()
	go func() { // want `goroutine does not propagate context "ctx"`
		workCtx(bg)
	}()
}

// [BAD]: Factory given a fresh x/net/context
//
// golang.org/x/net/context.Background() is as fresh as
// context.Background() (#195).
func badNetContextFreshFactoryErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(makeWorkerWithCtxErrgroup(netcontext.Background())) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Captured fresh x/net/context
//
// golang.org/x/net/context.TODO() is as fresh as context.TODO() (#195).
func badNetContextFreshCaptureWaitGroup(ctx context.Context) {
	bg := netcontext.TODO()
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		workCtx(bg)
	})
	wg.Wait()
}

// [BAD]: Factory given a fresh x/net/context
//
// golang.org/x/net/context.Background() is as fresh as
// context.Background() (#195).
func badNetContextFreshFactorySpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, makeWorkerWithCtxSpawner(netcontext.Background())) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== Context as a type parameter =====

// [BAD]: Type parameter ctx without use
//
// A type parameter constrained by context.Context is a context.
func badTypeParamCtxGoroutine[C context.Context](ctx C) {
	go func() { // want `goroutine does not propagate context "ctx"`
	}()
}

// [BAD]: Type parameter ctx without use
//
// A type parameter constrained by context.Context is a context.
func badTypeParamCtxErrgroup[C context.Context](ctx C) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Type parameter ctx without use
//
// A type parameter constrained by context.Context is a context.
func badTypeParamCtxWaitGroup[C context.Context](ctx C) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	})
	wg.Wait()
}

// [BAD]: Type parameter ctx without use
//
// A type parameter constrained by context.Context is a context.
func badTypeParamCtxSpawner[C context.Context](ctx C) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error { // want `runWithGroup\(\) func argument should use context "ctx"`
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Type parameter ctx used
//
// The closure uses the context of a type parameter.
func goodTypeParamCtxGoroutine[C context.Context](ctx C) {
	go func() {
		workCtx(ctx)
	}()
}

// [GOOD]: Type parameter ctx used
//
// The closure uses the context of a type parameter.
func goodTypeParamCtxErrgroup[C context.Context](ctx C) {
	g := new(errgroup.Group)
	g.Go(func() error {
		workCtx(ctx)
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Type parameter ctx used
//
// The closure uses the context of a type parameter.
func goodTypeParamCtxWaitGroup[C context.Context](ctx C) {
	var wg sync.WaitGroup
	wg.Go(func() {
		workCtx(ctx)
	})
	wg.Wait()
}

// [GOOD]: Type parameter ctx used
//
// The closure uses the context of a type parameter.
func goodTypeParamCtxSpawner[C context.Context](ctx C) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error {
		workCtx(ctx)
		return nil
	})
	_ = g.Wait()
}

// ===== Context as a type parameter with an embedding constraint =====

// [BAD]: Embedding constraint ctx without use
//
// A constraint that embeds context.Context, or an alias of it, makes a
// context too.
func badEmbedConstraintCtxGoroutine[C interface{ Ctx }](ctx C) {
	go func() { // want `goroutine does not propagate context "ctx"`
	}()
}

// [BAD]: Embedding constraint ctx without use
//
// A constraint that embeds context.Context, or an alias of it, makes a
// context too.
func badEmbedConstraintCtxErrgroup[C interface {
	context.Context
	Name() string
}](ctx C) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Embedding constraint ctx used
//
// The closure uses the context of a type parameter.
func goodEmbedConstraintCtxGoroutine[C interface{ Ctx }](ctx C) {
	go func() {
		workCtx(ctx)
	}()
}

// [GOOD]: Non-context type parameter
//
// A type parameter whose constraint does not hold context.Context gives no
// context.
func goodNonCtxTypeParamGoroutine[T any](v T) {
	go func() {
	}()
}

// ===== Context as a named interface that embeds it =====

// MyCtx is a named interface that embeds context.Context.
type MyCtx interface{ context.Context }

// NamedCtx embeds MyCtx and adds a method.
type NamedCtx interface {
	MyCtx
	Name() string
}

// doneOnly has one of the methods of context.Context, and does not embed it.
type doneOnly interface {
	Done() <-chan struct{}
}

// ownMethods declares the methods of context.Context itself.
type ownMethods interface {
	Deadline() (deadline time.Time, ok bool)
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}

func myCtxWorker(c MyCtx) func() error {
	return func() error {
		workCtx(c)
		return nil
	}
}

func myCtxTask(c MyCtx) func() {
	return func() {
		workCtx(c)
	}
}

// [BAD]: Named interface ctx param without use
//
// A parameter of a named interface that embeds context.Context is a context.
func badNamedIfaceCtxParamGoroutine(ctx MyCtx) {
	go func() { // want `goroutine does not propagate context "ctx"`
	}()
}

// [BAD]: Named interface ctx param without use
//
// A parameter of a named interface that embeds context.Context is a context.
func badNamedIfaceCtxParamErrgroup(ctx MyCtx) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Named interface ctx param without use
//
// A parameter of a named interface that embeds context.Context is a context.
func badNamedIfaceCtxParamWaitGroup(ctx MyCtx) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	})
	wg.Wait()
}

// [BAD]: Named interface ctx param without use
//
// A parameter of a named interface that embeds context.Context is a context.
func badNamedIfaceCtxParamSpawner(ctx MyCtx) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error { // want `runWithGroup\(\) func argument should use context "ctx"`
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Nested named interface ctx param without use
//
// An interface that embeds one that embeds context.Context is a context.
func badNestedNamedIfaceCtxParamGoroutine(ctx NamedCtx) {
	go func() { // want `goroutine does not propagate context "ctx"`
	}()
}

// [BAD]: Interface literal ctx param without use
//
// A parameter of an interface literal that embeds context.Context is a
// context.
func badIfaceLitCtxParamGoroutine(ctx interface{ context.Context }) {
	go func() { // want `goroutine does not propagate context "ctx"`
	}()
}

// [BAD]: Named interface constraint ctx without use
//
// A type parameter constrained by a named interface that embeds
// context.Context is a context.
func badNamedIfaceConstraintCtxGoroutine[C MyCtx](ctx C) {
	go func() { // want `goroutine does not propagate context "ctx"`
	}()
}

// [BAD]: Named interface constraint ctx without use
//
// A type parameter constrained by a named interface that embeds
// context.Context is a context.
func badNamedIfaceConstraintCtxErrgroup[C NamedCtx](ctx C) {
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Named interface ctx param used
//
// The closure uses the context of a named interface.
func goodNamedIfaceCtxParamGoroutine(ctx MyCtx) {
	go func() {
		workCtx(ctx)
	}()
}

// [GOOD]: Named interface ctx param used
//
// The closure uses the context of a named interface.
func goodNamedIfaceCtxParamErrgroup(ctx MyCtx) {
	g := new(errgroup.Group)
	g.Go(func() error {
		workCtx(ctx)
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Named interface ctx param used
//
// The closure uses the context of a named interface.
func goodNamedIfaceCtxParamWaitGroup(ctx MyCtx) {
	var wg sync.WaitGroup
	wg.Go(func() {
		workCtx(ctx)
	})
	wg.Wait()
}

// [GOOD]: Named interface ctx param used
//
// The closure uses the context of a named interface.
func goodNamedIfaceCtxParamSpawner(ctx MyCtx) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error {
		workCtx(ctx)
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Named interface constraint ctx used
//
// The closure uses the context of a type parameter.
func goodNamedIfaceConstraintCtxGoroutine[C MyCtx](ctx C) {
	go func() {
		workCtx(ctx)
	}()
}

// [GOOD]: Ctx passed as a named interface argument
//
// The goroutine's own parameter of a named interface that embeds
// context.Context is given ctx.
func goodNamedIfaceCtxArgGoroutine(ctx context.Context) {
	go func(c MyCtx) {
		workCtx(c)
	}(ctx)
}

// [GOOD]: Ctx passed to a factory of a named interface
//
// The factory takes ctx as a named interface that embeds context.Context.
func goodNamedIfaceCtxFactoryGoroutine(ctx context.Context) {
	go myCtxTask(ctx)()
}

// [GOOD]: Ctx passed to a factory of a named interface
//
// The factory takes ctx as a named interface that embeds context.Context.
func goodNamedIfaceCtxFactoryErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(myCtxWorker(ctx))
	_ = g.Wait()
}

// [GOOD]: Ctx passed to a factory of a named interface
//
// The factory takes ctx as a named interface that embeds context.Context.
func goodNamedIfaceCtxFactoryWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(myCtxTask(ctx))
	wg.Wait()
}

// [GOOD]: Ctx passed to a factory of a named interface
//
// The factory takes ctx as a named interface that embeds context.Context.
func goodNamedIfaceCtxFactorySpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, myCtxWorker(ctx))
	_ = g.Wait()
}

// [GOOD]: Interface with only some context methods
//
// An interface that does not embed context.Context is not a context, even
// when it has some of its methods.
func goodPartialCtxIfaceGoroutine(d doneOnly) {
	go func() {
	}()
}

// [LIMITATION]: Interface that declares the context methods itself
//
// An interface that declares every method of context.Context, and does not
// embed it, is not taken as a context. Only an embedded context.Context
// counts, so the goroutine below is not reported.
func limitationOwnMethodsCtxIfaceGoroutine(ctx ownMethods) {
	go func() {
	}()
}

// ===== Captured value with the methods of context.Context =====

// [GOOD]: Captured value with the methods of context.Context, made from ctx
//
// A value whose type implements context.Context counts as a context.
func goodCapturedOwnMethodsGoroutine(ctx context.Context) {
	var c ownMethods = ctx
	go func() { _ = c }()
}

// [GOOD]: Captured value with the methods of context.Context, made from ctx
//
// A value whose type implements context.Context counts as a context.
func goodCapturedOwnMethodsErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	var c ownMethods = ctx
	g.Go(func() error { _ = c; return nil })
	_ = g.Wait()
}

// [GOOD]: Captured value with the methods of context.Context, made from ctx
//
// A value whose type implements context.Context counts as a context.
func goodCapturedOwnMethodsWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	var c ownMethods = ctx
	wg.Go(func() { _ = c })
	wg.Wait()
}

// [GOOD]: Captured value with the methods of context.Context, made from ctx
//
// A value whose type implements context.Context counts as a context.
func goodCapturedOwnMethodsSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	var c ownMethods = ctx
	runWithGroup(g, func() error { _ = c; return nil })
	_ = g.Wait()
}

// [BAD]: Captured value with the methods of context.Context, made fresh
//
// It holds only context.Background(), so it is judged as a context.Context
// that holds one is, and as a field of this type is (#210).
func badCapturedOwnMethodsFreshGoroutine(ctx context.Context) {
	var c ownMethods = context.Background()
	go func() { _ = c }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Captured value with the methods of context.Context, made fresh
//
// It holds only context.Background(), so it is judged as a context.Context
// that holds one is, and as a field of this type is (#210).
func badCapturedOwnMethodsFreshErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	var c ownMethods = context.Background()
	g.Go(func() error { _ = c; return nil }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Captured value with the methods of context.Context, made fresh
//
// It holds only context.Background(), so it is judged as a context.Context
// that holds one is, and as a field of this type is (#210).
func badCapturedOwnMethodsFreshWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	var c ownMethods = context.TODO()
	wg.Go(func() { _ = c }) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Captured value with the methods of context.Context, made fresh
//
// It holds only context.Background(), so it is judged as a context.Context
// that holds one is, and as a field of this type is (#210).
func badCapturedOwnMethodsFreshSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	var c ownMethods = context.Background()
	runWithGroup(g, func() error { _ = c; return nil }) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Field with the methods of context.Context, made fresh
//
// A field read from a captured value is judged the same as a captured
// variable of its type.
func badCapturedOwnMethodsFieldFreshGoroutine(ctx context.Context) {
	h := struct{ c ownMethods }{c: context.Background()}
	go func() { _ = h.c }() // want `goroutine does not propagate context "ctx"`
}

// ===== Spawned function that takes a value that implements context.Context =====

//goroutinectx:spawner
func runWithWrapper(fn func(ctxWrapper)) {
	go fn(ctxWrapper{Context: context.Background()})
}

//goroutinectx:spawner
func runWithService(fn func(*ctxService)) {
	go fn(&ctxService{ctx: context.Background()})
}

// [GOOD]: Goroutine given a value that implements context.Context
//
// The parameter counts as a context, as the same value captured does
// (#210).
func goodGoParamContextImplGoroutine(ctx context.Context, w ctxWrapper) {
	go func(w ctxWrapper) {
		_ = w.user
	}(w)
}

// [GOOD]: Goroutine given a struct that holds a context
//
// The parameter counts as a context, as the same value captured and given
// to a call does (#210).
func goodGoParamHolderGoroutine(ctx context.Context, s *ctxService) {
	go func(s *ctxService) {
		_ = s
	}(s)
}

// [GOOD]: Spawner func argument that takes a value that implements context.Context
//
// The func argument takes a context, as one that takes a context.Context
// does (#210).
func goodSpawnerParamContextImplSpawner(ctx context.Context) {
	runWithWrapper(func(w ctxWrapper) {
		_ = w.user
	})
}

// [GOOD]: Spawner func argument that takes a struct that holds a context
//
// The func argument takes a context, as one that takes a context.Context
// does (#210).
func goodSpawnerParamHolderSpawner(ctx context.Context) {
	runWithService(func(s *ctxService) {
		_ = s
	})
}

// [BAD]: Goroutine given a struct that holds no context
//
// A struct with no context field is not a context.
func badGoParamPlainHolderGoroutine(ctx context.Context, p plainHolder) {
	go func(p plainHolder) { // want `goroutine does not propagate context "ctx"`
		_ = p.name
	}(p)
}

// ===== Goroutine given ctx through a parameter of another type =====

// runner is run by a goroutine it is given to.
type runner interface{ Run() }

// ctxRunner is a runner that holds a context.
type ctxRunner struct{ ctx context.Context }

func (r ctxRunner) Run() { <-r.ctx.Done() }

// plainRunner is a runner that holds no context.
type plainRunner struct{ name string }

func (r plainRunner) Run() { _ = r.name }

// ctxName gives a name made from ctx, which holds no context.
func ctxName(ctx context.Context) string { _ = ctx; return "" }

// [GOOD]: Goroutine given ctx in an interface
//
// The parameter is given a value that holds ctx, as the same value
// captured counts (#252).
func goodGoParamIfaceGoroutine(ctx context.Context) {
	go func(r runner) {
		r.Run()
	}(ctxRunner{ctx})
}

// [GOOD]: Goroutine given a func that uses ctx
//
// The func the goroutine runs captures ctx (#252).
func goodGoParamFuncGoroutine(ctx context.Context) {
	go func(f func()) {
		f()
	}(func() { <-ctx.Done() })
}

// [GOOD]: Goroutine given a slice of contexts
//
// The slice holds ctx (#252).
func goodGoParamSliceGoroutine(ctx context.Context) {
	go func(cs []context.Context) {
		<-cs[0].Done()
	}([]context.Context{ctx})
}

// [GOOD]: Goroutine given ctx as an any
//
// The parameter is ctx in an interface (#252).
func goodGoParamAnyGoroutine(ctx context.Context) {
	go func(v any) {
		<-v.(context.Context).Done()
	}(ctx)
}

// [BAD]: Goroutine given a fresh context as an any
//
// The parameter holds only context.Background(), not ctx.
func badGoParamAnyFreshGoroutine(ctx context.Context) {
	go func(v any) { // want `goroutine does not propagate context "ctx"`
		_ = v
	}(context.Background())
}

// [BAD]: Goroutine given a slice of fresh contexts
//
// The slice holds only context.TODO(), not ctx.
func badGoParamSliceFreshGoroutine(ctx context.Context) {
	go func(cs []context.Context) { // want `goroutine does not propagate context "ctx"`
		_ = cs
	}([]context.Context{context.TODO()})
}

// [BAD]: Goroutine given a func that does not use ctx
//
// The func captures nothing.
func badGoParamFuncNoCtxGoroutine(ctx context.Context) {
	go func(f func()) { // want `goroutine does not propagate context "ctx"`
		f()
	}(func() {})
}

// [BAD]: Goroutine given an interface that holds no context
//
// The runner is made from no context.
func badGoParamIfacePlainGoroutine(ctx context.Context) {
	go func(r runner) { // want `goroutine does not propagate context "ctx"`
		r.Run()
	}(plainRunner{name: "x"})
}

// [BAD]: Goroutine given a value made from ctx, of a type that holds none
//
// A string cannot hold a context, even when ctx was used to make it.
func badGoParamStringFromCtxGoroutine(ctx context.Context) {
	go func(name string) { // want `goroutine does not propagate context "ctx"`
		_ = name
	}(ctxName(ctx))
}

// newCtxRunner gives a runner that holds ctx.
func newCtxRunner(ctx context.Context) runner { return ctxRunner{ctx} }

// newCtxRunnerErr gives a runner that holds ctx, and an error.
func newCtxRunnerErr(ctx context.Context) (runner, error) { return ctxRunner{ctx}, nil }

// ctxRecord is data a call given ctx makes. Its any field may hold
// anything, but it is not made to hold the context.
type ctxRecord struct {
	Name  string
	Extra any
}

// listRecords gives the records it lists with ctx.
func listRecords(ctx context.Context) ([]*ctxRecord, error) { _ = ctx; return nil, nil }

// [GOOD]: Goroutine given an interface a call made from ctx
//
// The interface the call gives may hold what it was given (#252).
func goodGoParamIfaceFromCallGoroutine(ctx context.Context) {
	go func(r runner) {
		r.Run()
	}(newCtxRunner(ctx))
}

// [GOOD]: Goroutine given an interface a call of two results made from ctx
//
// The interface taken from the results may hold what the call was given
// (#252).
func goodGoParamIfaceFromCallErrGoroutine(ctx context.Context) {
	r, _ := newCtxRunnerErr(ctx)
	go func(r runner) {
		r.Run()
	}(r)
}

// [BAD]: Goroutine given data a call made with ctx
//
// The records are data the call listed, not a holder of ctx, though a
// record may hold anything in its any field.
func badGoParamDataFromCallGoroutine(ctx context.Context) {
	records, _ := listRecords(ctx)
	for _, r := range records {
		go func(r *ctxRecord) { // want `goroutine does not propagate context "ctx"`
			_ = r.Name
		}(r)
	}
}

// newCtxService gives a struct that holds ctx.
func newCtxService(ctx context.Context) *ctxService { return &ctxService{ctx: ctx} }

// [GOOD]: Goroutine given, as an any, a struct a call made to hold ctx
//
// The call gives a struct that holds a context (#252).
func goodGoParamHolderFromCallGoroutine(ctx context.Context) {
	go func(v any) {
		_ = v
	}(newCtxService(ctx))
}

// [GOOD]: Goroutine given an interface looked up in a map that holds ctx
//
// The map is made with a runner that holds ctx (#252).
func goodGoParamIfaceFromMapGoroutine(ctx context.Context) {
	runners := map[string]runner{"a": ctxRunner{ctx}}
	r, ok := runners["a"]
	if !ok {
		return
	}
	go func(r runner) {
		r.Run()
	}(r)
}

// [GOOD]: Goroutine given a func parameter it runs
//
// The func cannot be followed, so it may use ctx, as the same func
// captured passes (#296).
func goodGoParamFuncParamGoroutine(ctx context.Context, f func()) {
	go func(g func()) {
		g()
	}(f)
}

// [GOOD]: Goroutine in a closure, given a func parameter it captures
//
// The func the closure captures is a parameter, which cannot be followed
// (#296).
func goodGoParamFuncParamInClosureGoroutine(ctx context.Context, f func()) {
	func() {
		go func(g func()) {
			g()
		}(f)
	}()
}

// [GOOD]: Goroutine given an interface parameter it runs
//
// The runner cannot be followed, and its type is declared in the module,
// so it may hold ctx, as the same runner captured passes (#296).
func goodGoParamIfaceParamGoroutine(ctx context.Context, r runner) {
	go func(x runner) {
		x.Run()
	}(r)
}

// [BAD]: Goroutine given a func parameter it does not run
//
// The func is only compared, as the same func captured would be.
func badGoParamFuncParamNotRunGoroutine(ctx context.Context, f func()) {
	go func(g func()) { // want `goroutine does not propagate context "ctx"`
		_ = g == nil
	}(f)
}

// [BAD]: Goroutine given a func made by a factory that uses no context
//
// The factory returns a func that captures nothing.
func badGoParamFuncFromFactoryGoroutine(ctx context.Context) {
	go func(g func()) { // want `goroutine does not propagate context "ctx"`
		g()
	}(plainFuncFactory())
}

// plainFuncFactory gives a func that uses no context.
func plainFuncFactory() func() { return func() {} }

// [BAD]: Goroutine given an error parameter
//
// An interface of the standard library does not count, as the same error
// captured does not.
func badGoParamErrorParamGoroutine(ctx context.Context, err error) {
	go func(e error) { // want `goroutine does not propagate context "ctx"`
		_ = e.Error()
	}(err)
}

// [GOOD]: Goroutine given a func parameter it runs in a nested closure
//
// The nested closure captures the parameter and runs it in place, as the
// goroutine running it itself does (#322).
func goodGoParamFuncParamNestedGoroutine(ctx context.Context, f func()) {
	go func(g func()) {
		func() { g() }()
	}(f)
}

// [GOOD]: Goroutine given a func parameter it runs in a deferred nested closure
//
// A deferred closure runs it the same (#322).
func goodGoParamFuncParamDeferredGoroutine(ctx context.Context, f func()) {
	go func(g func()) {
		defer func() { g() }()
	}(f)
}

// [GOOD]: Goroutine given an interface parameter it calls in a nested closure
//
// The runner cannot be followed, as the parameter called directly (#322).
func goodGoParamIfaceParamNestedGoroutine(ctx context.Context, r runner) {
	go func(x runner) {
		func() { x.Run() }()
	}(r)
}

// [BAD]: Goroutine given a func without ctx it runs in a nested closure
//
// The factory returns a func that captures nothing.
func badGoParamFuncFromFactoryNestedGoroutine(ctx context.Context) {
	go func(g func()) { // want `goroutine does not propagate context "ctx"`
		func() { g() }()
	}(plainFuncFactory())
}

// [BAD]: Goroutine given a func parameter a nested closure only compares
//
// The func is never run, as with the parameter compared directly.
func badGoParamFuncParamNestedNotRunGoroutine(ctx context.Context, f func()) {
	go func(g func()) { // want `goroutine does not propagate context "ctx"`
		func() { _ = g == nil }()
	}(f)
}

// [BAD]: Goroutine given a func parameter a nested closure replaces before it runs it
//
// The func run is the one the closure sets, which uses no context.
func badGoParamFuncParamNestedReplacedGoroutine(ctx context.Context, f func()) {
	go func(g func()) { // want `goroutine does not propagate context "ctx"`
		func() {
			g = func() {}
			g()
		}()
	}(f)
}

// [BAD]: Goroutine given an error parameter it uses in a nested closure
//
// An interface of the standard library does not count, as the error used
// directly does not.
func badGoParamErrorParamNestedGoroutine(ctx context.Context, err error) {
	go func(e error) { // want `goroutine does not propagate context "ctx"`
		func() { _ = e.Error() }()
	}(err)
}

// goParamFuncHolder holds a func a goroutine sets.
type goParamFuncHolder struct{ fn func() }

// [BAD]: Goroutine given a func parameter it only stores to a field
//
// The func is never run in the goroutine.
func badGoParamFuncParamStoredGoroutine(ctx context.Context, f func(), h *goParamFuncHolder) {
	go func(g func()) { // want `goroutine does not propagate context "ctx"`
		h.fn = g
	}(f)
}

// [GOOD]: Goroutine given a slice parameter of funcs it runs an element of
//
// The funcs cannot be followed, as the same slice captured passes (#325).
func goodGoParamFuncSliceParamGoroutine(ctx context.Context, fs []func()) {
	go func(x []func()) {
		x[0]()
	}(fs)
}

// [GOOD]: Goroutine given a map parameter of funcs it runs a value of
//
// The funcs cannot be followed (#325).
func goodGoParamFuncMapParamGoroutine(ctx context.Context, m map[string]func()) {
	go func(x map[string]func()) {
		x["a"]()
	}(m)
}

// [GOOD]: Goroutine given a channel parameter of funcs it runs what it receives
//
// The funcs cannot be followed (#325).
func goodGoParamFuncChanParamGoroutine(ctx context.Context, jobs <-chan func()) {
	go func(c <-chan func()) {
		for job := range c {
			job()
		}
	}(jobs)
}

// [GOOD]: Goroutine given a slice parameter of funcs it runs in a nested closure
//
// The nested closure reads the parameter as the goroutine itself would
// (#322, #325).
func goodGoParamFuncSliceParamNestedGoroutine(ctx context.Context, fs []func()) {
	go func(x []func()) {
		func() { x[0]() }()
	}(fs)
}

// [GOOD]: Goroutine given a slice literal of funcs, one of them a parameter
//
// The parameter cannot be followed, as the same element captured (#325).
func goodGoParamFuncSliceOfParamGoroutine(ctx context.Context, f func()) {
	go func(x []func()) {
		x[0]()
	}([]func(){f})
}

// [BAD]: Goroutine given a slice literal of funcs without ctx
//
// No func in the slice uses ctx.
func badGoParamFuncSlicePlainGoroutine(ctx context.Context) {
	go func(x []func()) { // want `goroutine does not propagate context "ctx"`
		x[0]()
	}([]func(){func() {}})
}

// [BAD]: Goroutine given a map literal of funcs without ctx
//
// No func in the map uses ctx.
func badGoParamFuncMapPlainGoroutine(ctx context.Context) {
	go func(x map[string]func()) { // want `goroutine does not propagate context "ctx"`
		x["a"]()
	}(map[string]func(){"a": func() {}})
}

// [BAD]: Goroutine given a slice parameter of funcs it does not run
//
// The funcs are only counted, as the same slice captured would be.
func badGoParamFuncSliceParamNotRunGoroutine(ctx context.Context, fs []func()) {
	go func(x []func()) { // want `goroutine does not propagate context "ctx"`
		_ = len(x)
	}(fs)
}

// [GOOD]: Goroutine given an array parameter of funcs it runs an element of
//
// The funcs cannot be followed (#325).
func goodGoParamFuncArrayParamGoroutine(ctx context.Context, fs [1]func()) {
	go func(x [1]func()) {
		x[0]()
	}(fs)
}

// [GOOD]: Goroutine given a slice parameter of contexts it reads an element of
//
// The contexts cannot be followed, as the same slice captured passes (#338).
func goodGoParamCtxSliceParamGoroutine(ctx context.Context, cs []context.Context) {
	go func(x []context.Context) {
		<-x[0].Done()
	}(cs)
}

// [GOOD]: Goroutine given an array parameter of contexts it reads an element of
//
// The contexts cannot be followed (#338).
func goodGoParamCtxArrayParamGoroutine(ctx context.Context, cs [1]context.Context) {
	go func(x [1]context.Context) {
		<-x[0].Done()
	}(cs)
}

// [GOOD]: Goroutine given a map parameter of contexts it reads a value of
//
// The contexts cannot be followed (#338).
func goodGoParamCtxMapParamGoroutine(ctx context.Context, m map[string]context.Context) {
	go func(x map[string]context.Context) {
		<-x["a"].Done()
	}(m)
}

// [GOOD]: Goroutine given a channel parameter of contexts it receives from
//
// The contexts cannot be followed (#338).
func goodGoParamCtxChanParamGoroutine(ctx context.Context, c <-chan context.Context) {
	go func(x <-chan context.Context) {
		<-(<-x).Done()
	}(c)
}

// [GOOD]: Goroutine given a pointer parameter to a slice of contexts
//
// The contexts cannot be followed (#338).
func goodGoParamCtxSlicePtrParamGoroutine(ctx context.Context, cs *[]context.Context) {
	go func(x *[]context.Context) {
		<-(*x)[0].Done()
	}(cs)
}

// [GOOD]: Goroutine given a slice parameter of slices of contexts
//
// The contexts cannot be followed (#338).
func goodGoParamCtxNestedSliceParamGoroutine(ctx context.Context, cs [][]context.Context) {
	go func(x [][]context.Context) {
		<-x[0][0].Done()
	}(cs)
}

// [GOOD]: Goroutine given a slice parameter of contexts it ranges over in a nested closure
//
// The nested closure reads the parameter as the goroutine itself would
// (#322, #338).
func goodGoParamCtxSliceParamNestedRangeGoroutine(ctx context.Context, cs []context.Context) {
	go func(x []context.Context) {
		func() {
			for _, c := range x {
				<-c.Done()
			}
		}()
	}(cs)
}

// [GOOD]: Goroutine given a slice literal of contexts, one of them a parameter
//
// The parameter cannot be followed, as the same element captured (#338).
func goodGoParamCtxSliceOfParamGoroutine(ctx context.Context, c context.Context) {
	go func(x []context.Context) {
		<-x[0].Done()
	}([]context.Context{context.Background(), c})
}

// [BAD]: Goroutine given a slice literal of only fresh contexts
//
// No context in the slice is ctx.
func badGoParamCtxSliceFreshGoroutine(ctx context.Context) {
	go func(x []context.Context) { // want `goroutine does not propagate context "ctx"`
		<-x[0].Done()
	}([]context.Context{context.Background()})
}

// [BAD]: Goroutine given a map literal of only fresh contexts
//
// No context in the map is ctx.
func badGoParamCtxMapFreshGoroutine(ctx context.Context) {
	go func(x map[string]context.Context) { // want `goroutine does not propagate context "ctx"`
		<-x["a"].Done()
	}(map[string]context.Context{"a": context.TODO()})
}

// [BAD]: Goroutine given a pointer to a local slice of only fresh contexts
//
// What is stored to the variable holds no ctx.
func badGoParamCtxSlicePtrFreshGoroutine(ctx context.Context) {
	cs := []context.Context{context.Background()}
	go func(x *[]context.Context) { // want `goroutine does not propagate context "ctx"`
		<-(*x)[0].Done()
	}(&cs)
}

// [BAD]: Goroutine given a slice parameter of contexts it does not read
//
// The contexts are only counted, as the same slice captured would be.
func badGoParamCtxSliceParamNotReadGoroutine(ctx context.Context, cs []context.Context) {
	go func(x []context.Context) { // want `goroutine does not propagate context "ctx"`
		_ = len(x)
	}(cs)
}

// [BAD]: Goroutine given a parameter of a slice type that holds itself
//
// No context is ever read from it.
func badGoParamSelfSliceParamGoroutine(ctx context.Context, l goParamSelfSlice) {
	go func(x goParamSelfSlice) { // want `goroutine does not propagate context "ctx"`
		_ = len(x[0])
	}(l)
}

type goParamSelfSlice []goParamSelfSlice

type goParamCtxHolder struct{ ctx context.Context }

type goParamCtxHolderPtr struct{ p *goParamCtxHolder }

type goParamCtxNotHolder struct{ n int }

// [GOOD]: Goroutine given a slice parameter of structs it reads a context field of
//
// The contexts cannot be followed, as the same slice captured passes (#341).
func goodGoParamCtxHolderSliceParamGoroutine(ctx context.Context, cs []goParamCtxHolder) {
	go func(x []goParamCtxHolder) {
		<-x[0].ctx.Done()
	}(cs)
}

// [GOOD]: Goroutine given a map parameter of structs it reads a context field of
//
// The contexts cannot be followed (#341).
func goodGoParamCtxHolderMapParamGoroutine(ctx context.Context, m map[string]goParamCtxHolder) {
	go func(x map[string]goParamCtxHolder) {
		<-x["a"].ctx.Done()
	}(m)
}

// [GOOD]: Goroutine given a slice parameter of pointers to structs it reads a context field of
//
// The contexts cannot be followed (#341).
func goodGoParamCtxHolderPtrSliceParamGoroutine(ctx context.Context, cs []*goParamCtxHolder) {
	go func(x []*goParamCtxHolder) {
		<-x[0].ctx.Done()
	}(cs)
}

// [GOOD]: Goroutine given a channel parameter of structs it reads a context field of
//
// The contexts cannot be followed (#341).
func goodGoParamCtxHolderChanParamGoroutine(ctx context.Context, c <-chan goParamCtxHolder) {
	go func(x <-chan goParamCtxHolder) {
		<-(<-x).ctx.Done()
	}(c)
}

// [GOOD]: Goroutine given a struct parameter it reads a context from through a pointer field
//
// The context cannot be followed, as the same struct captured passes (#341).
func goodGoParamCtxHolderPtrFieldParamGoroutine(ctx context.Context, s goParamCtxHolderPtr) {
	go func(x goParamCtxHolderPtr) {
		<-x.p.ctx.Done()
	}(s)
}

// [GOOD]: Goroutine given a slice parameter of structs it reads a context from through a pointer field
//
// The context cannot be followed (#341).
func goodGoParamCtxHolderPtrFieldSliceParamGoroutine(ctx context.Context, cs []goParamCtxHolderPtr) {
	go func(x []goParamCtxHolderPtr) {
		<-x[0].p.ctx.Done()
	}(cs)
}

// [BAD]: Goroutine given a slice parameter of structs without a context field
//
// No context is read from it.
func badGoParamCtxNotHolderSliceParamGoroutine(ctx context.Context, cs []goParamCtxNotHolder) {
	go func(x []goParamCtxNotHolder) { // want `goroutine does not propagate context "ctx"`
		_ = x[0].n
	}(cs)
}

// [BAD]: Goroutine given a slice parameter of structs whose context it does not read
//
// The structs are only counted, as the same slice captured would be.
func badGoParamCtxHolderSliceParamNotReadGoroutine(ctx context.Context, cs []goParamCtxHolder) {
	go func(x []goParamCtxHolder) { // want `goroutine does not propagate context "ctx"`
		_ = len(x)
	}(cs)
}

// [BAD]: Goroutine given a local map of structs nothing is put into
//
// The map holds no context.
func badGoParamCtxHolderEmptyMapGoroutine(ctx context.Context) {
	m := map[string]goParamCtxHolder{}
	go func(x map[string]goParamCtxHolder) { // want `goroutine does not propagate context "ctx"`
		<-x["a"].ctx.Done()
	}(m)
}

// [BAD]: Goroutine given a local channel of structs nothing is sent on
//
// The channel holds no context.
func badGoParamCtxHolderEmptyChanGoroutine(ctx context.Context) {
	c := make(chan goParamCtxHolder)
	go func(x chan goParamCtxHolder) { // want `goroutine does not propagate context "ctx"`
		<-(<-x).ctx.Done()
	}(c)
}

// [BAD]: Goroutine given a slice parameter of a struct type that holds itself
//
// No context is ever read from it.
func badGoParamSelfStructSliceParamGoroutine(ctx context.Context, cs []goParamSelfStruct) {
	go func(x []goParamSelfStruct) { // want `goroutine does not propagate context "ctx"`
		_ = x[0].next.n
	}(cs)
}

type goParamSelfStruct struct {
	next *goParamSelfStruct
	n    int
}

// [GOOD]: Goroutine in a closure given a captured slice of ctx
//
// The argument reads the variable the closure captures, which holds ctx
// (#346).
func goodGoParamCtxSliceCapturedGoroutine(ctx context.Context) {
	cs := []context.Context{ctx}
	func() {
		go func(x []context.Context) {
			<-x[0].Done()
		}(cs)
	}()
}

// [GOOD]: Goroutine in a closure given a captured slice parameter of contexts
//
// The contexts cannot be followed, as the same slice given directly (#346).
func goodGoParamCtxSliceParamCapturedGoroutine(ctx context.Context, cs []context.Context) {
	func() {
		go func(x []context.Context) {
			<-x[0].Done()
		}(cs)
	}()
}

// [GOOD]: Goroutine in a closure given the address of a captured slice of ctx
//
// What is stored to the variable holds ctx (#346).
func goodGoParamCtxSlicePtrCapturedGoroutine(ctx context.Context) {
	cs := []context.Context{ctx}
	func() {
		go func(x *[]context.Context) {
			<-(*x)[0].Done()
		}(&cs)
	}()
}

// [GOOD]: Goroutine in a deferred closure given a captured slice reassigned to hold ctx
//
// Every value stored to the variable counts, as the closure may run after
// any of them (#346).
func goodGoParamCtxSliceCapturedReassignedGoroutine(ctx context.Context) {
	cs := []context.Context{context.Background()}
	defer func() {
		go func(x []context.Context) {
			<-x[0].Done()
		}(cs)
	}()
	cs = []context.Context{ctx}
}

// [GOOD]: Goroutine in nested closures given a captured slice of ctx
//
// The variable is followed out through each closure (#346).
func goodGoParamCtxSliceCapturedNestedGoroutine(ctx context.Context) {
	cs := []context.Context{ctx}
	func() {
		func() {
			go func(x []context.Context) {
				<-x[0].Done()
			}(cs)
		}()
	}()
}

// [GOOD]: Goroutine in a closure given a captured slice of funcs that use ctx
//
// The funcs the variable holds use ctx (#346).
func goodGoParamFuncSliceCapturedGoroutine(ctx context.Context) {
	fs := []func(){func() { _ = ctx }}
	func() {
		go func(x []func()) {
			x[0]()
		}(fs)
	}()
}

// [GOOD]: Goroutine in a closure given a captured slice parameter of funcs
//
// The funcs cannot be followed, as the same slice given directly (#346).
func goodGoParamFuncSliceParamCapturedGoroutine(ctx context.Context, fs []func()) {
	func() {
		go func(x []func()) {
			x[0]()
		}(fs)
	}()
}

// [BAD]: Goroutine in a closure given a captured slice of only fresh contexts
//
// The variable the argument reads holds no ctx (#346).
func badGoParamCtxSliceCapturedFreshGoroutine(ctx context.Context) {
	cs := []context.Context{context.Background()}
	func() {
		go func(x []context.Context) { // want `goroutine does not propagate context "ctx"`
			<-x[0].Done()
		}(cs)
	}()
}

// [BAD]: Goroutine in a closure given the address of a captured slice of only fresh contexts
//
// What is stored to the variable holds no ctx (#346).
func badGoParamCtxSlicePtrCapturedFreshGoroutine(ctx context.Context) {
	cs := []context.Context{context.Background()}
	func() {
		go func(x *[]context.Context) { // want `goroutine does not propagate context "ctx"`
			<-(*x)[0].Done()
		}(&cs)
	}()
}

// [BAD]: Goroutine in a deferred closure given a captured slice of only fresh contexts
//
// The variable the argument reads holds no ctx (#346).
func badGoParamCtxSliceDeferCapturedFreshGoroutine(ctx context.Context) {
	cs := []context.Context{context.Background()}
	defer func() {
		go func(x []context.Context) { // want `goroutine does not propagate context "ctx"`
			<-x[0].Done()
		}(cs)
	}()
}

// [BAD]: Goroutine in nested closures given a captured slice of only fresh contexts
//
// The variable is followed out through each closure (#346).
func badGoParamCtxSliceCapturedNestedFreshGoroutine(ctx context.Context) {
	cs := []context.Context{context.TODO()}
	func() {
		func() {
			go func(x []context.Context) { // want `goroutine does not propagate context "ctx"`
				<-x[0].Done()
			}(cs)
		}()
	}()
}

// [BAD]: Goroutine in a closure given a captured slice of funcs without ctx
//
// No func the variable holds uses ctx (#346).
func badGoParamFuncSliceCapturedPlainGoroutine(ctx context.Context) {
	fs := []func(){func() {}}
	func() {
		go func(x []func()) { // want `goroutine does not propagate context "ctx"`
			x[0]()
		}(fs)
	}()
}

// ===== Go parameters given holders of only fresh contexts (#351, #343) =====

// setCtx sets the context h holds.
func (h *goParamCtxHolder) setCtx(ctx context.Context) { h.ctx = ctx }

// [BAD]: Goroutine given a pointer to a holder of only a fresh context
//
// The struct the pointer points to holds no ctx (#351).
func badGoParamCtxHolderPtrFreshGoroutine(ctx context.Context) {
	h := goParamCtxHolder{context.Background()}
	go func(x *goParamCtxHolder) { // want `goroutine does not propagate context "ctx"`
		<-x.ctx.Done()
	}(&h)
}

// [BAD]: Goroutine in a closure given a pointer to a captured holder of only a fresh context
//
// The struct the pointer points to holds no ctx (#351).
func badGoParamCtxHolderPtrFreshCapturedGoroutine(ctx context.Context) {
	h := goParamCtxHolder{context.Background()}
	func() {
		go func(x *goParamCtxHolder) { // want `goroutine does not propagate context "ctx"`
			<-x.ctx.Done()
		}(&h)
	}()
}

// [BAD]: Goroutine given a pointer literal of a holder of only a fresh context
//
// The struct holds no ctx (#351).
func badGoParamCtxHolderPtrLiteralFreshGoroutine(ctx context.Context) {
	go func(x *goParamCtxHolder) { // want `goroutine does not propagate context "ctx"`
		<-x.ctx.Done()
	}(&goParamCtxHolder{ctx: context.TODO()})
}

// [BAD]: Goroutine given a holder of only a fresh context
//
// The struct holds no ctx (#351).
func badGoParamCtxHolderFreshGoroutine(ctx context.Context) {
	go func(x goParamCtxHolder) { // want `goroutine does not propagate context "ctx"`
		<-x.ctx.Done()
	}(goParamCtxHolder{context.Background()})
}

// [BAD]: Goroutine given a slice literal of holders of only fresh contexts
//
// No struct in the slice holds ctx (#343).
func badGoParamCtxHolderSliceFreshGoroutine(ctx context.Context) {
	go func(x []goParamCtxHolder) { // want `goroutine does not propagate context "ctx"`
		<-x[0].ctx.Done()
	}([]goParamCtxHolder{{ctx: context.Background()}})
}

// [BAD]: Goroutine given a local slice of holders of only fresh contexts
//
// No struct in the slice holds ctx (#343).
func badGoParamCtxHolderSliceVarFreshGoroutine(ctx context.Context) {
	cs := []goParamCtxHolder{{ctx: context.Background()}, {ctx: context.TODO()}}
	go func(x []goParamCtxHolder) { // want `goroutine does not propagate context "ctx"`
		<-x[1].ctx.Done()
	}(cs)
}

// [GOOD]: Goroutine given a pointer to a holder of ctx
//
// The struct the pointer points to holds ctx.
func goodGoParamCtxHolderPtrGoroutine(ctx context.Context) {
	h := goParamCtxHolder{ctx}
	go func(x *goParamCtxHolder) {
		<-x.ctx.Done()
	}(&h)
}

// [GOOD]: Goroutine in a closure given a pointer to a captured holder of ctx
//
// The struct the pointer points to holds ctx.
func goodGoParamCtxHolderPtrCapturedGoroutine(ctx context.Context) {
	h := goParamCtxHolder{ctx}
	func() {
		go func(x *goParamCtxHolder) {
			<-x.ctx.Done()
		}(&h)
	}()
}

// [GOOD]: Goroutine given a pointer to a holder set to ctx after it starts
//
// Every value stored to the struct counts, as the goroutine may read it
// after any of them (#351).
func goodGoParamCtxHolderPtrSetAfterGoroutine(ctx context.Context) {
	h := goParamCtxHolder{context.Background()}
	go func(x *goParamCtxHolder) {
		<-x.ctx.Done()
	}(&h)
	h.ctx = ctx
}

// [GOOD]: Goroutine given a pointer to a holder a method sets to ctx
//
// The method writes the struct in a way not followed, so it may hold ctx
// (#351).
func goodGoParamCtxHolderPtrSetByMethodGoroutine(ctx context.Context) {
	h := &goParamCtxHolder{}
	h.setCtx(ctx)
	go func(x *goParamCtxHolder) {
		<-x.ctx.Done()
	}(h)
}

// [BAD]: Goroutine given a pointer to a holder of only a fresh context, also given to a method
//
// The method is given no context to write into the struct (#357).
func badGoParamCtxHolderPtrFreshGivenToMethodGoroutine(ctx context.Context) {
	h := goParamCtxHolder{context.Background()}
	h.resetCtx()
	go func(x *goParamCtxHolder) { // want `goroutine does not propagate context "ctx"`
		<-x.ctx.Done()
	}(&h)
}

// resetCtx sets the context h holds to a fresh one.
func (h *goParamCtxHolder) resetCtx() { h.ctx = context.Background() }

// [GOOD]: Goroutine given a pointer to a holder a closure sets to ctx
//
// The closure writes the struct in a way not followed, so it may hold ctx
// (#351).
func goodGoParamCtxHolderPtrSetInClosureGoroutine(ctx context.Context) {
	h := goParamCtxHolder{context.Background()}
	func() { h.ctx = ctx }()
	go func(x *goParamCtxHolder) {
		<-x.ctx.Done()
	}(&h)
}

// [GOOD]: Goroutine given a pointer to a holder set to ctx through another pointer
//
// The other pointer is a way of writing the struct not followed (#351).
func goodGoParamCtxHolderPtrSetThroughAliasGoroutine(ctx context.Context) {
	h := goParamCtxHolder{context.Background()}
	p := &h
	p.ctx = ctx
	go func(x *goParamCtxHolder) {
		<-x.ctx.Done()
	}(&h)
}

// [GOOD]: Goroutine given a pointer a constructor makes
//
// The result of the call cannot be followed (#351).
func goodGoParamCtxHolderPtrMadeGoroutine(ctx context.Context) {
	go func(x *goParamCtxHolder) {
		<-x.ctx.Done()
	}(newGoParamCtxHolder(ctx))
}

// newGoParamCtxHolder gives a holder of ctx.
func newGoParamCtxHolder(ctx context.Context) *goParamCtxHolder {
	return &goParamCtxHolder{ctx: ctx}
}

// [GOOD]: Goroutine given a holder of ctx
//
// The struct holds ctx.
func goodGoParamCtxHolderGoroutine(ctx context.Context) {
	go func(x goParamCtxHolder) {
		<-x.ctx.Done()
	}(goParamCtxHolder{ctx})
}

// [GOOD]: Goroutine given a holder of a struct a pointer field points to
//
// The struct pointed to cannot be followed, so it may hold ctx.
func goodGoParamCtxHolderPtrFieldLiteralGoroutine(ctx context.Context) {
	go func(x goParamCtxHolderPtr) {
		<-x.p.ctx.Done()
	}(goParamCtxHolderPtr{&goParamCtxHolder{ctx}})
}

// [GOOD]: Goroutine given a slice literal of holders of ctx
//
// A struct in the slice holds ctx (#343).
func goodGoParamCtxHolderSliceGoroutine(ctx context.Context) {
	go func(x []goParamCtxHolder) {
		<-x[0].ctx.Done()
	}([]goParamCtxHolder{{ctx: ctx}})
}

// [GOOD]: Goroutine given a local slice of holders, one set to ctx
//
// A struct in the slice holds ctx (#343).
func goodGoParamCtxHolderSliceSetGoroutine(ctx context.Context) {
	cs := []goParamCtxHolder{{ctx: context.Background()}}
	cs[0].ctx = ctx
	go func(x []goParamCtxHolder) {
		<-x[0].ctx.Done()
	}(cs)
}

// [GOOD]: Goroutine given a local slice of holders, one set to ctx at an index not constant
//
// The write at that index is not followed, so the slice may hold ctx
// (#343).
func goodGoParamCtxHolderSliceSetAtIndexGoroutine(ctx context.Context, i int) {
	cs := []goParamCtxHolder{{ctx: context.Background()}}
	cs[i].ctx = ctx
	go func(x []goParamCtxHolder) {
		<-x[0].ctx.Done()
	}(cs)
}

// [GOOD]: Goroutine given a local slice of holders also given to a call
//
// The call may write the slice (#343).
func goodGoParamCtxHolderSliceGivenToCallGoroutine(ctx context.Context) {
	cs := []goParamCtxHolder{{ctx: context.Background()}}
	fillGoParamCtxHolders(cs, ctx)
	go func(x []goParamCtxHolder) {
		<-x[0].ctx.Done()
	}(cs)
}

// fillGoParamCtxHolders sets each holder of cs to ctx.
func fillGoParamCtxHolders(cs []goParamCtxHolder, ctx context.Context) {
	for i := range cs {
		cs[i].ctx = ctx
	}
}

// [BAD]: Goroutine given a pointer to a holder of a nil context
//
// A nil context holds no ctx (#351).
func badGoParamCtxHolderPtrNilGoroutine(ctx context.Context) {
	h := goParamCtxHolder{ctx: nil}
	go func(x *goParamCtxHolder) { // want `goroutine does not propagate context "ctx"`
		<-x.ctx.Done()
	}(&h)
}

// [GOOD]: Goroutine given a slice of holders also kept in a variable whose address a call is given
//
// The call may set the holders through the variable (#351).
func goodGoParamCtxHolderSliceKeptSetByCallGoroutine(ctx context.Context) {
	cs := []goParamCtxHolder{{ctx: context.Background()}}
	kept := cs
	setGoParamCtxHolders(&kept, ctx)
	go func(x []goParamCtxHolder) {
		<-x[0].ctx.Done()
	}(cs)
}

// setGoParamCtxHolders sets the first holder of cs to ctx.
func setGoParamCtxHolders(cs *[]goParamCtxHolder, ctx context.Context) {
	(*cs)[0].ctx = ctx
}

// freshGoParamCtxs gives a slice of only a fresh context.
func freshGoParamCtxs() []context.Context {
	return []context.Context{context.Background()}
}

// goParamCtxsOf gives a slice of ctx.
func goParamCtxsOf(ctx context.Context) []context.Context {
	return []context.Context{ctx}
}

// [BAD]: Goroutine given a slice of only fresh contexts a function makes
//
// The slices the function returns hold no ctx.
func badGoParamCtxSliceMadeFreshGoroutine(ctx context.Context) {
	go func(x []context.Context) { // want `goroutine does not propagate context "ctx"`
		<-x[0].Done()
	}(freshGoParamCtxs())
}

// [GOOD]: Goroutine given a slice of ctx a function makes
//
// The slices the function returns hold ctx.
func goodGoParamCtxSliceMadeGoroutine(ctx context.Context) {
	go func(x []context.Context) {
		<-x[0].Done()
	}(goParamCtxsOf(ctx))
}
