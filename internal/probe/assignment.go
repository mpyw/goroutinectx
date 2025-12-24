package probe

import (
	"go/ast"
	"go/token"
	"go/types"
)

// FuncLitOfIdent is a convenience method that combines VarOf and FuncLitAssignedTo.
// Returns the last func literal assignment found.
func (c *Context) FuncLitOfIdent(ident *ast.Ident) *ast.FuncLit {
	v := c.VarOf(ident)
	if v == nil {
		return nil
	}
	return c.FuncLitAssignedTo(v, token.NoPos)
}

// FuncLitAssignedTo searches for the func literal assigned to the variable.
// If beforePos is token.NoPos, returns the LAST assignment found.
// If beforePos is set, returns the last assignment BEFORE that position.
func (c *Context) FuncLitAssignedTo(v *types.Var, beforePos token.Pos) *ast.FuncLit {
	f := c.FileOf(v.Pos())
	if f == nil {
		return nil
	}

	var result *ast.FuncLit
	ast.Inspect(f, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		if beforePos != token.NoPos && assign.Pos() >= beforePos {
			return true
		}
		if fl := c.funcLitInAssignment(assign, v); fl != nil {
			result = fl
		}
		return true
	})

	return result
}

// funcLitInAssignment checks if the assignment assigns a func literal to v.
func (c *Context) funcLitInAssignment(assign *ast.AssignStmt, v *types.Var) *ast.FuncLit {
	for i, lhs := range assign.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}
		if c.Pass.TypesInfo.ObjectOf(ident) != v {
			continue
		}
		if i >= len(assign.Rhs) {
			continue
		}
		if fl, ok := assign.Rhs[i].(*ast.FuncLit); ok {
			return fl
		}
	}
	return nil
}

// CallExprAssignedToIdent is a convenience method that combines VarOf and CallExprAssignedTo.
// Returns the last call expression assignment found.
func (c *Context) CallExprAssignedToIdent(ident *ast.Ident) *ast.CallExpr {
	v := c.VarOf(ident)
	if v == nil {
		return nil
	}
	return c.CallExprAssignedTo(v, token.NoPos)
}

// CallExprAssignedTo searches for the call expression assigned to the variable.
func (c *Context) CallExprAssignedTo(v *types.Var, beforePos token.Pos) *ast.CallExpr {
	f := c.FileOf(v.Pos())
	if f == nil {
		return nil
	}

	var result *ast.CallExpr
	ast.Inspect(f, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		if beforePos != token.NoPos && assign.Pos() >= beforePos {
			return true
		}
		if call := c.callExprInAssignment(assign, v); call != nil {
			result = call
		}
		return true
	})

	return result
}

// callExprInAssignment checks if the assignment assigns a call expression to v.
func (c *Context) callExprInAssignment(assign *ast.AssignStmt, v *types.Var) *ast.CallExpr {
	for i, lhs := range assign.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}
		if c.Pass.TypesInfo.ObjectOf(ident) != v {
			continue
		}
		if i >= len(assign.Rhs) {
			continue
		}
		if call, ok := assign.Rhs[i].(*ast.CallExpr); ok {
			return call
		}
	}
	return nil
}
