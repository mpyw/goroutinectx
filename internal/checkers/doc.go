// Package checkers provides individual checker implementations for goroutinectx.
//
// # Checker Overview
//
// This package contains implementations for detecting context propagation issues:
//
//	┌─────────────────────────────────────────────────────────────────────┐
//	│                         Checker Types                               │
//	├──────────────────────┬──────────────────────────────────────────────┤
//	│ GoStmtChecker        │ Checks go statements                         │
//	│  - Goroutine         │ go func() { ... }() without ctx              │
//	│  - GoroutineDerive   │ go func() { ... }() without deriver call     │
//	├──────────────────────┼──────────────────────────────────────────────┤
//	│ CallChecker          │ Checks function call expressions             │
//	│  - CallArgChecker    │ Generic callback argument checker            │
//	│    - Errgroup        │ errgroup.Group.Go() callbacks                │
//	│    - Waitgroup       │ sync.WaitGroup.Go() callbacks (Go 1.25+)     │
//	│    - Conc            │ github.com/sourcegraph/conc callbacks        │
//	│  - SpawnerChecker    │ //goroutinectx:spawner marked functions      │
//	│  - GotaskChecker     │ gotask library functions                     │
//	└──────────────────────┴──────────────────────────────────────────────┘
//
// # GoStmtChecker
//
// Checks for go statements that don't properly propagate context:
//
//	// Bad - context not captured
//	func worker(ctx context.Context) {
//	    go func() {           // <- Warning: goroutine does not propagate context
//	        doWork()
//	    }()
//	}
//
//	// Good - context captured
//	func worker(ctx context.Context) {
//	    go func() {
//	        doWork(ctx)       // <- OK: context used
//	    }()
//	}
//
// # CallArgChecker
//
// Factory functions create checkers for specific APIs:
//
//	checker := NewErrgroupChecker(deriveMatcher)
//	checker := NewWaitgroupChecker(deriveMatcher)
//	checker := NewConcChecker(deriveMatcher)
//
// Example detection:
//
//	func worker(ctx context.Context) {
//	    g, _ := errgroup.WithContext(ctx)
//	    g.Go(func() error {   // <- Warning: closure should use context
//	        return doWork()
//	    })
//	}
//
// # GoroutineDerive
//
// When configured with -goroutine-deriver flag, checks that goroutines call
// a specific context derivation function:
//
//	// With -goroutine-deriver=apm.NewGoroutineContext
//	func worker(ctx context.Context) {
//	    go func() {
//	        ctx := apm.NewGoroutineContext(ctx)  // <- Required
//	        doWork(ctx)
//	    }()
//	}
//
// # Gotask Checker
//
// Checks gotask library usage for proper context derivation:
//
//	// gotask.Do* functions - task arguments must call deriver
//	gotask.DoAll(ctx,
//	    gotask.NewTask(func(ctx context.Context) error {
//	        ctx = apm.NewGoroutineContext(ctx)  // <- Required in task body
//	        return doWork(ctx)
//	    }),
//	)
package checkers
