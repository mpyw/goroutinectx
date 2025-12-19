// Package goroutinederive checks that goroutines call specified functions
// to derive a new context (e.g., apm.NewGoroutine).
package goroutinederive

import (
	"go/ast"
	"go/types"

	"github.com/mpyw/goroutinectx/internal/context"
	"github.com/mpyw/goroutinectx/internal/directives/deriver"
	"github.com/mpyw/goroutinectx/internal/directives/ignore"
)

// Checker checks that goroutines call specified functions to derive context.
//
// Supports OR (comma) and AND (plus) operators:
//   - OR:  "pkg1.Func1,pkg2.Func2" - at least one must be called
//   - AND: "pkg1.Func1+pkg2.Func2" - all must be called
//   - Mixed: "pkg1.Func1+pkg2.Func2,pkg3.Func3" - (Func1 AND Func2) OR Func3
type Checker struct {
	derives *deriver.Matcher
}

// New creates a new goroutine-derive checker.
// The deriveFuncsStr supports OR (comma) and AND (plus) operators.
// Format: "pkg/path.Func" or "pkg/path.Type.Method".
func New(deriveFuncsStr string) *Checker {
	return &Checker{
		derives: deriver.NewMatcher(deriveFuncsStr),
	}
}

// CheckGoStmt implements checkers.GoStmtChecker.
func (c *Checker) CheckGoStmt(cctx *context.CheckContext, goStmt *ast.GoStmt) {
	call := goStmt.Call

	// Check if ANY OR group is satisfied in the goroutine
	if !c.callSatisfiesDerive(cctx, call) {
		cctx.Reportf(ignore.GoroutineDerive, goStmt.Pos(), "goroutine should call %s to derive context", c.derives.Original)
	}
}

// callSatisfiesDerive checks if a call expression (or its chain) satisfies derive requirements.
// Handles patterns like:
//   - go func() { ... }()           -> check func literal body
//   - go fn()                        -> check func assigned to fn
//   - go fn()()                      -> check returned func from fn()
//   - go fn(ctx)()                   -> check returned func
func (c *Checker) callSatisfiesDerive(cctx *context.CheckContext, call *ast.CallExpr) bool {
	switch fun := call.Fun.(type) {
	case *ast.FuncLit:
		// go func() { ... }()
		// If the func literal has its own context parameter, it will be checked in its own scope
		if context.HasContextOrCarrierParam(cctx.Pass, fun.Type, cctx.Carriers) {
			return true
		}

		return c.derives.SatisfiesAnyGroup(cctx.Pass, fun.Body)

	case *ast.CallExpr:
		// go fn()() - check the returned function
		return c.returnedFuncSatisfiesDerive(cctx, fun)

	case *ast.Ident:
		// go fn() where fn is a variable
		return c.identFuncSatisfiesDerive(cctx, fun)

	default:
		// go obj.Method() - can't trace, assume it's okay
		return true
	}
}

// identFuncSatisfiesDerive checks if a function stored in a variable satisfies derive.
func (c *Checker) identFuncSatisfiesDerive(cctx *context.CheckContext, ident *ast.Ident) bool {
	obj := cctx.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return true // Can't trace, assume okay
	}

	v, ok := obj.(*types.Var)
	if !ok {
		return true
	}

	// Use the identifier's position to find the LAST assignment before usage
	funcLit := cctx.FindFuncLitAssignmentBefore(v, ident.Pos())
	if funcLit == nil {
		return true // Can't find assignment, assume okay
	}

	if context.HasContextOrCarrierParam(cctx.Pass, funcLit.Type, cctx.Carriers) {
		return true
	}

	return c.derives.SatisfiesAnyGroup(cctx.Pass, funcLit.Body)
}

// returnedFuncSatisfiesDerive checks if a call returns a func that satisfies derive.
func (c *Checker) returnedFuncSatisfiesDerive(cctx *context.CheckContext, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return true // Can't trace, assume okay
	}

	obj := cctx.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return true
	}

	v, ok := obj.(*types.Var)
	if !ok {
		return true
	}

	// Use the identifier's position to find the LAST assignment before usage
	funcLit := cctx.FindFuncLitAssignmentBefore(v, ident.Pos())
	if funcLit == nil {
		return true
	}

	// Check if any return statement returns a func that satisfies derive
	return c.funcLitReturnSatisfiesDerive(cctx, funcLit)
}

// funcLitReturnSatisfiesDerive checks if any return statement returns a func that satisfies derive.
func (c *Checker) funcLitReturnSatisfiesDerive(cctx *context.CheckContext, funcLit *ast.FuncLit) bool {
	var satisfies bool

	ast.Inspect(funcLit.Body, func(n ast.Node) bool {
		if satisfies {
			return false
		}
		// Skip nested func literals (they have their own returns)
		if fl, ok := n.(*ast.FuncLit); ok && fl != funcLit {
			return false
		}

		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		for _, result := range ret.Results {
			if c.returnedValueSatisfiesDerive(cctx, result) {
				satisfies = true

				return false
			}
		}

		return true
	})

	return satisfies
}

// returnedValueSatisfiesDerive checks if a returned value is a func that satisfies derive.
func (c *Checker) returnedValueSatisfiesDerive(cctx *context.CheckContext, result ast.Expr) bool {
	if innerFuncLit, ok := result.(*ast.FuncLit); ok {
		if context.HasContextOrCarrierParam(cctx.Pass, innerFuncLit.Type, cctx.Carriers) {
			return true
		}

		return c.derives.SatisfiesAnyGroup(cctx.Pass, innerFuncLit.Body)
	}

	ident, ok := result.(*ast.Ident)
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

	innerFuncLit := cctx.FindFuncLitAssignment(v)
	if innerFuncLit == nil {
		return false
	}

	if context.HasContextOrCarrierParam(cctx.Pass, innerFuncLit.Type, cctx.Carriers) {
		return true
	}

	return c.derives.SatisfiesAnyGroup(cctx.Pass, innerFuncLit.Body)
}
