// Package externalspawner tests the -external-spawner flag.
package externalspawner

import (
	"context"
	"fmt"

	"github.com/example/workerpool"
)

// ===== SHOULD REPORT =====

// [BAD]: Pool.Submit without ctx
//
// External spawner method call without ctx
func badPoolSubmit(ctx context.Context) {
	p := &workerpool.Pool{}
	p.Submit(func() { // want `Submit\(\) func argument should use context "ctx"`
		fmt.Println("no ctx")
	})
}

// [BAD]: Run without ctx
//
// External spawner package function without ctx
func badRun(ctx context.Context) {
	workerpool.Run(func() { // want `Run\(\) func argument should use context "ctx"`
		fmt.Println("no ctx")
	})
}

// ===== SHOULD NOT REPORT =====

// [GOOD]: Pool.Submit with ctx
//
// External spawner method call with ctx
func goodPoolSubmit(ctx context.Context) {
	p := &workerpool.Pool{}
	p.Submit(func() {
		_ = ctx.Done()
	})
}

// [GOOD]: Run with ctx
//
// External spawner package function with ctx
func goodRun(ctx context.Context) {
	workerpool.Run(func() {
		_ = ctx.Done()
	})
}

// [GOOD]: No ctx param
//
// No context parameter - not checked
func goodNoCtxParam() {
	p := &workerpool.Pool{}
	p.Submit(func() {
		fmt.Println("ok")
	})
}
