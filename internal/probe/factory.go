package probe

import (
	"go/ast"
	"go/token"
	"go/types"
)

// BlockReturnsContextUsingFunc checks if a block's return statements
// return functions that use context.
func (c *Context) BlockReturnsContextUsingFunc(body *ast.BlockStmt, excludeFuncLit *ast.FuncLit) bool {
	if body == nil {
		return true
	}

	usesContext := false

	ast.Inspect(body, func(n ast.Node) bool {
		if usesContext {
			return false
		}
		if fl, ok := n.(*ast.FuncLit); ok && fl != excludeFuncLit {
			if c.FuncLitUsesContext(fl) {
				usesContext = true
				return false
			}
			if c.BlockReturnsContextUsingFunc(fl.Body, fl) {
				usesContext = true
				return false
			}
			return false
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
func (c *Context) returnedValueUsesContext(result ast.Expr) bool {
	if innerFuncLit, ok := result.(*ast.FuncLit); ok {
		return c.FuncLitUsesContext(innerFuncLit)
	}

	ident, ok := result.(*ast.Ident)
	if !ok {
		return false
	}

	innerFuncLit := c.FuncLitOfIdent(ident)
	if innerFuncLit == nil {
		return false
	}

	return c.FuncLitUsesContext(innerFuncLit)
}
