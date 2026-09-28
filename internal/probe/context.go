package probe

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/mpyw/goroutinectx/internal/directive/carrier"
	"github.com/mpyw/goroutinectx/internal/ssa"
)

// Context provides context for pattern checking.
type Context struct {
	Pass      *analysis.Pass
	Inspector *inspector.Inspector // inspector over Pass.Files
	Tracer    *ssa.Tracer
	SSAProg   *ssa.Program
	CtxNames  []string
	Carriers  []carrier.Carrier
}

// VarOf extracts *types.Var from an identifier.
func (c *Context) VarOf(ident *ast.Ident) *types.Var {
	v, _ := c.Pass.TypesInfo.ObjectOf(ident).(*types.Var)
	return v
}

// FileOf finds the file that contains the given position.
func (c *Context) FileOf(pos token.Pos) *ast.File {
	for _, f := range c.Pass.Files {
		if f.Pos() <= pos && pos < f.End() {
			return f
		}
	}
	return nil
}

// FuncDeclOf finds the FuncDecl for a types.Func.
func (c *Context) FuncDeclOf(fn *types.Func) *ast.FuncDecl {
	pos := fn.Pos()
	f := c.FileOf(pos)
	if f == nil {
		return nil
	}
	for _, decl := range f.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			if funcDecl.Name.Pos() == pos {
				return funcDecl
			}
		}
	}
	return nil
}
