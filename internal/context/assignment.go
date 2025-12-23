package context

import (
	"go/ast"
	"go/token"
	"go/types"
)

// FuncLitOfIdent is a convenience method that combines VarOf and FuncLitAssignedTo.
// Returns nil if the identifier doesn't refer to a variable or no func literal assignment is found.
//
// Example:
//
//	fn := func() { doWork(ctx) }
//	g.Go(fn)  // fn is an identifier
//	// FuncLitOfIdent(ast_of_fn, pos_of_g.Go) returns the func literal
func (c *CheckContext) FuncLitOfIdent(ident *ast.Ident, beforePos token.Pos) *ast.FuncLit {
	v := c.VarOf(ident)
	if v == nil {
		return nil
	}
	return c.FuncLitAssignedTo(v, beforePos)
}

// FuncLitAssignedTo searches for the func literal assigned to the variable.
// If beforePos is token.NoPos, returns the LAST assignment found.
// If beforePos is set, returns the last assignment BEFORE that position.
//
// Example:
//
//	fn := func() { doWork(ctx) }  // <-- returns this FuncLit
//	g.Go(fn)
//	// FuncLitAssignedTo(v_of_fn, pos_of_g.Go) returns the func literal
//
// Example (multiple assignments):
//
//	fn := func() { doA(ctx) }  // first assignment
//	fn = func() { doB(ctx) }   // second assignment  <-- returns this one
//	g.Go(fn)
func (c *CheckContext) FuncLitAssignedTo(v *types.Var, beforePos token.Pos) *ast.FuncLit {
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
		// Skip assignments at or after beforePos
		if beforePos != token.NoPos && assign.Pos() >= beforePos {
			return true
		}
		if fl := c.funcLitInAssignment(assign, v); fl != nil {
			result = fl // Keep updating - we want the LAST assignment
		}
		return true
	})

	return result
}

// funcLitInAssignment checks if the assignment assigns a func literal to v.
func (c *CheckContext) funcLitInAssignment(assign *ast.AssignStmt, v *types.Var) *ast.FuncLit {
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

// CallExprAssignedTo searches for the call expression assigned to the variable.
// If beforePos is token.NoPos, returns the LAST assignment found.
// If beforePos is set, returns the last assignment BEFORE that position.
//
// Example:
//
//	task := gotask.NewTask(fn)  // <-- returns this CallExpr
//	gotask.DoAll(ctx, task)
//	// CallExprAssignedTo(v_of_task, pos_of_DoAll) returns gotask.NewTask(fn)
func (c *CheckContext) CallExprAssignedTo(v *types.Var, beforePos token.Pos) *ast.CallExpr {
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
		// Skip assignments at or after beforePos
		if beforePos != token.NoPos && assign.Pos() >= beforePos {
			return true
		}
		if call := c.callExprInAssignment(assign, v); call != nil {
			result = call // Keep updating - we want the LAST assignment
		}
		return true
	})

	return result
}

// callExprInAssignment checks if the assignment assigns a call expression to v.
func (c *CheckContext) callExprInAssignment(assign *ast.AssignStmt, v *types.Var) *ast.CallExpr {
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
