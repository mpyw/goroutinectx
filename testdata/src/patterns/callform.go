// Spawning APIs called as a method expression, or through a method or
// function value, rather than as a plain call.

package patterns

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ===== Method expression =====

// [BAD]: Method expression
//
// The receiver comes first, and the callback after it is checked (#120).
func badMethodExprErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	(*errgroup.Group).Go(g, func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Method expression
//
// The receiver comes first, and the callback after it is checked (#120).
func badMethodExprWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	(*sync.WaitGroup).Go(&wg, func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
	})
	wg.Wait()
}

// [GOOD]: Method expression with ctx
//
// The callback uses ctx.
func goodMethodExprErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	(*errgroup.Group).Go(g, func() error {
		_ = ctx
		return nil
	})
	_ = g.Wait()
}

// ===== Method expression in a variable =====

// [BAD]: Method expression in a variable
//
// The receiver comes first, and the callback after it is checked (#185).
func badMethodExprVarErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	goFn := (*errgroup.Group).Go
	goFn(g, func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Method expression in a variable
//
// The receiver comes first, and the callback after it is checked (#185).
func badMethodExprVarWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	goFn := (*sync.WaitGroup).Go
	goFn(&wg, func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
	})
	wg.Wait()
}

// [GOOD]: Method expression in a variable with ctx
//
// The callback uses ctx.
func goodMethodExprVarErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	goFn := (*errgroup.Group).Go
	goFn(g, func() error {
		_ = ctx
		return nil
	})
	_ = g.Wait()
}

// ===== Method or function value =====

// [BAD]: Method or function value
//
// A call through a method value is found through SSA (#120).
func badMethodValueErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	goFn := g.Go
	goFn(func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
		return nil
	})
	_ = g.Wait()
}

// [BAD]: Method or function value
//
// A call through a method value is found through SSA (#120).
func badMethodValueWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	goFn := wg.Go
	goFn(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
	})
	wg.Wait()
}

// [BAD]: Method or function value
//
// A call through a function value is found through SSA (#120).
func badFuncValueSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	run := runWithGroup
	run(g, func() error { // want `runWithGroup\(\) func argument should use context "ctx"`
		fmt.Println("no context")
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Method or function value with ctx
//
// The callback uses ctx.
func goodMethodValueWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	goFn := wg.Go
	goFn(func() {
		_ = ctx
	})
	wg.Wait()
}

// [NOTCHECKED]: Method value that may be another function
//
// A variable that holds one of two functions is not followed, so it passes.
func notCheckedMethodValueEitherErrgroup(ctx context.Context, b bool) {
	g := new(errgroup.Group)
	goFn := g.Go
	if b {
		goFn = func(f func() error) { _ = f() }
	}
	goFn(func() error {
		return nil
	})
	_ = g.Wait()
}
