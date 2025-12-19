// Package filefilter tests file filtering functionality.
// Tests that:
// - Generated files are always skipped (see generated.go)
// - Test files are analyzed by default (see code_test.go)
package filefilter

import (
	"context"
	"fmt"
)

// badGoroutine should be reported in regular files.
func badGoroutine(ctx context.Context) {
	go func() { // want `goroutine does not propagate context "ctx"`
		fmt.Println("no context")
	}()
}

// goodGoroutine properly uses context.
func goodGoroutine(ctx context.Context) {
	go func() {
		_ = ctx
	}()
}
