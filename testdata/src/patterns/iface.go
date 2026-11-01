// A captured value of an interface type, judged by the values it can be
// (#240), and funcs put into a channel or a map given to a factory.

package patterns

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ===== Method of a captured interface whose value holds a context =====

// [GOOD]: Method of a captured interface whose value holds a context
//
// The value may read the context, as for the concrete value and for go r.Run()
// (#240).
func goodCapturedIfaceCtxMethodGoroutine(ctx context.Context) {
	var r taskRunner = ctxTaskRunner{ctx: ctx}
	go func() { r.Run() }()
}

// [GOOD]: Method of a captured interface whose value holds a context
//
// The value may read the context, as for the concrete value and for go r.Run()
// (#240).
func goodCapturedIfaceCtxMethodErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	var r taskRunner = ctxTaskRunner{ctx: ctx}
	g.Go(func() error { r.Run(); return nil })
	_ = g.Wait()
}

// [GOOD]: Method of a captured interface whose value holds a context
//
// The value may read the context, as for the concrete value and for go r.Run()
// (#240).
func goodCapturedIfaceCtxMethodWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	var r taskRunner = ctxTaskRunner{ctx: ctx}
	wg.Go(func() { r.Run() })
	wg.Wait()
}

// [GOOD]: Method of a captured interface whose value holds a context
//
// The value may read the context, as for the concrete value and for go r.Run()
// (#240).
func goodCapturedIfaceCtxMethodSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	var r taskRunner = ctxTaskRunner{ctx: ctx}
	runWithGroup(g, func() error { r.Run(); return nil })
	_ = g.Wait()
}

// [GOOD]: Method of a captured interface whose value is a pointer that holds a context
//
// The pointer is followed to the struct it points to (#240).
func goodCapturedIfacePtrCtxMethodGoroutine(ctx context.Context) {
	var r taskRunner = &ctxTaskRunner{ctx: ctx}
	go func() { r.Run() }()
}

// [GOOD]: Method of a captured interface whose value is a pointer that holds a context
//
// The pointer is followed to the struct it points to (#240).
func goodCapturedIfacePtrCtxMethodErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	var r taskRunner = &ctxTaskRunner{ctx: ctx}
	g.Go(func() error { r.Run(); return nil })
	_ = g.Wait()
}

// [GOOD]: Method of a captured interface whose value is a pointer that holds a context
//
// The pointer is followed to the struct it points to (#240).
func goodCapturedIfacePtrCtxMethodWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	var r taskRunner = &ctxTaskRunner{ctx: ctx}
	wg.Go(func() { r.Run() })
	wg.Wait()
}

// [GOOD]: Method of a captured interface whose value is a pointer that holds a context
//
// The pointer is followed to the struct it points to (#240).
func goodCapturedIfacePtrCtxMethodSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	var r taskRunner = &ctxTaskRunner{ctx: ctx}
	runWithGroup(g, func() error { r.Run(); return nil })
	_ = g.Wait()
}

// [GOOD]: Method of a captured interface that holds a context on one branch
//
// One of the values the interface can be holds a context (#240).
func goodCapturedIfaceOneBranchCtxMethodGoroutine(ctx context.Context) {
	var r taskRunner = plainTaskRunner{}
	if conditionFlag {
		r = ctxTaskRunner{ctx: ctx}
	}
	go func() { r.Run() }()
}

// ===== Captured interface whose value holds a context, given to a function =====

// [GOOD]: Captured interface whose value holds a context, given to a function
//
// The function may read the context, as for the concrete value (#240).
func goodCapturedIfaceCtxArgGoroutine(ctx context.Context) {
	var r taskRunner = ctxTaskRunner{ctx: ctx}
	go func() { runAny(r) }()
}

// [GOOD]: Captured interface whose value holds a context, given to a function
//
// The function may read the context, as for the concrete value (#240).
func goodCapturedIfaceCtxArgErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	var r taskRunner = ctxTaskRunner{ctx: ctx}
	g.Go(func() error { runAny(r); return nil })
	_ = g.Wait()
}

