package probe

import (
	"go/ast"
	"slices"

	"github.com/mpyw/goroutinectx/internal/directive/carrier"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// FuncLitUsesContext checks if a function literal references any context variable.
// Does NOT descend into nested func literals.
func (c *Context) FuncLitUsesContext(lit *ast.FuncLit) bool {
	return c.nodeUsesContext(lit.Body, true)
}

// ArgUsesContext checks if an expression references a context variable.
// Unlike FuncLitUsesContext, this DOES descend into nested func literals.
func (c *Context) ArgUsesContext(expr ast.Expr) bool {
	return c.nodeUsesContext(expr, false)
}

// ArgsUseContext checks if any argument references a context variable.
func (c *Context) ArgsUseContext(args []ast.Expr) bool {
	return slices.ContainsFunc(args, c.ArgUsesContext)
}

// nodeUsesContext checks if a node references any context variable.
func (c *Context) nodeUsesContext(node ast.Node, skipNestedFuncLit bool) bool {
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
		if typeutil.IsContextType(obj.Type()) || carrier.IsCarrierType(obj.Type(), c.Carriers) {
			found = true
			return false
		}
		return true
	})
	return found
}
