// Package deriver provides derive function matching with OR/AND semantics.
//
// # Overview
//
// When using -goroutine-deriver flag, this package parses the flag value
// and provides matching logic for detecting deriver function calls.
//
// # Flag Syntax
//
// The -goroutine-deriver flag supports three operators:
//
//	Comma (,) = OR  : At least one function must be called
//	Plus  (+) = AND : All functions must be called
//	Mixed       : Combination of OR and AND
//
// Examples:
//
//	# Single deriver
//	-goroutine-deriver=apm.NewGoroutineContext
//
//	# OR - either function satisfies
//	-goroutine-deriver=apm.NewGoroutineContext,otel.StartSpan
//
//	# AND - both functions must be called
//	-goroutine-deriver=apm.NewGoroutineContext+trace.StartSpan
//
//	# Mixed - (A AND B) OR C
//	-goroutine-deriver=apm.Func+trace.Func,otel.Func
//
// # Function Specification Format
//
// Functions are specified as package path + function/method name:
//
//	# Package function
//	github.com/example/pkg.FuncName
//
//	# Method on type
//	github.com/example/pkg.TypeName.MethodName
//
// # Matcher Usage
//
// Parse the flag value to create a [Matcher]:
//
//	matcher := deriver.Parse("-goroutine-deriver", flagValue)
//	if matcher == nil {
//	    // No deriver configured
//	}
//
// Check if a code block satisfies the matcher:
//
//	// For AST analysis
//	if matcher.SatisfiesAnyGroup(pass, funcBody) {
//	    // At least one OR group is fully satisfied
//	}
//
// # Matcher Structure
//
//	type Matcher struct {
//	    OrGroups [][]funcspec.Spec  // Each group = AND, groups = OR
//	    Original string             // Original flag value for messages
//	}
//
// Example for "A+B,C":
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
// The [Matcher.Original] field preserves the flag value for error messages:
//
//	return internal.Fail(
//	    "goroutine should call " + matcher.Original + " to derive context",
//	)
//
// # Empty Matcher
//
// Use [Matcher.IsEmpty] to check if no derivers are configured:
//
//	if matcher == nil || matcher.IsEmpty() {
//	    return internal.OK()  // No derive check required
//	}
package deriver
