package probe

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
)

// BlockReturnsContextUsingFunc checks if a block's return statements
// return functions that use context.
// Only checks what's actually returned, not all nested func literals.
func (c *Context) BlockReturnsContextUsingFunc(body *ast.BlockStmt, excludeFuncLit *ast.FuncLit) bool {
	if body == nil {
		return true
	}

	usesContext := false

	ast.Inspect(body, func(n ast.Node) bool {
		if usesContext {
			return false
		}
		// Skip descending into nested func literals (except excludeFuncLit which is the factory itself)
		if fl, ok := n.(*ast.FuncLit); ok && fl != excludeFuncLit {
			return false
		}

		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		if slices.ContainsFunc(ret.Results, c.returnedValueUsesContext) {
			usesContext = true
			return false
		}
		return true
	})

	return usesContext
}

// FactoryReturnsContextUsingFunc checks if a factory FuncLit's return statements
// return functions that use context.
func (c *Context) FactoryReturnsContextUsingFunc(factory *ast.FuncLit) bool {
	return c.BlockReturnsContextUsingFunc(factory.Body, factory)
}

// FactoryCallReturnsContextUsingFunc checks if a factory call returns a context-using func.
func (c *Context) FactoryCallReturnsContextUsingFunc(call *ast.CallExpr) bool {
	if c.ArgsUseContext(call.Args) {
		return true
	}

	switch fun := call.Fun.(type) {
	case *ast.FuncLit:
		if c.FuncLitHasContextParam(fun) {
			return true
		}
		return c.FactoryReturnsContextUsingFunc(fun)

	case *ast.Ident:
		return c.IdentFactoryReturnsContextUsingFunc(fun)

	case *ast.CallExpr:
		return c.FactoryCallReturnsContextUsingFunc(fun)
	}

	return true // Can't analyze, assume OK
}

// IdentFactoryReturnsContextUsingFunc checks if an identifier refers to a factory
// that returns a context-using func.
func (c *Context) IdentFactoryReturnsContextUsingFunc(ident *ast.Ident) bool {
	obj := c.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return true
	}

	if v := c.VarOf(ident); v != nil {
		funcLit := c.FuncLitAssignedTo(v, token.NoPos)
		if funcLit == nil {
			return true
		}
		if c.FuncLitHasContextParam(funcLit) {
			return true
		}
		return c.FactoryReturnsContextUsingFunc(funcLit)
	}

	if fn, ok := obj.(*types.Func); ok {
		funcDecl := c.FuncDeclOf(fn)
		if funcDecl == nil {
			return true
		}
		if c.FuncTypeHasContextParam(funcDecl.Type) {
			return true
		}
		return c.BlockReturnsContextUsingFunc(funcDecl.Body, nil)
	}

	return true
}

// returnedValueUsesContext checks if a returned value is a func that uses context.
// For identifiers, checks ALL assignments from last unconditional onwards.
func (c *Context) returnedValueUsesContext(result ast.Expr) bool {
	if innerFuncLit, ok := result.(*ast.FuncLit); ok {
		// Check if the func lit directly uses context OR returns a context-using func
		return c.FuncLitUsesContext(innerFuncLit) || c.BlockReturnsContextUsingFunc(innerFuncLit.Body, innerFuncLit)
	}

	ident, ok := result.(*ast.Ident)
	if !ok {
		return false
	}

	assigns := c.FuncLitAssignmentsOfIdent(ident)
	if len(assigns) == 0 {
		return false
	}

	return c.funcLitAssignmentsAllUseOrReturnContext(assigns)
}

// funcLitAssignmentsAllUseOrReturnContext checks if ALL func literal assignments from
// last unconditional onwards use context OR return a context-using func.
func (c *Context) funcLitAssignmentsAllUseOrReturnContext(assigns []FuncLitAssignment) bool {
	// ALL must use context OR return context-using func
	for _, assign := range EffectiveFuncLitAssignments(assigns) {
		lit := assign.Lit
		// Check if the func lit directly uses context OR returns a context-using func
		if !c.FuncLitUsesContext(lit) && !c.BlockReturnsContextUsingFunc(lit.Body, lit) {
			return false
		}
	}
	return true
}
