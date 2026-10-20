// Package errgroup holds the errgroup cases that the other checkers do not
// share. The shared ones are in testdata/src/patterns.
// This file covers daily patterns: TryGo and errgroup.WithContext.
// See evil.go for adversarial tests.
package errgroup

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

// ===== SHOULD REPORT =====

// [BAD]: Literal without ctx - TryGo
//
// TryGo without ctx
func badErrgroupTryGo(ctx context.Context) {
	g := new(errgroup.Group)
	g.TryGo(func() error { // want `errgroup.Group.TryGo\(\) closure should use context "ctx"`
		fmt.Println("no context")
		return nil
	})
	_ = g.Wait()
}

// ===== SHOULD NOT REPORT =====

// [GOOD]: Literal with derived ctx - errgroup.WithContext
//
// errgroup.WithContext pattern
func goodErrgroupWithContext(ctx context.Context) {
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		_ = ctx.Done()
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: Derived ctx in for loop with function variable
//
// Real-world production pattern: derived ctx used in closure inside for loop
func goodErrgroupWithContextForLoop(ctx context.Context) {
	eg, ctx := errgroup.WithContext(ctx)
	for _, f := range []func(context.Context) error{doWork, doWork} {
		eg.Go(func() error {
			return f(ctx) // uses derived ctx
		})
	}
	_ = eg.Wait()
}

// ===== MULTIPLE CONTEXT PARAMETERS =====

func doWork(ctx context.Context) error {
	_ = ctx
	return nil
}
