// A func read from a struct field, slice, map, interface, channel or spawner.

package patterns

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ===== Struct field =====

// [GOOD]: Struct field selector captures ctx
//
// Function in struct field captures context.
func goodStructFieldCapturesCtx(ctx context.Context) {
	s := struct{ handler func() }{
		handler: func() { _ = ctx },
	}
	go s.handler()
}

// [GOOD]: Struct field with ctx
//
// Function stored in struct field captures context.
func goodStructFieldWithCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	holder := taskHolderWithCtxErrgroup{
		task: func() error {
			_ = ctx // Uses ctx
			return nil
		},
	}
	g.Go(holder.task) // OK - now tracked
	_ = g.Wait()
}

// [GOOD]: Struct field with ctx
//
// Function stored in struct field captures context.
func goodStructFieldWithCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	holder := taskHolderWithCtxWaitGroup{
		task: func() {
			_ = ctx // Uses ctx
		},
	}
	wg.Go(holder.task) // OK - now tracked
	wg.Wait()
}

// [GOOD]: Struct field func argument with context
//
// A func in a struct field that uses ctx.
func goodStructFieldArg(ctx context.Context) {
	g := new(errgroup.Group)
	job := spawnerJob{Run: func() error { _ = ctx; return nil }}
	runWithGroup(g, job.Run)
	_ = g.Wait()
}

// [BAD]: Struct field selector captures ctx
//
// Function in struct field does not capture context.
func badStructFieldMissingCtx(ctx context.Context) {
	s := struct{ handler func() }{
		handler: func() { fmt.Println("no ctx") },
	}
	go s.handler() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Function from struct field without ctx
//
// Function stored in struct field does not capture context.
func badStructFieldWithoutCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	holder := taskHolderErrgroup{
		task: func() error {
			fmt.Println("no ctx")
			return nil
		},
	}
	g.Go(holder.task) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Function from struct field without ctx
//
// Function stored in struct field does not capture context.
func badStructFieldWithoutCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	holder := taskHolderWaitGroup{
		task: func() {
			fmt.Println("no ctx")
		},
	}
	wg.Go(holder.task) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Struct field func argument without context
//
// A func in a struct field is looked up, as errgroup does.
func badStructFieldArg(ctx context.Context) {
	g := new(errgroup.Group)
	job := spawnerJob{Run: func() error { return nil }}
	runWithGroup(g, job.Run) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== Slice index =====

// [GOOD]: Index expression captures ctx
//
// Function in slice captures context.
func goodIndexExprCapturesCtx(ctx context.Context) {
	handlers := []func(){
		func() { _ = ctx },
	}
	go handlers[0]()
}

// [GOOD]: Slice index with ctx
//
// Function in slice element captures context.
func goodSliceIndexWithCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	tasks := []func() error{
		func() error {
			_ = ctx // Uses ctx
			return nil
		},
	}
	g.Go(tasks[0]) // OK - now tracked
	_ = g.Wait()
}

// [GOOD]: Slice index with ctx
//
// Function in slice element captures context.
func goodSliceIndexWithCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	tasks := []func(){
		func() {
			_ = ctx // Uses ctx
		},
	}
	wg.Go(tasks[0]) // OK - now tracked
	wg.Wait()
}

// [BAD]: Index expression captures ctx
//
// Function in slice does not capture context.
func badIndexExprMissingCtx(ctx context.Context) {
	handlers := []func(){
		func() { fmt.Println("no ctx") },
	}
	go handlers[0]() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Function from slice without ctx
//
// Function from slice without ctx - NOW TRACKED
func badSliceValueWithoutCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	tasks := []func() error{
		func() error { return nil },
	}
	g.Go(tasks[0]) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Function from slice without ctx
//
// Function from slice without ctx - NOW TRACKED
func badSliceValueWithoutCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	tasks := []func(){
		func() {},
	}
	wg.Go(tasks[0]) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Indexed func argument without context
//
// A func in a slice literal is looked up, as errgroup does.
func badIndexArg(ctx context.Context) {
	g := new(errgroup.Group)
	fns := []func() error{func() error { return nil }}
	runWithGroup(g, fns[0]) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== Map value =====

// [GOOD]: Map index expression captures ctx
//
// Function in map with string key captures context.
func goodMapIndexCapturesCtx(ctx context.Context) {
	handlers := map[string]func(){
		"work": func() { _ = ctx },
	}
	go handlers["work"]()
}

// [GOOD]: Map key with ctx
//
// Function in map value captures context.
func goodMapKeyWithCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	tasks := map[string]func() error{
		"key": func() error {
			_ = ctx // Uses ctx
			return nil
		},
	}
	g.Go(tasks["key"]) // OK - now tracked
	_ = g.Wait()
}

// [GOOD]: Map key with ctx
//
// Function in map value captures context.
func goodMapKeyWithCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	tasks := map[string]func(){
		"key": func() {
			_ = ctx // Uses ctx
		},
	}
	wg.Go(tasks["key"]) // OK - now tracked
	wg.Wait()
}