// [GOOD]: Captured interface whose value holds a context, given to a function
//
// The function may read the context, as for the concrete value (#240).
func goodCapturedIfaceCtxArgWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	var r taskRunner = ctxTaskRunner{ctx: ctx}
	wg.Go(func() { runAny(r) })
	wg.Wait()
}

// [GOOD]: Captured interface whose value holds a context, given to a function
//
// The function may read the context, as for the concrete value (#240).
func goodCapturedIfaceCtxArgSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	var r taskRunner = ctxTaskRunner{ctx: ctx}
	runWithGroup(g, func() error { runAny(r); return nil })
	_ = g.Wait()
}

// [GOOD]: Captured interface whose value holds a context, given to a function of its own type
//
// The function may read the context, as for the concrete value (#240).
func goodCapturedIfaceCtxOwnTypeArgGoroutine(ctx context.Context) {
	var r taskRunner = &ctxTaskRunner{ctx: ctx}
	go func() { runTaskRunner(r) }()
}

// ===== Method of a captured interface whose value holds no context =====

// [BAD]: Method of a captured interface whose value holds no context
//
// Every value the interface can be is followed, and none holds a context.
func badCapturedIfacePlainMethodGoroutine(ctx context.Context) {
	var r taskRunner = plainTaskRunner{}
	go func() { r.Run() }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Method of a captured interface whose value holds no context
//
// Every value the interface can be is followed, and none holds a context.
func badCapturedIfacePlainMethodErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	var r taskRunner = plainTaskRunner{}
	g.Go(func() error { r.Run(); return nil }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Method of a captured interface whose value holds no context
//
// Every value the interface can be is followed, and none holds a context.
func badCapturedIfacePlainMethodWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	var r taskRunner = plainTaskRunner{}
	wg.Go(func() { r.Run() }) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Method of a captured interface whose value holds no context
//
// Every value the interface can be is followed, and none holds a context.
func badCapturedIfacePlainMethodSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	var r taskRunner = plainTaskRunner{}
	runWithGroup(g, func() error { r.Run(); return nil }) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Captured interface whose value holds no context, given to a function
//
// Every value the interface can be is followed, and none holds a context.
func badCapturedIfacePlainArgGoroutine(ctx context.Context) {
	var r taskRunner = &plainTaskRunner{}
	go func() { runAny(r) }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Method of a captured interface that is nil or holds no context
//
// A nil interface holds nothing.
func badCapturedIfaceNilOrPlainMethodGoroutine(ctx context.Context) {
	r := taskRunner(nil)
	if conditionFlag {
		r = plainTaskRunner{}
	}
	go func() { // want `goroutine does not propagate context "ctx"`
		if r != nil {
			r.Run()
		}
	}()
}

// ===== Captured interface changed from another interface, given to a function =====

// [GOOD]: Captured interface changed from an interface whose value holds a context
//
// The interface it is changed from is judged the same way (#240).
func goodCapturedIfaceChangedCtxArgGoroutine(ctx context.Context) {
	var r taskRunner = ctxTaskRunner{ctx: ctx}
	var a any = r
	go func() { runAny(a) }()
}

// [GOOD]: Captured interface changed from ctx
//
// The context it is changed from is not fresh (#240).
func goodCapturedIfaceChangedFromCtxArgGoroutine(ctx context.Context) {
	var a any = ctx
	go func() { runAny(a) }()
}

// [BAD]: Captured interface changed from an interface whose value holds no context
//
// The interface it is changed from holds no context.
func badCapturedIfaceChangedPlainArgGoroutine(ctx context.Context) {
	var r taskRunner = plainTaskRunner{}
	var a any = r
	go func() { runAny(a) }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Captured interface changed from a fresh context
//
// context.Background() carries nothing of ctx.
func badCapturedIfaceChangedFromFreshCtxArgGoroutine(ctx context.Context) {
	var a any = context.Background()
	go func() { runAny(a) }() // want `goroutine does not propagate context "ctx"`
}

// ===== Method of a captured interface that cannot be followed =====

// [NOTCHECKED]: Method of a captured interface from a parameter
//
// What a parameter holds cannot be followed, and its interface is declared
// in the analyzed package, so it may hold a context, as go r.Run() is left
// alone (#240).
func notCheckedCapturedIfaceParamMethodGoroutine(ctx context.Context, r taskRunner) {
	go func() { r.Run() }()
}

// [NOTCHECKED]: Method of a captured interface from a parameter
//
// What a parameter holds cannot be followed, and its interface is declared
// in the analyzed package, so it may hold a context, as go r.Run() is left
// alone (#240).
func notCheckedCapturedIfaceParamMethodErrgroup(ctx context.Context, r taskRunner) {
	g := new(errgroup.Group)
	g.Go(func() error { r.Run(); return nil })
	_ = g.Wait()
}

// [NOTCHECKED]: Method of a captured interface from a parameter
//
// What a parameter holds cannot be followed, and its interface is declared
// in the analyzed package, so it may hold a context, as go r.Run() is left
// alone (#240).
func notCheckedCapturedIfaceParamMethodWaitGroup(ctx context.Context, r taskRunner) {
	var wg sync.WaitGroup
	wg.Go(func() { r.Run() })
	wg.Wait()
}

// [NOTCHECKED]: Method of a captured interface from a parameter
//
// What a parameter holds cannot be followed, and its interface is declared
// in the analyzed package, so it may hold a context, as go r.Run() is left
// alone (#240).
func notCheckedCapturedIfaceParamMethodSpawner(ctx context.Context, r taskRunner) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error { r.Run(); return nil })
	_ = g.Wait()
}

// [NOTCHECKED]: Captured interface from a call, given to a function
//
// What a call returns cannot be followed, so it may hold a context (#240).
func notCheckedCapturedIfaceCallArgGoroutine(ctx context.Context) {
	r := newPlainTaskRunner()
	go func() { runAny(r) }()
}

// [NOTCHECKED]: Method of a captured interface whose value holds only a fresh context
//
// The method is not looked into, so it passes, as for the concrete value.
func notCheckedCapturedIfaceFreshCtxMethodGoroutine(ctx context.Context) {
	var r taskRunner = ctxTaskRunner{ctx: context.Background()}
	go func() { r.Run() }()
}

// ===== Method of a captured interface of the standard library that cannot be followed =====

// [BAD]: Method of a captured net.Conn from a parameter
//
// The analyzed code declares no net.Conn, so a value of it that cannot be
// followed is taken to hold no context (#240).
func badCapturedStdIfaceParamMethodGoroutine(ctx context.Context, conn net.Conn) {
	go func() { _ = conn.Close() }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Method of a captured net.Conn from a parameter
//
// The analyzed code declares no net.Conn, so a value of it that cannot be
// followed is taken to hold no context (#240).
func badCapturedStdIfaceParamMethodErrgroup(ctx context.Context, conn net.Conn) {
	g := new(errgroup.Group)
	g.Go(func() error { return conn.Close() }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Method of a captured net.Conn from a parameter
//
// The analyzed code declares no net.Conn, so a value of it that cannot be
// followed is taken to hold no context (#240).
func badCapturedStdIfaceParamMethodWaitGroup(ctx context.Context, conn net.Conn) {
	var wg sync.WaitGroup
	wg.Go(func() { _ = conn.Close() }) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Method of a captured net.Conn from a parameter
//
// The analyzed code declares no net.Conn, so a value of it that cannot be
// followed is taken to hold no context (#240).
func badCapturedStdIfaceParamMethodSpawner(ctx context.Context, conn net.Conn) {
	g := new(errgroup.Group)
	runWithGroup(g, func() error { return conn.Close() }) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Captured io.Reader field of a parameter, given to a function
//
// The analyzed code declares no io.Reader (#240).
func badCapturedStdIfaceFieldArgGoroutine(ctx context.Context, req *http.Request) {
	go func() { _, _ = io.Copy(io.Discard, req.Body) }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Captured error from a call, given to errors.Is
//
// error is declared by no package, so a value of it that cannot be followed
// is taken to hold no context (#240).
func badCapturedErrorFromCallArgGoroutine(ctx context.Context) {
	err := failingCall()
	go func() { _ = errors.Is(err, errStop) }() // want `goroutine does not propagate context "ctx"`
}

// ===== Interface whose value holds a context, given to a factory =====

// [GOOD]: Interface whose value holds a context, given to a factory
//
// The struct literal converted to the interface holds ctx (#240).
func goodIfaceCtxToFactoryGoroutine(ctx context.Context) {
	go wrapTaskRunner(ctxTaskRunner{ctx: ctx})()
}

// [GOOD]: Interface whose value holds a context, given to a factory
//
// The struct literal converted to the interface holds ctx (#240).
func goodIfaceCtxToFactoryErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(wrapTaskRunnerErr(ctxTaskRunner{ctx: ctx}))
	_ = g.Wait()
}

// [GOOD]: Interface whose value holds a context, given to a factory
//
// The struct literal converted to the interface holds ctx (#240).
func goodIfaceCtxToFactoryWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(wrapTaskRunner(ctxTaskRunner{ctx: ctx}))
	wg.Wait()
}

// [GOOD]: Interface whose value holds a context, given to a factory
//
// The struct literal converted to the interface holds ctx (#240).
func goodIfaceCtxToFactorySpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, wrapTaskRunnerErr(ctxTaskRunner{ctx: ctx}))
	_ = g.Wait()
}

// [GOOD]: Interface whose value is a pointer that holds a context, given to a factory
//
// The struct literal the pointer points to holds ctx (#240).
func goodIfacePtrCtxToFactoryGoroutine(ctx context.Context) {
	go wrapTaskRunner(&ctxTaskRunner{ctx: ctx})()
}

// [GOOD]: Interface variable whose value holds a context, given to a factory
//
// The struct literal converted to the interface holds ctx (#240).
func goodIfaceVarCtxToFactoryGoroutine(ctx context.Context) {
	var r taskRunner = ctxTaskRunner{ctx: ctx}
	go wrapTaskRunner(r)()
}

// [NOTCHECKED]: Interface whose value holds no context, given to a factory
//
// The closure the factory returns calls a method of its parameter, which
// cannot be followed, so it passes (#240).
func notCheckedIfacePlainToFactoryGoroutine(ctx context.Context) {
	go wrapTaskRunner(plainTaskRunner{})()
}

// ===== Channel of funcs given to a factory =====

// [GOOD]: Channel of funcs that use ctx, given to a factory
//
// A func sent on the channel captures ctx (#240).
func goodFuncChanCtxToFactoryGoroutine(ctx context.Context) {
	ch := make(chan func(), 1)
	ch <- func() { use(ctx) }
	go runFromChan(ch)()
}

// [GOOD]: Channel of funcs that use ctx, given to a factory
//
// A func sent on the channel captures ctx (#240).
func goodFuncChanCtxToFactoryErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan func(), 1)
	ch <- func() { use(ctx) }
	g.Go(runFromChanErr(ch))
	_ = g.Wait()
}

// [GOOD]: Channel of funcs that use ctx, given to a factory
//
// A func sent on the channel captures ctx (#240).
func goodFuncChanCtxToFactoryWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	ch := make(chan func(), 1)
	ch <- func() { use(ctx) }
	wg.Go(runFromChan(ch))
	wg.Wait()
}

// [GOOD]: Channel of funcs that use ctx, given to a factory
//
// A func sent on the channel captures ctx (#240).
func goodFuncChanCtxToFactorySpawner(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan func(), 1)
	ch <- func() { use(ctx) }
	runWithGroup(g, runFromChanErr(ch))
	_ = g.Wait()
}

