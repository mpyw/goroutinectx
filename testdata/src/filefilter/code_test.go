package filefilter

import (
	"context"
	"fmt"
)

// badGoroutineInTest is reported when -test=true (default).
func badGoroutineInTest(ctx context.Context) {
	go func() { // want `goroutine does not propagate context "ctx"`
		fmt.Println("no context in test file")
	}()
}
