// Package deriver provides derive function matching with OR/AND semantics.
//
// # Overview
//
// When the -goroutine-deriver flag is set, this package parses its value
// into a [Matcher] that tells whether a function is one of the derivers.
// Whether a goroutine calls them is judged by flow.ClosureCallsDeriver.
//
// # Flag Syntax
//
// The -goroutine-deriver flag supports three operators:
//
//	Comma (,) = OR  : At least one group must be called
//	Plus  (+) = AND : All functions of a group must be called
//	Mixed       : Combination of OR and AND
//
// Examples:
//
//	# Single deriver
//	-goroutine-deriver=github.com/org/apm.NewGoroutineContext
//
//	# OR - either function satisfies
//	-goroutine-deriver=github.com/org/apm.NewGoroutineContext,github.com/org/otel.StartSpan
//
//	# AND - both functions must be called
//	-goroutine-deriver=github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+github.com/newrelic/go-agent/v3/newrelic.NewContext
//
// # Function Specification Format
//
// Functions are named with the full import path of their package. A short
// name such as apm.NewGoroutineContext is rejected, since it never matches;
// a standard library package such as context is fine.
//
//	# Package function
//	github.com/example/pkg.FuncName
//
//	# Method on type
//	github.com/example/pkg.TypeName.MethodName
//
// # Matcher Usage
//
// Build a matcher from the OR groups with [NewMatcher]. The flag splits
// its value at commas into the groups. The golangci-lint plugin and
// goroutinectx.Config take them as a list:
//
//	matcher, err := deriver.NewMatcher([]string{"A+B", "C"})
//	if err != nil {
//	    // A group is empty, or a specification names no import path
//	}
//
// [Matcher.MatchesFunc] tells whether a called function is any of the
// derivers. flow.ClosureCallsDeriver walks a goroutine's function and tells
// whether an OR group is called at its start, or only in defer:
//
//	result := flow.ClosureCallsDeriver(fn, matcher)
//	if result.FoundAtStart {
//	    // At least one OR group is fully called at the start
//	}
//
// # Matcher Structure
//
//	type Matcher struct {
//	    OrGroups [][]funcspec.Spec  // Each group = AND, groups = OR
//	    Original string             // The groups joined by commas, for messages
//	}
//
// Example for "A+B,C", or the groups ["A+B", "C"]:
//
//	OrGroups = [
//	    [A, B],  // First AND group
//	    [C],     // Second AND group (single element)
//	]
//
// The matcher is satisfied if:
//   - Group 0: Both A AND B are called, OR
//   - Group 1: C is called
//
// # Error Messages
//
// [Matcher.Original] joins the groups with commas for messages, as in
// "goroutine should call <Original> to derive context".
//
// # Empty Matcher
//
// [Matcher.IsEmpty] tells that no derivers are configured. A nil Matcher is
// empty, so a run without -goroutine-deriver can pass nil.
package deriver
