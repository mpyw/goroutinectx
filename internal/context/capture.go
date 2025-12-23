package context

import (
	"go/ast"

	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// FuncLitCapturesContextSSA uses SSA analysis to check if a func literal captures context.
// Returns (result, true) if SSA analysis succeeded, or (false, false) if it failed.
//
// Example (captures context - returns true):
//
//	func example(ctx context.Context) {
//	    g.Go(func() error {
//	        return doWork(ctx)  // ctx is captured from outer scope
//	    })
//	}
//
// Example (does not capture - returns false):
//
//	func example(ctx context.Context) {
//	    g.Go(func() error {
//	        return doWork()  // ctx is not used
//	    })
//	}
func (c *CheckContext) FuncLitCapturesContextSSA(lit *ast.FuncLit) (bool, bool) {
	if c.SSAProg == nil || c.Tracer == nil {
		return false, false
	}

	// Skip if closure has its own context parameter
	if c.FuncLitHasContextParam(lit) {
		return true, true
	}

	ssaFn := c.SSAProg.FindFuncLit(lit)
	if ssaFn == nil {
		return false, false // SSA lookup failed
	}

	return c.Tracer.ClosureCapturesContext(ssaFn, c.Carriers), true
}

// FuncTypeHasContextParam checks if a function type has a context.Context parameter.
//
// Example (has context param - returns true):
//
//	func(ctx context.Context) error { ... }
//	func(ctx context.Context, data []byte) error { ... }
//
// Example (no context param - returns false):
//
//	func() error { ... }
//	func(data []byte) error { ... }
func (c *CheckContext) FuncTypeHasContextParam(fnType *ast.FuncType) bool {
	if fnType == nil || fnType.Params == nil {
		return false
	}
	for _, field := range fnType.Params.List {
		typ := c.Pass.TypesInfo.TypeOf(field.Type)
		if typ == nil {
			continue
		}
		if typeutil.IsContextType(typ) {
			return true
		}
	}
	return false
}

// FuncLitHasContextParam checks if a function literal has a context.Context parameter.
// This is a convenience wrapper around FuncTypeHasContextParam.
//
// Example (has context param - returns true):
//
//	g.Go(func(ctx context.Context) error {  // <-- this func lit
//	    return doWork(ctx)
//	})
func (c *CheckContext) FuncLitHasContextParam(lit *ast.FuncLit) bool {
	return c.FuncTypeHasContextParam(lit.Type)
}

// FuncLitCapturesContext checks if a func literal captures context (AST-based).
// Returns true if the func has its own context param, or if it uses a context from outer scope.
//
// Example (has own param - returns true):
//
//	g.Go(func(ctx context.Context) error { return nil })
//
// Example (captures outer context - returns true):
//
//	func example(ctx context.Context) {
//	    g.Go(func() error {
//	        return doWork(ctx)  // uses outer ctx
//	    })
//	}
//
// Example (does not capture - returns false):
//
//	func example(ctx context.Context) {
//	    g.Go(func() error {
//	        return doWork()  // ctx not used
//	    })
//	}
func (c *CheckContext) FuncLitCapturesContext(lit *ast.FuncLit) bool {
	return c.FuncLitHasContextParam(lit) || c.FuncLitUsesContext(lit)
}

// FuncLitUsesContext checks if a function literal references any context variable.
// It does NOT descend into nested func literals - they have their own scope and
// will be checked separately.
//
// Example (uses context - returns true):
//
//	func example(ctx context.Context) {
//	    g.Go(func() error {
//	        doWork(ctx)  // direct reference to ctx
//	        return nil
//	    })
//	}
//
// Example (nested closure NOT counted - returns false):
//
//	func example(ctx context.Context) {
//	    g.Go(func() error {
//	        // ctx used in nested closure, NOT in this func lit
//	        defer func() { _ = ctx }()
//	        return nil
//	    })
//	}
func (c *CheckContext) FuncLitUsesContext(lit *ast.FuncLit) bool {
	return c.nodeReferencesContext(lit.Body, true)
}

// ArgUsesContext checks if an expression references a context variable.
// Unlike FuncLitUsesContext, this DOES descend into nested func literals.
//
// Example (direct reference - returns true):
//
//	doWork(ctx)  // ctx is an arg
//
// Example (wrapped in func lit - returns true):
//
//	doWork(func() { _ = ctx })  // ctx used inside arg
func (c *CheckContext) ArgUsesContext(expr ast.Expr) bool {
	return c.nodeReferencesContext(expr, false)
}

// ArgsUseContext checks if any argument references a context variable.
//
// Example (one arg uses context - returns true):
//
//	makeWorker(ctx, data)  // ctx is passed
//
// Example (no context - returns false):
//
//	makeWorker(data, 123)  // no context in args
func (c *CheckContext) ArgsUseContext(args []ast.Expr) bool {
	for _, arg := range args {
		if c.ArgUsesContext(arg) {
			return true
		}
	}
	return false
}

// nodeReferencesContext checks if a node references any context variable.
// If skipNestedFuncLit is true, nested function literals are not traversed.
func (c *CheckContext) nodeReferencesContext(node ast.Node, skipNestedFuncLit bool) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		if skipNestedFuncLit {
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
		}
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		obj := c.Pass.TypesInfo.ObjectOf(ident)
		if obj == nil {
			return true
		}
		if typeutil.IsContextOrCarrierType(obj.Type(), c.Carriers) {
			found = true
			return false
		}
		return true
	})
	return found
}
