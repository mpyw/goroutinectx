package probe

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ast/inspector"
)

// FuncLitAssignment represents a func literal assignment with its conditionality.
type FuncLitAssignment struct {
	Lit         *ast.FuncLit
	Conditional bool // true if inside if/for/switch/select
}

// FuncLitOfIdent is a convenience method that combines VarOf and FuncLitAssignedTo.
// Returns the last func literal assignment found.
func (c *Context) FuncLitOfIdent(ident *ast.Ident) *ast.FuncLit {
	v := c.VarOf(ident)
	if v == nil {
		return nil
	}
	return c.FuncLitAssignedTo(v, token.NoPos)
}

// FuncLitsOfIdent returns ALL func literals assigned to the identifier's variable.
// This is needed for conditional reassignment patterns where different branches
// assign different closures to the same variable.
func (c *Context) FuncLitsOfIdent(ident *ast.Ident) []*ast.FuncLit {
	v := c.VarOf(ident)
	if v == nil {
		return nil
	}
	return c.FuncLitsAssignedTo(v, token.NoPos)
}

// FuncLitAssignmentsOfIdent returns ALL func literal assignments with conditionality info.
func (c *Context) FuncLitAssignmentsOfIdent(ident *ast.Ident) []FuncLitAssignment {
	v := c.VarOf(ident)
	if v == nil {
		return nil
	}
	return c.FuncLitAssignmentsTo(v, token.NoPos)
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

// FuncLitsAssignedTo searches for ALL func literals assigned to the variable.
// If beforePos is token.NoPos, returns ALL assignments found.
// If beforePos is set, returns all assignments BEFORE that position.
// This is needed for conditional reassignment patterns.
func (c *Context) FuncLitsAssignedTo(v *types.Var, beforePos token.Pos) []*ast.FuncLit {
	f := c.FileOf(v.Pos())
	if f == nil {
		return nil
	}

	var results []*ast.FuncLit
	ast.Inspect(f, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		if beforePos != token.NoPos && assign.Pos() >= beforePos {
			return true
		}
		if fl := c.funcLitInAssignment(assign, v); fl != nil {
			results = append(results, fl)
		}
		return true
	})

	return results
}

// FuncLitAssignmentsTo searches for ALL func literal assignments with conditionality info.
func (c *Context) FuncLitAssignmentsTo(v *types.Var, beforePos token.Pos) []FuncLitAssignment {
	f := c.FileOf(v.Pos())
	if f == nil {
		return nil
	}

	var results []FuncLitAssignment
	insp := inspector.New([]*ast.File{f})

	insp.WithStack([]ast.Node{(*ast.AssignStmt)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}
		assign := n.(*ast.AssignStmt)
		if beforePos != token.NoPos && assign.Pos() >= beforePos {
			return true
		}
		fl := c.funcLitInAssignment(assign, v)
		if fl == nil {
			return true
		}

		// Check if assignment is inside a control structure
		conditional := isInControlStructure(stack)

		results = append(results, FuncLitAssignment{
			Lit:         fl,
			Conditional: conditional,
		})
		return true
	})

	return results
}

// isInControlStructure checks if the stack contains a control structure.
func isInControlStructure(stack []ast.Node) bool {
	for _, node := range stack {
		switch node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			return true
		}
	}
	return false
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
