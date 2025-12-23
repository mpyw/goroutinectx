package context

import (
	"go/ast"
	"go/token"
	"go/types"
)

// BlockReturnsContextUsingFunc checks if a block's return statements
// return functions that use context. Recursively checks nested func literals.
// excludeFuncLit can be set to exclude a specific FuncLit from being counted (e.g., the parent).
//
// Example (returns context-using func - returns true):
//
//	func makeWorker(ctx context.Context) func() {
//	    return func() {
//	        doWork(ctx)  // returned func uses ctx
//	    }
//	}
//
// Example (returned func doesn't use context - returns false):
//
//	func makeWorker(ctx context.Context) func() {
//	    return func() {
//	        doWork()  // returned func ignores ctx
//	    }
//	}
func (c *CheckContext) BlockReturnsContextUsingFunc(body *ast.BlockStmt, excludeFuncLit *ast.FuncLit) bool {
	if body == nil {
		return true // No body to check
	}

	usesContext := false

	ast.Inspect(body, func(n ast.Node) bool {
		if usesContext {
			return false
		}
		// For nested func literals, check both direct usage and returned values
		if fl, ok := n.(*ast.FuncLit); ok && fl != excludeFuncLit {
			// Check if this nested func lit uses context directly
			if c.FuncLitUsesContext(fl) {
				usesContext = true
				return false
			}
			// Recursively check if it returns functions that use context
			if c.BlockReturnsContextUsingFunc(fl.Body, fl) {
				usesContext = true
				return false
			}
			return false // Don't descend into nested func literals
		}

		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		for _, result := range ret.Results {
			if c.returnedValueUsesContext(result) {
				usesContext = true
				return false
			}
		}
		return true
	})

	return usesContext
}

// FactoryReturnsContextUsingFunc checks if a factory FuncLit's return statements
// return functions that use context.
//
// Example (factory returns context-using func - returns true):
//
//	func example(ctx context.Context) {
//	    g.Go(func() func() error {  // <-- factory func lit
//	        return func() error {
//	            return doWork(ctx)  // returned func uses ctx
//	        }
//	    }())
//	}
func (c *CheckContext) FactoryReturnsContextUsingFunc(factory *ast.FuncLit) bool {
	return c.BlockReturnsContextUsingFunc(factory.Body, factory)
}

// FactoryCallReturnsContextUsingFunc checks if a factory call returns a context-using func.
// Handles: fn(ctx), fn() where fn captures ctx, (func(){...})(), and nested calls.
//
// Example (ctx passed to factory - returns true):
//
//	g.Go(makeWorker(ctx))  // ctx is passed to makeWorker
//
// Example (IIFE factory returns context-using func - returns true):
//
//	func example(ctx context.Context) {
//	    g.Go(func() func() error {
//	        return func() error { return doWork(ctx) }
//	    }())  // IIFE returns func using ctx
//	}
//
// Example (variable factory - returns true):
//
//	func example(ctx context.Context) {
//	    factory := func() func() error {
//	        return func() error { return doWork(ctx) }
//	    }
//	    g.Go(factory())  // factory returns func using ctx
//	}
func (c *CheckContext) FactoryCallReturnsContextUsingFunc(call *ast.CallExpr) bool {
	// Check if ctx is passed as an argument to the call
	if c.ArgsUseContext(call.Args) {
		return true
	}

	// Check if the factory function itself is a closure that captures ctx
	switch fun := call.Fun.(type) {
	case *ast.FuncLit:
		if c.FuncLitHasContextParam(fun) {
			return true
		}
		return c.FactoryReturnsContextUsingFunc(fun)

	case *ast.Ident:
		return c.IdentFactoryReturnsContextUsingFunc(fun)

	case *ast.CallExpr:
		// Handle nested CallExpr for deeper chains like fn()()()
		return c.FactoryCallReturnsContextUsingFunc(fun)
	}

	return true // Can't analyze, assume OK
}

// IdentFactoryReturnsContextUsingFunc checks if an identifier refers to a factory
// that returns a context-using func. Handles both local variables and package-level functions.
//
// Example (local variable factory - returns true):
//
//	func example(ctx context.Context) {
//	    makeWorker := func() func() error {
//	        return func() error { return doWork(ctx) }
//	    }
//	    g.Go(makeWorker())  // makeWorker is a local var
//	}
//
// Example (package-level factory - returns true):
//
//	func makeWorker(ctx context.Context) func() error {
//	    return func() error { return doWork(ctx) }
//	}
//
//	func example(ctx context.Context) {
//	    g.Go(makeWorker(ctx))  // makeWorker is package-level func
//	}
func (c *CheckContext) IdentFactoryReturnsContextUsingFunc(ident *ast.Ident) bool {
	obj := c.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return true // Can't trace, assume OK
	}

	// Handle local variable pointing to a func literal
	if v := c.VarOf(ident); v != nil {
		funcLit := c.FuncLitAssignedTo(v, token.NoPos)
		if funcLit == nil {
			return true // Can't trace, assume OK
		}
		// Skip if closure has its own context parameter
		if c.FuncLitHasContextParam(funcLit) {
			return true
		}
		return c.FactoryReturnsContextUsingFunc(funcLit)
	}

	// Handle package-level function declaration
	if fn, ok := obj.(*types.Func); ok {
		funcDecl := c.FuncDeclOf(fn)
		if funcDecl == nil {
			return true // Can't trace, assume OK
		}
		// Skip if function has context parameter
		if c.FuncTypeHasContextParam(funcDecl.Type) {
			return true
		}
		return c.BlockReturnsContextUsingFunc(funcDecl.Body, nil)
	}

	return true // Can't analyze, assume OK
}

// returnedValueUsesContext checks if a returned value is a func that uses context.
func (c *CheckContext) returnedValueUsesContext(result ast.Expr) bool {
	// If it's a func literal, check directly
	if innerFuncLit, ok := result.(*ast.FuncLit); ok {
		return c.FuncLitUsesContext(innerFuncLit)
	}

	// If it's an identifier (variable), find its assignment
	ident, ok := result.(*ast.Ident)
	if !ok {
		return false
	}

	innerFuncLit := c.FuncLitOfIdent(ident, token.NoPos)
	if innerFuncLit == nil {
		return false
	}

	return c.FuncLitUsesContext(innerFuncLit)
}