// [BAD]: Channel of funcs that do not use ctx, given to a factory
//
// No func sent on the channel captures ctx.
func badFuncChanPlainToFactoryGoroutine(ctx context.Context) {
	ch := make(chan func(), 1)
	ch <- func() {}
	go runFromChan(ch)() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Channel of funcs that do not use ctx, given to a factory
//
// No func sent on the channel captures ctx.
func badFuncChanPlainToFactoryErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan func(), 1)
	ch <- func() {}
	g.Go(runFromChanErr(ch)) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Channel of funcs that do not use ctx, given to a factory
//
// No func sent on the channel captures ctx.
func badFuncChanPlainToFactoryWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	ch := make(chan func(), 1)
	ch <- func() {}
	wg.Go(runFromChan(ch)) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Channel of funcs that do not use ctx, given to a factory
//
// No func sent on the channel captures ctx.
func badFuncChanPlainToFactorySpawner(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan func(), 1)
	ch <- func() {}
	runWithGroup(g, runFromChanErr(ch)) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== Map of funcs given to a factory =====

// [GOOD]: Map of funcs that use ctx, given to a factory
//
// A func set in the map literal captures ctx (#240).
func goodFuncMapCtxToFactoryGoroutine(ctx context.Context) {
	go runFromMap(map[string]func(){"a": func() { use(ctx) }})()
}

