package probe

import (
	"go/ast"

	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// FuncTypeHasContextParam checks if a function type has a context.Context parameter.
func (c *Context) FuncTypeHasContextParam(fnType *ast.FuncType) bool {
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
func (c *Context) FuncLitHasContextParam(lit *ast.FuncLit) bool {
	return c.FuncTypeHasContextParam(lit.Type)
}
