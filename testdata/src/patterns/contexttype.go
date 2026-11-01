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
