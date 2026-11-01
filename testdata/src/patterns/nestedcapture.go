// A value the spawned closure captures and only a closure inside it uses,
// as a deferred recover handler calling cancel (#268).

package patterns

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"golang.org/x/sync/errgroup"
)

// nestedRunner is run by a method call on the interface.
type nestedRunner interface{ Run() }

// nestedCtxRunner holds a context and waits for it.
type nestedCtxRunner struct{ ctx context.Context }

func (r nestedCtxRunner) Run() { <-r.ctx.Done() }

// nestedRecover runs f and recovers a panic in it.
func nestedRecover(f func()) {
	defer func() { _ = recover() }()
	f()
}

// ===== cancel called in a deferred closure =====

// [GOOD]: cancel in a deferred recover closure
//
// The deferred closure inside the goroutine calls cancel of a context made
// from ctx.
func goodNestedCancelInDeferGoroutine(ctx context.Context) {
	_, cancel := context.WithCancel(ctx)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				cancel()
			}
		}()
		fmt.Println("work")
	}()
}

// [GOOD]: cancel in a deferred recover closure
//
// The deferred closure inside the goroutine calls cancel of a context made
// from ctx.
func goodNestedCancelInDeferErrgroup(ctx context.Context) {
	_, cancel := context.WithCancel(ctx)
	g := new(errgroup.Group)
	g.Go(func() error {
		defer func() {
			if r := recover(); r != nil {
				cancel()
			}
		}()
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: cancel in a deferred recover closure
//
// The deferred closure inside the goroutine calls cancel of a context made
// from ctx.
func goodNestedCancelInDeferWaitGroup(ctx context.Context) {
	_, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				cancel()
			}
		}()
	})
	wg.Wait()
}

// [GOOD]: cancel in a deferred recover closure
//
// The deferred closure inside the goroutine calls cancel of a context made
// from ctx.
func goodNestedCancelInDeferSpawner(ctx context.Context) {
	_, cancel := context.WithCancel(ctx)
	g := new(errgroup.Group)
	runWithGroup(g, func() error {
		defer func() {
			if r := recover(); r != nil {
				cancel()
			}
		}()
		return nil
	})
	_ = g.Wait()
}

// [BAD]: cancel of a fresh context in a deferred recover closure
//
// The cancel is of a context made from context.Background(), not ctx.
func badNestedFreshCancelInDeferGoroutine(ctx context.Context) {
	_, cancel := context.WithCancel(context.Background())
	go func() { // want `goroutine does not propagate context "ctx"`
		defer func() {
			if r := recover(); r != nil {
				cancel()
			}
		}()
		fmt.Println("work")
	}()
}

// [BAD]: cancel of a fresh context in a deferred recover closure
//
// The cancel is of a context made from context.Background(), not ctx.
func badNestedFreshCancelInDeferErrgroup(ctx context.Context) {
	_, cancel := context.WithCancel(context.Background())
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		defer func() {
			if r := recover(); r != nil {
				cancel()
			}
		}()
		return nil
	})
	_ = g.Wait()
}

// [BAD]: cancel of a fresh context in a deferred recover closure
//
// The cancel is of a context made from context.Background(), not ctx.
func badNestedFreshCancelInDeferWaitGroup(ctx context.Context) {
	_, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		defer func() {
			if r := recover(); r != nil {
				cancel()
			}
		}()
	})
	wg.Wait()
}

// [BAD]: cancel of a fresh context in a deferred recover closure
//
// The cancel is of a context made from context.Background(), not ctx.
func badNestedFreshCancelInDeferSpawner(ctx context.Context) {
	_, cancel := context.WithCancel(context.Background())
	g := new(errgroup.Group)
	runWithGroup(g, func() error { // want `runWithGroup\(\) func argument should use context "ctx"`
		defer func() {
			if r := recover(); r != nil {
				cancel()
			}
		}()
		return nil
	})
	_ = g.Wait()
}

// ===== A func run by a helper =====

// [GOOD]: Parameter func run inside a recover helper
//
// fn cannot be followed, so it passes, as go func() { fn() }() does.
func goodNestedParamInHelperGoroutine(ctx context.Context, fn func()) {
	go func() {
		nestedRecover(func() { fn() })
	}()
}

// [GOOD]: cancel run inside a recover helper
//
// The inner closure calls cancel of a context made from ctx.
func goodNestedCancelInHelperErrgroup(ctx context.Context) {
	_, cancel := context.WithCancel(ctx)
	g := new(errgroup.Group)
	g.Go(func() error {
		nestedRecover(func() { cancel() })
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Func without ctx run inside a recover helper
//
// The inner closure runs a func that does not use ctx.
func badNestedFuncInHelperErrgroup(ctx context.Context) {
	fn := func() { fmt.Println("work") }
	g := new(errgroup.Group)
	g.Go(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		nestedRecover(func() { fn() })
		return nil
	})
	_ = g.Wait()
}

// ===== Values read in an inner closure =====

// [GOOD]: Interface method called in an inner closure
//
// The interface holds a value with ctx, and the inner closure calls its
// method, as go func() { r.Run() }() does.
func goodNestedInterfaceMethodGoroutine(ctx context.Context) {
	var r nestedRunner = nestedCtxRunner{ctx: ctx}
	go func() {
		func() { r.Run() }()
	}()
}

// [GOOD]: Command made with ctx run in an inner closure
//
// exec.CommandContext puts ctx in the *exec.Cmd, and the inner closure runs
// it.
func goodNestedCommandContextWaitGroup(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "true")
	var wg sync.WaitGroup
	wg.Go(func() {
		run := func() error { return cmd.Run() }
		_ = run()
	})
	wg.Wait()
}

// [BAD]: Command made without ctx run in an inner closure
//
// exec.Command takes no context.
func badNestedCommandWaitGroup(ctx context.Context) {
	cmd := exec.Command("true")
	var wg sync.WaitGroup
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		run := func() error { return cmd.Run() }
		_ = run()
	})
	wg.Wait()
}

// [GOOD]: Type assertion to a context in an inner closure
//
// The inner closure asserts the captured value to a context and waits for
// it.
func goodNestedAssertGoroutine(ctx context.Context) {
	var a any = ctx
	go func() {
		func() { <-a.(context.Context).Done() }()
	}()
}

// [BAD]: Captured struct without reading its context, in an inner closure
//
// The inner closure reads only a field that is not the context.
func badNestedOtherFieldGoroutine(ctx context.Context) {
	h := struct {
		ctx  context.Context
		name string
	}{ctx, "x"}
	go func() { // want `goroutine does not propagate context "ctx"`
		func() { fmt.Println(h.name) }()
	}()
}
