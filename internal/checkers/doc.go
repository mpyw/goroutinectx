// Package checkers defines goroutinectx's checkers, in one table and one
// file per kind. The analyzer's flags, the settings of goroutinectx.New and
// of the golangci-lint plugin, and the checkers Run builds all come from
// [Defs]. The checkers' names are in internal/checkername.
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
//	│  - SpawnCallback     │ Callback argument checker of one table entry │
//	│    - Errgroup        │ errgroup.Group.Go() callbacks                │
//	│    - Waitgroup       │ sync.WaitGroup.Go() callbacks (Go 1.25+)     │
//	│    - Conc            │ github.com/sourcegraph/conc callbacks        │
//	│  - SpawnerChecker    │ //goroutinectx:spawner marked functions      │
//	│  - GotaskChecker     │ gotask library functions                     │
//	├──────────────────────┼──────────────────────────────────────────────┤
//	│ PackageChecker       │ Checks the whole package, after the others   │
//	│  - Spawnerlabel      │ //goroutinectx:spawner missing or unneeded   │
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
// # SpawnCallbackChecker
//
// One checker type covers errgroup, waitgroup and conc. It checks the calls
// to the registry entries of its checker name:
//
//	c := newSpawnCallbackChecker(checkername.Errgroup, deriveMatcher)
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
//
// # Spawnerlabel Checker
//
// Checks that //goroutinectx:spawner labels the functions that spawn, and
// only those. It runs with -spawnerlabel.
//
// A function needs the label when it calls a supported API, such as
// errgroup.Group.Go, with a func argument, or calls another spawner with one,
// or calls gotask's DoAsync:
//
//	func runTask(task func() error) { // should have //goroutinectx:spawner
//	    g := new(errgroup.Group)
//	    g.Go(task)
//	    _ = g.Wait()
//	}
//
// The calls made in the closures the function runs count too: IIFEs, a
// closure called through a variable, the goroutines it starts, and the
// closures a call returns when that result is called. A closure it only
// makes, and does not run, does not count. A go statement on its own, as
// go fn(), is not such a call.
//
// A labeled function needs it only while it makes such a call or takes a
// func parameter, the arguments the spawner checker checks. A type parameter
// whose constraint allows only func types, as F ~func() does, is one:
//
//	//goroutinectx:spawner // unnecessary
//	func simpleHelper() {
//	    fmt.Println("hello")
//	}
//
// It runs over the whole pass, after the per-node checkers:
//
//	diags := newSpawnerlabelChecker(spawners, ssaProg, f).Check(pass, skipFiles)
package checkers
