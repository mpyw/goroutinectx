package linedirectivegen

import "context"

// The code below claims to be in generated.go, but this file is hand-written.
//
//line generated.go:1
func badGoroutineClaimingGenerated(ctx context.Context) {
	go func() {}() // want `goroutine does not propagate context "ctx"`
}
