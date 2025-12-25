// Package internal provides the core analysis engine for goroutinectx.
//
// # Architecture Overview
//
// The analyzer follows a modular architecture with clear separation of concerns:
//
//	                            +------------------+
//	                            |   analyzer.go    |  Entry point
//	                            +--------+---------+
//	                                     |
//	                            +--------v---------+
//	                            |      Runner      |  Orchestration
//	                            +--------+---------+
//	                                     |
//	        +----------------------------+----------------------------+
//	        |                            |                            |
//	  +-----v-------+          +---------v----------+       +---------v----------+
//	  | GoStmtChecker|          |   CallChecker     |       |   SpawnerLabel     |
//	  | (go stmt)   |          |   (func calls)    |       |   (directives)     |
//	  +-----+-------+          +---------+----------+       +--------------------+
//	        |                            |
//	        +------------+---------------+
//	                     |
//	            +--------v---------+
//	            |   probe.Context  |  AST analysis helpers
//	            +--------+---------+
//	                     |
//	        +------------+------------+
//	        |            |            |
//	   +----v-----+ +----v-----+ +----v------+
//	   |  scope   | |   ssa    | | typeutil  |
//	   +----------+ +----------+ +-----------+
//
// # Checker Types
//
// There are two primary checker interfaces:
//
//   - [GoStmtChecker]: Checks go statements (e.g., `go func() { ... }()`)
//   - [CallChecker]: Checks function call expressions (e.g., `g.Go(func() { ... })`)
//
// Example checker registration:
//
//	goStmtCheckers := []GoStmtChecker{
//	    &checkers.Goroutine{},
//	    checkers.NewGoroutineDerive(deriveMatcher),
//	}
//	callCheckers := []CallChecker{
//	    checkers.NewErrgroupChecker(deriveMatcher),
//	    checkers.NewWaitgroupChecker(deriveMatcher),
//	}
//
// # Execution Flow
//
//  1. [Runner.Run] receives the analysis pass and AST inspector
//  2. [scope.Build] identifies functions with context parameters
//  3. Inspector walks the AST with a node filter
//  4. For each node in a context-aware scope:
//     - go statements -> [GoStmtChecker.CheckGoStmt]
//     - call expressions -> [CallChecker.CheckCall]
//  5. Results are reported via pass.Reportf
//
// # Result Handling
//
// Checkers return [Result] to indicate pass/fail:
//
//	// Pass - no issue found
//	return internal.OK()
//
//	// Fail with message
//	return internal.Fail("goroutine does not propagate context")
//
//	// Fail with defer-specific message (for derive checks)
//	return internal.FailWithDefer(
//	    "goroutine should call deriver",
//	    "goroutine calls deriver in defer, but should call at start",
//	)
package internal
