// Package spawner provides //goroutinectx:spawner directive parsing.
//
// # Overview
//
// The spawner directive marks a function as spawning goroutines with
// its function arguments. The analyzer then checks that closures passed
// to these functions properly propagate context.
//
// # Directive Usage
//
// Mark a function with the directive in its doc comment:
//
//	//goroutinectx:spawner
//	func runInBackground(fn func()) {
//	    go fn()  // fn is spawned as goroutine
//	}
//
// The analyzer will then warn when closures don't use context:
//
//	func handler(ctx context.Context) {
//	    runInBackground(func() {
//	        doWork()  // Warning: should use context
//	    })
//	}
//
// # Parsing
//
// Use [Parse] to find all spawner-marked functions in a package:
//
//	spawners := spawner.Parse(pass)
//
// This returns a set of *types.Func that are marked as spawners.
//
// # Map Structure
//
// The returned map can be queried with [Map.IsSpawner]:
//
//	spawners := spawner.Parse(pass)
//	if spawners.IsSpawner(fn) {
//	    // Function is marked with //goroutinectx:spawner
//	}
//
// # Multiple Function Arguments
//
// When a spawner function takes multiple function arguments, all are checked:
//
//	//goroutinectx:spawner
//	func fanOut(tasks ...func()) {
//	    for _, task := range tasks {
//	        go task()
//	    }
//	}
//
//	func handler(ctx context.Context) {
//	    fanOut(
//	        func() { taskA() },      // Warning: should use context
//	        func() { taskB(ctx) },   // OK
//	    )
//	}
//
// # External Spawners
//
// For functions in external packages, use the -external-spawner flag:
//
//	-external-spawner=mycompany/pkg.RunAsync
//
// The directive is only for functions in the analyzed package.
//
// # Interaction with Checkers
//
// The spawner directive affects the [checkers.SpawnerChecker] which:
//  1. Detects calls to spawner-marked functions
//  2. Identifies function-typed arguments
//  3. Checks if those functions propagate context
package spawner
