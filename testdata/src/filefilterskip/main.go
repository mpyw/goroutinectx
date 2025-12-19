// Package filefilterskip tests file filtering with -test=false.
// Tests that:
// - Generated files are always skipped (see generated.go)
// - Test files are skipped when -test=false (see code_test.go)
package filefilterskip

import (
	"context"
	"fmt"
)

// badGoroutine should be reported in regular files even with -test=false.
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