// [GOOD]: Map of funcs that use ctx, given to a factory
//
// A func set in the map literal captures ctx (#240).
func goodFuncMapCtxToFactoryErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(runFromMapErr(map[string]func(){"a": func() { use(ctx) }}))
	_ = g.Wait()
}

// [GOOD]: Map of funcs that use ctx, given to a factory
//
// A func set in the map literal captures ctx (#240).
func goodFuncMapCtxToFactoryWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(runFromMap(map[string]func(){"a": func() { use(ctx) }}))
	wg.Wait()
}

// [GOOD]: Map of funcs that use ctx, given to a factory
//
// A func set in the map literal captures ctx (#240).
func goodFuncMapCtxToFactorySpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, runFromMapErr(map[string]func(){"a": func() { use(ctx) }}))
	_ = g.Wait()
}

// [BAD]: Map of funcs that do not use ctx, given to a factory
//
// No func set in the map literal captures ctx.
func badFuncMapPlainToFactoryGoroutine(ctx context.Context) {
	go runFromMap(map[string]func(){"a": func() {}})() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Map of funcs that do not use ctx, given to a factory
//
// No func set in the map literal captures ctx.
func badFuncMapPlainToFactoryErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	g.Go(runFromMapErr(map[string]func(){"a": func() {}})) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Map of funcs that do not use ctx, given to a factory
//
// No func set in the map literal captures ctx.
func badFuncMapPlainToFactoryWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(runFromMap(map[string]func(){"a": func() {}})) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Map of funcs that do not use ctx, given to a factory
//
// No func set in the map literal captures ctx.
func badFuncMapPlainToFactorySpawner(ctx context.Context) {
	g := new(errgroup.Group)
	runWithGroup(g, runFromMapErr(map[string]func(){"a": func() {}})) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== Slice of funcs given to a factory =====

// [GOOD]: Slice of funcs that use ctx, given to a factory
//
// A func in the slice literal captures ctx (#240).
func goodFuncSliceCtxToFactoryGoroutine(ctx context.Context) {
	go runFromSlice([]func(){func() { use(ctx) }})()
}

// [NOTCHECKED]: Slice of funcs that do not use ctx, given to a factory
//
// The closure the factory returns runs an element of its parameter, which
// is not followed to the slice it is given, so it passes.
func notCheckedFuncSlicePlainToFactoryGoroutine(ctx context.Context) {
	go runFromSlice([]func(){func() {}})()
}

// taskRunner runs a task.
type taskRunner interface{ Run() }

// ctxTaskRunner runs a task with its context.
type ctxTaskRunner struct{ ctx context.Context }

func (r ctxTaskRunner) Run() { use(r.ctx) }

// plainTaskRunner runs a task with no context.
type plainTaskRunner struct{}

func (plainTaskRunner) Run() {}

func newPlainTaskRunner() taskRunner { return plainTaskRunner{} }

func runTaskRunner(r taskRunner) { r.Run() }

func wrapTaskRunner(r taskRunner) func() { return func() { r.Run() } }

func wrapTaskRunnerErr(r taskRunner) func() error { return func() error { r.Run(); return nil } }

func runFromChan(ch chan func()) func() { return func() { f := <-ch; f() } }

func runFromChanErr(ch chan func()) func() error { return func() error { f := <-ch; f(); return nil } }

func runFromMap(m map[string]func()) func() { return func() { m["a"]() } }

func runFromMapErr(m map[string]func()) func() error { return func() error { m["a"](); return nil } }

func runFromSlice(fs []func()) func() { return func() { fs[0]() } }

var errStop = errors.New("stop")

func failingCall() error { return errStop }
