// Package conc contains test fixtures for the conc context propagation checker.
// This file covers stream package patterns - Stream.Go.
package conc

import (
	"context"
	"fmt"

	"github.com/sourcegraph/conc/stream"
)

// ===== stream.Stream.Go =====

// [BAD]: stream.Stream.Go without ctx
//
// Stream.Go task does not use context.
func badStreamGo(ctx context.Context) {
	s := stream.New()
	s.Go(func() stream.Callback { // want `stream.Stream.Go\(\) task should use context "ctx"`
		fmt.Println("task")
		return func() {
			fmt.Println("callback")
		}
	})
	s.Wait()
}

// [GOOD]: stream.Stream.Go with ctx
//
// Stream.Go task uses context.
func goodStreamGo(ctx context.Context) {
	s := stream.New()
	s.Go(func() stream.Callback {
		_ = ctx
		fmt.Println("task")
		return func() {
			fmt.Println("callback")
		}
	})
	s.Wait()
}

// [GOOD]: stream.Stream.Go no ctx param
//
// No context parameter - not checked.
func goodStreamGoNoCtxParam() {
	s := stream.New()
	s.Go(func() stream.Callback {
		fmt.Println("task")
		return func() {
			fmt.Println("callback")
		}
	})
	s.Wait()
}

// [BAD]: stream.Stream.Go variable func without ctx
//
// Variable task function without context.
func badStreamGoVariableFunc(ctx context.Context) {
	s := stream.New()
	task := func() stream.Callback {
		fmt.Println("task")
		return func() {
			fmt.Println("callback")
		}
	}
	s.Go(task) // want `stream.Stream.Go\(\) task should use context "ctx"`
	s.Wait()
}

// [GOOD]: stream.Stream.Go variable func with ctx
//
// Variable task function uses context.
func goodStreamGoVariableFunc(ctx context.Context) {
	s := stream.New()
	task := func() stream.Callback {
		_ = ctx
		fmt.Println("task")
		return func() {
			fmt.Println("callback")
		}
	}
	s.Go(task) // OK - task uses ctx
	s.Wait()
}

// ===== Loop patterns =====

// [BAD]: stream.Stream.Go in loop without ctx
//
// Stream.Go in loop without context.
func badStreamGoLoop(ctx context.Context) {
	s := stream.New()
	for i := 0; i < 3; i++ {
		s.Go(func() stream.Callback { // want `stream.Stream.Go\(\) task should use context "ctx"`
			return nil
		})
	}
	s.Wait()
}

// [GOOD]: stream.Stream.Go in loop with ctx
//
// Stream.Go in loop with context.
func goodStreamGoLoop(ctx context.Context) {
	s := stream.New()
	for i := 0; i < 3; i++ {
		s.Go(func() stream.Callback {
			_ = ctx
			return nil
		})
	}
	s.Wait()
}
