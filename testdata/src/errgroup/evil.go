// Package errgroup contains test fixtures for the errgroup context propagation checker.
// This file covers adversarial patterns - tests analyzer limits: higher-order functions,
// non-literal function arguments, interface methods.
// See basic.go for daily patterns.
package errgroup

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

// ===== FACTORY RETURN CONDITIONAL REASSIGNMENT =====
// These patterns test conditional reassignment of variables returned by factory functions.

var factoryConditionFlag bool

// [BAD]: Factory returns conditionally reassigned variable - first uses ctx
//
// Factory function returns a variable that is conditionally reassigned.
// First assignment uses context, but conditional reassignment doesn't.
func badFactoryConditionalReassignFirstUsesCtx(ctx context.Context) {
	g := new(errgroup.Group)
	makeWorker := func() func() error {
		worker := func() error {
			_ = ctx // First uses ctx
			return nil
		}
		if factoryConditionFlag {
			worker = func() error {
				fmt.Println("no ctx") // Conditional doesn't use ctx
				return nil
			}
		}
		return worker
	}
	g.Go(makeWorker()) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Factory returns conditionally reassigned variable - first doesn't use ctx
//
// Factory function returns a variable that is conditionally reassigned.
// First assignment ignores context, conditional reassignment uses it.
func badFactoryConditionalReassignFirstNoCtx(ctx context.Context) {
	g := new(errgroup.Group)
	makeWorker := func() func() error {
		worker := func() error {
			fmt.Println("no ctx") // First doesn't use ctx
			return nil
		}
		if factoryConditionFlag {
			worker = func() error {
				_ = ctx // Conditional uses ctx
				return nil
			}
		}
		return worker
	}
	g.Go(makeWorker()) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [GOOD]: Factory returns conditionally reassigned variable - all use ctx
//
// Factory function returns a variable that is conditionally reassigned.
// All assignments use context, so all paths propagate it.
func goodFactoryConditionalReassignAllUseCtx(ctx context.Context) {
	g := new(errgroup.Group)
	makeWorker := func() func() error {
		worker := func() error {
			_ = ctx // First uses ctx
			return nil
		}
		if factoryConditionFlag {
			worker = func() error {
				_ = ctx // Conditional also uses ctx
				return nil
			}
		}
		return worker
	}
	g.Go(makeWorker()) // OK - all assignments use ctx
	_ = g.Wait()
}

// ===== IIFE PATTERNS (DIRECT, WITHOUT PARENTHESES) =====

// [GOOD]: IIFE factory with context
//
// Immediately invoked function expression that captures context.
func goodIIFEFactoryWithCtx(ctx context.Context) {
	g := new(errgroup.Group)
	// Direct IIFE (no parentheses around func) - goes to *ast.FuncLit case
	g.Go(func() func() error {
		return func() error {
			_ = ctx
			return nil
		}
	}())
	_ = g.Wait()
}

// [BAD]: IIFE factory without context
//
// Immediately invoked function expression that does not capture context.
func badIIFEFactoryWithoutCtx(ctx context.Context) {
	g := new(errgroup.Group)
	// Direct IIFE (no parentheses around func) - goes to *ast.FuncLit case
	g.Go(func() func() error { // want `errgroup.Group.Go\(\) closure should use context "ctx"`
		return func() error {
			fmt.Println("no ctx")
			return nil
		}
	}())
	_ = g.Wait()
}

// [GOOD]: IIFE factory with context param
//
// Immediately invoked function expression with context parameter.
func goodIIFEFactoryWithCtxParam(ctx context.Context) {
	g := new(errgroup.Group)
	// IIFE with context param - FuncLitHasContextParam returns true
	g.Go(func(c context.Context) func() error {
		return func() error {
			_ = c
			return nil
		}
	}(ctx))
	_ = g.Wait()
}

// ===== NESTED FACTORY PATTERNS =====

func makeFactory() func() func() error {
	return func() func() error {
		return func() error {
			fmt.Println("no ctx")
			return nil
		}
	}
}

func makeFactoryWithCtx(ctx context.Context) func() func() error {
	return func() func() error {
		return func() error {
			_ = ctx
			return nil
		}
	}
}

// [GOOD]: Nested factory call with context
//
// Nested factory call where ctx is passed to outer factory.
func goodNestedFactoryWithCtx(ctx context.Context) {
	g := new(errgroup.Group)
	// makeFactoryWithCtx(ctx)() returns func() error - recursive CallExpr case
	g.Go(makeFactoryWithCtx(ctx)())
	_ = g.Wait()
}

// [BAD]: Nested factory call without context
//
// Nested factory call where no context is captured.
func badNestedFactoryWithoutCtx(ctx context.Context) {
	g := new(errgroup.Group)
	// makeFactory()() returns func() error - recursive CallExpr case
	g.Go(makeFactory()()) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}
