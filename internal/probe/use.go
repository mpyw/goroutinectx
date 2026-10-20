package probe

import (
	"go/ast"

	"github.com/mpyw/goroutinectx/internal/carrier"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// FuncLitUsesContext checks if a function literal references any context variable.
// Does NOT descend into nested func literals.
func (c *Context) FuncLitUsesContext(lit *ast.FuncLit) bool {
	return c.nodeUsesContext(lit.Body)
}

// nodeUsesContext checks if a node references any context variable, outside
// nested func literals.
func (c *Context) nodeUsesContext(node ast.Node) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		if _, ok := n.(*ast.FuncLit); ok {
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
		if typeutil.IsContextType(obj.Type()) || carrier.IsCarrierType(obj.Type(), c.Carriers) {
			found = true
			return false
		}
		return true
	})
	return found
}
