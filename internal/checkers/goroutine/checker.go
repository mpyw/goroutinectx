// Package goroutine checks go statements for context propagation.
package goroutine

import (
	"go/ast"
	"go/types"

	"github.com/mpyw/goroutinectx/internal/context"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Checker checks go statements for context propagation.
type Checker struct{}

// New creates a new goroutine checker.
func New() *Checker {
	return &Checker{}
}

// CheckGoStmt implements checkers.GoStmtChecker.
func (c *Checker) CheckGoStmt(cctx *context.CheckContext, goStmt *ast.GoStmt) {
	call := goStmt.Call

	// Check if context is used in the goroutine call chain
	if !callUsesContext(cctx, call) {
		cctx.Reportf(goStmt.Pos(), "goroutine does not propagate context %q", cctx.Scope.Name)
	}
}

// callUsesContext checks if a call expression (or its chain) uses context.
// Handles patterns like:
//   - go func() { ... }()           -> check func literal body
//   - go fn()                        -> check arguments + returned func
//   - go fn()()                      -> check all levels recursively
//   - go fn(ctx)()                   -> ctx used in inner call
//   - go fn()(ctx)                   -> ctx used in outer call
func callUsesContext(cctx *context.CheckContext, call *ast.CallExpr) bool {
	// Check if any argument in this call uses context
	for _, arg := range call.Args {
		if cctx.Scope.UsesContext(cctx.Pass, arg) {
			return true
		}
	}

	// Check the function being called
	switch fun := call.Fun.(type) {
	case *ast.FuncLit:
		// go func() { ... }() - check the func literal body
		return cctx.CheckClosureUsesContext(fun)

	case *ast.CallExpr:
		// go fn()() - check both the inner call and what it returns
		// First check if inner call uses context in its arguments
		if callUsesContext(cctx, fun) {
			return true
		}
		// Then check if the returned function (if we can find it) uses context
		return returnedFuncUsesContext(cctx, fun)

	case *ast.Ident:
		// go fn() where fn is a variable holding a func
		// Check if the func literal assigned to fn uses context
		return identFuncUsesContext(cctx, fun)

	default:
		// go obj.Method() - already checked args above
		return false
	}
}

// identFuncUsesContext checks if a function stored in a variable uses context.
func identFuncUsesContext(cctx *context.CheckContext, ident *ast.Ident) bool {
	obj := cctx.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return false
	}

	v, ok := obj.(*types.Var)
	if !ok {
		return false
	}

	funcLit := cctx.FindFuncLitAssignment(v)
	if funcLit == nil {
		return false
	}

	return funcLitUsesContextOrReturnsCtxFunc(cctx, funcLit)
}

// returnedFuncUsesContext tries to find the function literal returned by a call
// and checks if it uses context.
func returnedFuncUsesContext(cctx *context.CheckContext, call *ast.CallExpr) bool {
	// First, check if ctx is passed as an argument to the call
	for _, arg := range call.Args {
		if cctx.Scope.UsesContext(cctx.Pass, arg) {
			return true
		}
	}

	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}

	obj := cctx.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return false
	}

	v, ok := obj.(*types.Var)
	if !ok {
		return false
	}

	funcLit := cctx.FindFuncLitAssignment(v)
	if funcLit == nil {
		return false
	}

	return cctx.FuncLitReturnUsesContext(funcLit)
}

// funcLitUsesContextOrReturnsCtxFunc checks if a func literal either:
// 1. Directly uses context in its body (including nested closures), OR
// 2. Returns another func that (recursively) uses context.
func funcLitUsesContextOrReturnsCtxFunc(cctx *context.CheckContext, funcLit *ast.FuncLit) bool {
	// Check if this func's body uses context (including nested closures)
	if usesContextDeep(cctx, funcLit.Body) {
		return true
	}

	// Check if this func returns another func that uses context
	return cctx.FuncLitReturnUsesContext(funcLit)
}

// usesContextDeep checks if the given AST node uses any context.Context variable,
// INCLUDING nested function literals.
// It checks for both the original context parameters AND any context.Context typed variable
// (to handle shadowing like `ctx := errgroup.WithContext(ctx)`).
func usesContextDeep(cctx *context.CheckContext, node ast.Node) bool {
	if cctx.Scope == nil || len(cctx.Scope.Vars) == 0 {
		return false
	}

	found := false

	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		// DO NOT skip nested function literals - we want to trace ctx
		// through closures like: captured := ctx; return func() { use(captured) }
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}

		obj := cctx.Pass.TypesInfo.ObjectOf(ident)
		if obj == nil {
			return true
		}

		// Check 1: Is it one of the original context parameters?
		if cctx.Scope.IsContextVar(obj) {
			found = true

			return false
		}

		// Check 2: Is it ANY variable with type context.Context?
		// This handles shadowing like: ctx := errgroup.WithContext(ctx)
		if v, ok := obj.(*types.Var); ok {
			if typeutil.IsContextType(v.Type()) {
				found = true

				return false
			}
		}

		return true
	})

	return found
}
