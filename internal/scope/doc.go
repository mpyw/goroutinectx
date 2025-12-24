// Package scope provides context scope detection for functions.
//
// # Overview
//
// The scope package identifies functions that have context.Context parameters
// and tracks the parameter names for use in error messages.
//
// # Scope Detection
//
// A function has "context scope" if it declares a context.Context parameter
// or a configured carrier type parameter:
//
//	// Has context scope - ctx is tracked
//	func worker(ctx context.Context) {
//	    // ctx available: ["ctx"]
//	}
//
//	// Has context scope - multiple names tracked
//	func handler(reqCtx, bgCtx context.Context) {
//	    // ctx available: ["reqCtx", "bgCtx"]
//	}
//
//	// No context scope - no context parameter
//	func helper() {
//	    // Not analyzed for context propagation
//	}
//
// # Building Scope Map
//
// Use [Build] to create a scope map for all functions in a package:
//
//	funcScopes := scope.Build(pass, inspector, carriers)
//
// The resulting [Map] maps AST nodes (FuncDecl, FuncLit) to their [Scope]:
//
//	type Map map[ast.Node]*Scope
//
//	type Scope struct {
//	    CtxNames []string  // Context parameter names
//	}
//
// # Finding Enclosing Scope
//
// During analysis, use [FindEnclosing] to find the context scope for a node:
//
//	inspector.WithStack(nodeFilter, func(n ast.Node, push bool, stack []ast.Node) bool {
//	    scope := scope.FindEnclosing(funcScopes, stack)
//	    if scope == nil {
//	        return true  // No context in scope, skip analysis
//	    }
//	    // Analyze node with context available
//	    ctxName := scope.CtxName()  // Returns first name or "ctx"
//	    ...
//	})
//
// # Nested Functions
//
// Nested functions inherit context scope from their enclosing function:
//
//	func outer(ctx context.Context) {
//	    // outer has scope with ["ctx"]
//
//	    inner := func() {
//	        // inner does NOT have its own scope
//	        // FindEnclosing returns outer's scope
//	    }
//	}
//
// However, if a nested function has its own context parameter, it has its own scope:
//
//	func outer(outerCtx context.Context) {
//	    inner := func(innerCtx context.Context) {
//	        // inner has its own scope with ["innerCtx"]
//	    }
//	}
package scope
