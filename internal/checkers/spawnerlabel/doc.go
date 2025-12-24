// Package spawnerlabel provides spawner label directive validation.
//
// # Overview
//
// This package validates //goroutinectx:spawner directives by checking that
// the marked function actually calls one of its function arguments in a
// goroutine.
//
// # Why Validate?
//
// Without validation, users might incorrectly mark functions that don't
// actually spawn goroutines:
//
//	//goroutinectx:spawner  // Warning: does not spawn goroutine
//	func notASpawner(fn func()) {
//	    fn()  // Called synchronously, not in goroutine
//	}
//
// # Valid Spawner Patterns
//
// A valid spawner must call a function argument in a goroutine:
//
//	//goroutinectx:spawner
//	func validSpawner(fn func()) {
//	    go fn()  // OK: fn called in goroutine
//	}
//
//	//goroutinectx:spawner
//	func alsoValid(fn func()) {
//	    g.Go(func() error {
//	        fn()  // OK: fn called in errgroup goroutine
//	        return nil
//	    })
//	}
//
// # Checker Interface
//
// Unlike other checkers, spawnerlabel operates at the pass level rather
// than per-node. It implements a different interface:
//
//	type Checker struct {
//	    reg      *registry.Registry
//	    derivers *deriver.Matcher
//	}
//
//	func (c *Checker) Check(
//	    pass *analysis.Pass,
//	    spawners spawner.Map,
//	    ignoreMaps map[string]ignore.Map,
//	)
//
// # Integration
//
// The checker is called separately from the main checker loop:
//
//	spawnerlabelChecker := spawnerlabel.New(reg, derivers)
//	spawnerlabelChecker.Check(pass, spawners, ignoreMaps)
//
// # Error Messages
//
// When a spawner directive is misused:
//
//	//goroutinectx:spawner
//	func notSpawner(fn func()) {
//	    fn()  // Warning: function marked as spawner but does not spawn
//	          // its function arguments in a goroutine
//	}
package spawnerlabel