// [BAD]: Map index expression captures ctx
//
// Function in map with string key does not capture context.
func badMapIndexMissingCtx(ctx context.Context) {
	handlers := map[string]func(){
		"work": func() { fmt.Println("no ctx") },
	}
	go handlers["work"]() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Function from map without ctx
//
// Function from map without ctx - NOW TRACKED
func badMapValueWithoutCtxErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	tasks := map[string]func() error{
		"task1": func() error { return nil },
	}
	g.Go(tasks["task1"]) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Function from map without ctx
//
// Function from map without ctx - NOW TRACKED
func badMapValueWithoutCtxWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	tasks := map[string]func(){
		"task1": func() {},
	}
	wg.Go(tasks["task1"]) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// ===== Interface method =====

type myFactoryErrgroup struct{}

func (f *myFactoryErrgroup) CreateWorker(ctx context.Context) func() error {
	return func() error {
		_ = ctx // Implementation captures ctx
		return nil
	}
}

type myFactoryWaitGroup struct{}

func (f *myFactoryWaitGroup) CreateWorker(ctx context.Context) func() {
	return func() {
		_ = ctx // Implementation captures ctx
	}
}

// [GOOD]: Interface method with argument
//
// Context is passed as argument to interface method.
func goodInterfaceMethodWithCtxArgErrgroup(ctx context.Context, factory WorkerFactoryErrgroup) {
	g := new(errgroup.Group)
	// ctx IS passed as argument to CreateWorker - analyzer detects ctx usage
	g.Go(factory.CreateWorker(ctx)) // OK - ctx passed as argument
	_ = g.Wait()
}

// [GOOD]: Interface method with argument
//
// Context is passed as argument to interface method.
func goodInterfaceMethodWithCtxArgWaitGroup(ctx context.Context, factory WorkerFactoryWaitGroup) {
	var wg sync.WaitGroup
	// ctx IS passed as argument to CreateWorker - analyzer detects ctx usage
	wg.Go(factory.CreateWorker(ctx)) // OK - ctx passed as argument
	wg.Wait()
}

// [NOTCHECKED]: Interface method with argument
//
// Interface method call without context argument - not traced.
func badInterfaceMethodWithoutCtxArgNotTracedErrgroup(ctx context.Context, factory WorkerFactoryNoCtxErrgroup) {
	g := new(errgroup.Group)
	// ctx NOT passed to CreateWorker - should fail but can't trace
	g.Go(factory.CreateWorker()) // No error - can't trace interface methods
	_ = g.Wait()
}

// [NOTCHECKED]: Interface method with argument
//
// Interface method call without context argument - not traced.
func badInterfaceMethodWithoutCtxArgNotTracedWaitGroup(ctx context.Context, factory WorkerFactoryNoCtxWaitGroup) {
	var wg sync.WaitGroup
	// ctx NOT passed to CreateWorker - should fail but can't trace
	wg.Go(factory.CreateWorker()) // No error - can't trace interface methods
	wg.Wait()
}

// ===== Through an interface =====

// [LIMITATION]: Function through interface{} - ctx captured but not traced
//
// Context captured but lost through interface type assertion.
func goodFuncThroughInterfaceNotTracedErrgroup(ctx context.Context) {
	g := new(errgroup.Group)

	var i interface{} = func() error {
		_ = ctx // fn DOES capture ctx
		return nil
	}

	// Type assert to get func back
	fn := i.(func() error)
	// Can't trace through interface{} assertion, assume OK
	g.Go(fn) // No error - zero false positives policy
	_ = g.Wait()
}

// [LIMITATION]: Function through interface{} - ctx captured but not traced
//
// Context captured but lost through interface type assertion.
func goodFuncThroughInterfaceNotTracedWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup

	var i interface{} = func() {
		_ = ctx // fn DOES capture ctx
	}

	// Type assert to get func back
	fn := i.(func())
	// Can't trace through interface{} assertion, assume OK
	wg.Go(fn) // No error - zero false positives policy
	wg.Wait()
}

