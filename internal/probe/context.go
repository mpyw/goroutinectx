package probe

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/mpyw/goroutinectx/internal/carrier"
	"github.com/mpyw/goroutinectx/internal/ssa"
)

// Context provides context for pattern checking.
type Context struct {
	Pass      *analysis.Pass
	Inspector *inspector.Inspector // inspector over Pass.Files
	SSAProg   *ssa.Program
	CtxNames  []string
	Carriers  []carrier.Carrier
}

// VarOf extracts *types.Var from an identifier.
func (c *Context) VarOf(ident *ast.Ident) *types.Var {
	v, _ := c.Pass.TypesInfo.ObjectOf(ident).(*types.Var)
	return v
}
