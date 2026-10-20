// Package waitgroup holds the sync.WaitGroup cases that the other checkers do
// not share. The shared ones are in testdata/src/patterns.
package waitgroup

import (
	"context"
	"fmt"
	"sync"
)

// ===== SHOULD REPORT =====

// [BAD]: Literal without ctx - pointer receiver
//
// Pointer receiver variant
func badWaitGroupGoPtr(ctx context.Context) {
	wg := new(sync.WaitGroup)
	wg.Go(func() { // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
		fmt.Println("no context")
	})
	wg.Wait()
}

// ===== SHOULD NOT REPORT =====

// [GOOD]: Traditional pattern (Add/Done)
//
// Traditional pattern (Add/Done) - not checked by waitgroup checker
func goodTraditionalPattern(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = ctx.Done()
	}()
	wg.Wait()
}
