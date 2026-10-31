// Methods of captured values whose context field is unexported, run as a
// module, so that the analyzer knows which packages are of the module.

package use

import (
	"context"
	"sync"
	"testing"

	"example.com/app/holder"
)

//goroutinectx:spawner
func spawn(fn func()) {
	go fn()
}

// ===== Method of a captured value of another package of the module =====

// [GOOD]: Method of a captured value of another package of the module
//
// The module can have put ctx in the unexported field, as holder.New does.
func goodModuleServiceMethodGoroutine(ctx context.Context) {
	s := holder.New(ctx)
	go func() { s.Run() }()
}

// [GOOD]: Method of a captured value of another package of the module
//
// The module can have put ctx in the unexported field, as holder.New does.
func goodModuleServiceMethodWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	s := holder.New(ctx)
	wg.Go(func() { s.Run() })
	wg.Wait()
}

// [GOOD]: Method of a captured value of another package of the module
//
// The module can have put ctx in the unexported field, as holder.New does.
func goodModuleServiceMethodSpawner(ctx context.Context) {
	s := holder.New(ctx)
	spawn(func() { s.Run() })
}

// ===== Method of a captured value of the standard library =====

// [BAD]: Method of a captured value of the standard library
//
// testing is not of the module, so the unexported context of t is not ctx
// (#191).
func badStdTestingTMethodGoroutine(ctx context.Context, t *testing.T) {
	go func() { t.Log("x") }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Method of a captured value of the standard library
//
// testing is not of the module, so the unexported context of t is not ctx
// (#191).
func badStdTestingTMethodWaitGroup(ctx context.Context, t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() { t.Log("x") }) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Method of a captured value of the standard library
//
// testing is not of the module, so the unexported context of t is not ctx
// (#191).
func badStdTestingTMethodSpawner(ctx context.Context, t *testing.T) {
	spawn(func() { t.Log("x") }) // want `spawn\(\) func argument should use context "ctx"`
}
