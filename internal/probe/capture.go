package probe

import (
	"go/ast"
)

// SelectorExprCapturesContext checks if a struct field func captures context.
func (c *Context) SelectorExprCapturesContext(sel *ast.SelectorExpr) bool {
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return true
	}

	v := c.VarOf(ident)
	if v == nil {
		return true
	}

	fieldName := sel.Sel.Name
	funcLit := c.FuncLitAssignedToStructField(v, fieldName)
	if funcLit == nil {
		return true
	}

	return c.FuncLitUsesContext(funcLit)
}

// IndexExprCapturesContext checks if a slice/map indexed func captures context.
func (c *Context) IndexExprCapturesContext(idx *ast.IndexExpr) bool {
	ident, ok := idx.X.(*ast.Ident)
	if !ok {
		return true
	}

	v := c.VarOf(ident)
	if v == nil {
		return true
	}

	funcLit := c.FuncLitAssignedToIndex(v, idx.Index)
	if funcLit == nil {
		return true
	}

	return c.FuncLitUsesContext(funcLit)
}
