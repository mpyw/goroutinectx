package probe

import (
	"go/ast"
	"slices"

	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// FuncTypeHasContextParam checks if a function type has a context.Context parameter.
func (c *Context) FuncTypeHasContextParam(fnType *ast.FuncType) bool {
	if fnType == nil || fnType.Params == nil {
		return false
	}
	return slices.ContainsFunc(fnType.Params.List, func(field *ast.Field) bool {
		typ := c.Pass.TypesInfo.TypeOf(field.Type)
		return typ != nil && typeutil.IsContextType(typ)
	})
}

// FuncLitHasContextParam checks if a function literal has a context.Context parameter.
func (c *Context) FuncLitHasContextParam(lit *ast.FuncLit) bool {
	return c.FuncTypeHasContextParam(lit.Type)
}
