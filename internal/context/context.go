// Package context provides CheckContext for pattern checking.
package context

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/directives/carrier"
	internalssa "github.com/mpyw/goroutinectx/internal/ssa"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// CheckContext provides context for pattern checking.
type CheckContext struct {
	Pass    *analysis.Pass
	Tracer  *internalssa.Tracer
	SSAProg *internalssa.Program
	// CtxNames holds the context variable names from the enclosing scope (AST-based).
	// This is used when SSA-based context detection fails.
	CtxNames []string
	// Carriers holds the configured context carrier types.
	Carriers []carrier.Carrier
}

// Report reports a diagnostic at the given position.
func (c *CheckContext) Report(pos token.Pos, msg string) {
	c.Pass.Reportf(pos, "%s", msg)
}

// FuncTypeHasContextParam checks if a function type has a context.Context parameter.
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
func (c *CheckContext) FuncLitHasContextParam(lit *ast.FuncLit) bool {
	return c.FuncTypeHasContextParam(lit.Type)
}

// FuncLitUsesContext checks if a function literal references any context variable.
// It does NOT descend into nested func literals - they have their own scope and
// will be checked separately.
func (c *CheckContext) FuncLitUsesContext(lit *ast.FuncLit) bool {
	usesCtx := false
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		if usesCtx {
			return false
		}
		// Skip nested function literals - they will be checked separately
		if nested, ok := n.(*ast.FuncLit); ok && nested != lit {
			return false
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
			usesCtx = true
			return false
		}
		return true
	})
	return usesCtx
}

// ExtractCallFunc extracts the types.Func from a call expression.
func (c *CheckContext) ExtractCallFunc(call *ast.CallExpr) *types.Func {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		if f, ok := c.Pass.TypesInfo.ObjectOf(fun).(*types.Func); ok {
			return f
		}

	case *ast.SelectorExpr:
		sel := c.Pass.TypesInfo.Selections[fun]
		if sel != nil {
			if f, ok := sel.Obj().(*types.Func); ok {
				return f
			}
		} else {
			if f, ok := c.Pass.TypesInfo.ObjectOf(fun.Sel).(*types.Func); ok {
				return f
			}
		}
	}

	return nil
}

// ArgUsesContext checks if an expression references a context variable.
func (c *CheckContext) ArgUsesContext(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if found {
			return false
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

// FindFuncLitAssignment searches for the func literal assigned to the variable.
// If beforePos is token.NoPos, returns the LAST assignment found.
// If beforePos is set, returns the last assignment BEFORE that position.
func (c *CheckContext) FindFuncLitAssignment(v *types.Var, beforePos token.Pos) *ast.FuncLit {
	var result *ast.FuncLit
	declPos := v.Pos()

	for _, f := range c.Pass.Files {
		if f.Pos() > declPos || declPos >= f.End() {
			continue
		}

		ast.Inspect(f, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			// Skip assignments at or after beforePos
			if beforePos != token.NoPos && assign.Pos() >= beforePos {
				return true
			}
			if fl := c.findFuncLitInAssignment(assign, v); fl != nil {
				result = fl // Keep updating - we want the LAST assignment
			}
			return true
		})
		break
	}

	return result
}

// findFuncLitInAssignment checks if the assignment assigns a func literal to v.
func (c *CheckContext) findFuncLitInAssignment(assign *ast.AssignStmt, v *types.Var) *ast.FuncLit {
	for i, lhs := range assign.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}
		if c.Pass.TypesInfo.ObjectOf(ident) != v {
			continue
		}
		if i >= len(assign.Rhs) {
			continue
		}
		if fl, ok := assign.Rhs[i].(*ast.FuncLit); ok {
			return fl
		}
	}
	return nil
}

// BlockReturnsContextUsingFunc checks if a block's return statements
// return functions that use context. Recursively checks nested func literals.
// excludeFuncLit can be set to exclude a specific FuncLit from being counted (e.g., the parent).
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
func (c *CheckContext) FactoryReturnsContextUsingFunc(factory *ast.FuncLit) bool {
	return c.BlockReturnsContextUsingFunc(factory.Body, factory)
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

	obj := c.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return false
	}

	v, ok := obj.(*types.Var)
	if !ok {
		return false
	}

	innerFuncLit := c.FindFuncLitAssignment(v, token.NoPos)
	if innerFuncLit == nil {
		return false
	}

	return c.FuncLitUsesContext(innerFuncLit)
}