// [NOTCHECKED]: Function through interface without ctx
//
// Function through interface{} type assertion without context - not traced.
func badFuncThroughInterfaceWithoutCtxNotTracedErrgroup(ctx context.Context) {
	g := new(errgroup.Group)

	var i interface{} = func() error {
		fmt.Println("no ctx") // fn does NOT use ctx
		return nil
	}

	fn := i.(func() error)
	// Can't trace through interface{} assertion
	g.Go(fn) // No error - can't trace interface assertion
	_ = g.Wait()
}

// [NOTCHECKED]: Function through interface without ctx
//
// Function through interface{} type assertion without context - not traced.
func badFuncThroughInterfaceWithoutCtxNotTracedWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup

	var i interface{} = func() {
		fmt.Println("no ctx") // fn does NOT use ctx
	}

	fn := i.(func())
	// Can't trace through interface{} assertion
	wg.Go(fn) // No error - can't trace interface assertion
	wg.Wait()
}

// ===== From a channel =====

// [LIMITATION]: Function from channel - ctx captured but not traced
//
// Function received from channel cannot be traced statically.
func goodFuncFromChannelNotTracedErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	ch := make(chan func() error, 1)
	ch <- func() error {
		_ = ctx // The func DOES capture ctx
		return nil
	}
	fn := <-ch
	// Can't trace through channel receive, assume OK
	g.Go(fn) // No error - zero false positives policy
	_ = g.Wait()
}

// [LIMITATION]: Function from channel - ctx captured but not traced
//
// Function received from channel cannot be traced statically.
func goodFuncFromChannelNotTracedWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	ch := make(chan func(), 1)
	ch <- func() {
		_ = ctx // The func DOES capture ctx
	}
	fn := <-ch
	// Can't trace through channel receive, assume OK
	wg.Go(fn) // No error - zero false positives policy
	wg.Wait()
}

// ===== Through a spawner =====

// [GOOD]: Function with ctx passed through spawner
//
// Function with ctx passed through spawner - should pass
func goodFuncPassedThroughSpawnerErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error {
		_ = ctx // fn uses ctx
		return nil
	}
	runWithGroup(g, fn) // OK - fn uses ctx, and runWithGroup is marked as spawner
	_ = g.Wait()
}

// [GOOD]: Function with ctx passed through spawner
//
// Function with ctx passed through spawner - should pass
func goodFuncPassedThroughSpawnerWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	fn := func() {
		_ = ctx // fn uses ctx
	}
	runWithWaitGroupGo(&wg, fn) // OK - fn uses ctx, and runWithWaitGroup is marked as spawner
	wg.Wait()
}

// [BAD]: Function without ctx passed through spawner
//
// Function without ctx passed through spawner - should report
func badFuncPassedThroughSpawnerErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error {
		fmt.Println("no ctx")
		return nil
	}
	runWithGroup(g, fn) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Function without ctx passed through spawner
//
// Function without ctx passed through spawner - should report
func badFuncPassedThroughSpawnerWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	fn := func() {
		fmt.Println("no ctx")
	}
	runWithWaitGroupGo(&wg, fn) // want `runWithWaitGroupGo\(\) func argument should use context "ctx"`
	wg.Wait()
}

type taskHolderWithCtxErrgroup struct {
	task func() error
}

// When ctx is passed as argument, analyzer detects ctx usage.
type WorkerFactoryErrgroup interface {
	CreateWorker(ctx context.Context) func() error
}

type WorkerFactoryNoCtxErrgroup interface {
	CreateWorker() func() error
}

type taskHolderErrgroup struct {
	task func() error
}

type taskHolderWithCtxWaitGroup struct {
	task func()
}

// When ctx is passed as argument, analyzer detects ctx usage.
type WorkerFactoryWaitGroup interface {
	CreateWorker(ctx context.Context) func()
}

type WorkerFactoryNoCtxWaitGroup interface {
	CreateWorker() func()
}

//goroutinectx:spawner
func runWithWaitGroupGo(wg *sync.WaitGroup, fn func()) {
	wg.Go(fn)
}

type taskHolderWaitGroup struct {
	task func()
}

// spawnerJob holds a func in a field.
type spawnerJob struct {
	Run func() error
}
