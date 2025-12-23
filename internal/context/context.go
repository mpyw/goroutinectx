// Package context provides CheckContext for pattern checking.
package context

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/directives/carrier"
	"github.com/mpyw/goroutinectx/internal/funcspec"
	internalssa "github.com/mpyw/goroutinectx/internal/ssa"
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

// VarOf extracts *types.Var from an identifier.
// Returns nil if the identifier doesn't refer to a variable.
func (c *CheckContext) VarOf(ident *ast.Ident) *types.Var {
	obj := c.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return nil
	}
	v, ok := obj.(*types.Var)
	if !ok {
		return nil
	}
	return v
}

// FileOf finds the file that contains the given position.
// Returns nil if no file contains the position.
func (c *CheckContext) FileOf(pos token.Pos) *ast.File {
	for _, f := range c.Pass.Files {
		if f.Pos() <= pos && pos < f.End() {
			return f
		}
	}
	return nil
}

// FuncDeclOf finds the FuncDecl for a types.Func.
// Returns nil if the function declaration is not found in the analyzed files.
func (c *CheckContext) FuncDeclOf(fn *types.Func) *ast.FuncDecl {
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

// FuncOf extracts the types.Func from a call expression.
func (c *CheckContext) FuncOf(call *ast.CallExpr) *types.Func {
	return funcspec.ExtractFunc(c.Pass, call)
}
