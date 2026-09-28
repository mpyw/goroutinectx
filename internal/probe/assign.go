package probe

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"iter"
	"slices"
	"strings"

	"golang.org/x/tools/go/ast/inspector"
)

// FuncLitAssignment represents a func literal assignment with its conditionality.
type FuncLitAssignment struct {
	Lit         *ast.FuncLit
	Conditional bool // true if inside if/for/switch/select
}

// EffectiveFuncLitAssignments returns the suffix of assigns that a check must
// consider: everything from the last unconditional assignment onwards.
//
// Assignments before that point are always overwritten, so they cannot affect
// the value observed at the spawn site. Conditional assignments after it may or
// may not run, so every one of them has to satisfy the check independently.
func EffectiveFuncLitAssignments(assigns []FuncLitAssignment) []FuncLitAssignment {
	for i, assign := range slices.Backward(assigns) {
		if !assign.Conditional {
			return assigns[i:]
		}
	}
	return assigns
}

// FuncLitAssignedToIdent is a convenience method that combines VarOf and FuncLitAssignedTo.
// Returns the last func literal assignment found.
func (c *Context) FuncLitAssignedToIdent(ident *ast.Ident) *ast.FuncLit {
	v := c.VarOf(ident)
	if v == nil {
		return nil
	}
	return c.FuncLitAssignedTo(v, token.NoPos)
}

// FuncLitsAssignedToIdent returns ALL func literals assigned to the identifier's variable.
// This is needed for conditional reassignment patterns where different branches
// assign different closures to the same variable.
func (c *Context) FuncLitsAssignedToIdent(ident *ast.Ident) []*ast.FuncLit {
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

// assignFileCursor finds the cursor of the file that contains pos, to search
// that file for assignments.
// It picks the same file as FileOf.
func (c *Context) assignFileCursor(pos token.Pos) (inspector.Cursor, bool) {
	for cur := range c.Inspector.Root().Children() {
		f := cur.Node()
		if f.Pos() <= pos && pos < f.End() {
			return cur, true
		}
	}
	return inspector.Cursor{}, false
}

// assignStmtsBefore yields each assignment in the file that declares v, with
// its cursor. If beforePos is set, it yields only those before that position.
func (c *Context) assignStmtsBefore(v *types.Var, beforePos token.Pos) iter.Seq2[inspector.Cursor, *ast.AssignStmt] {
	return func(yield func(inspector.Cursor, *ast.AssignStmt) bool) {
		file, ok := c.assignFileCursor(v.Pos())
		if !ok {
			return
		}
		for cur := range file.Preorder((*ast.AssignStmt)(nil)) {
			assign := cur.Node().(*ast.AssignStmt)
			if beforePos != token.NoPos && assign.Pos() >= beforePos {
				continue
			}
			if !yield(cur, assign) {
				return
			}
		}
	}
}

// assignValuesTo yields each value the assignment gives v: the right-hand side
// at the position of every left-hand identifier that denotes v.
func (c *Context) assignValuesTo(assign *ast.AssignStmt, v *types.Var) iter.Seq[ast.Expr] {
	return func(yield func(ast.Expr) bool) {
		for i, lhs := range assign.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if !ok || c.Pass.TypesInfo.ObjectOf(ident) != v || i >= len(assign.Rhs) {
				continue
			}
			if !yield(assign.Rhs[i]) {
				return
			}
		}
	}
}

// FuncLitAssignedTo searches for the func literal assigned to the variable.
// If beforePos is token.NoPos, returns the LAST assignment found.
// If beforePos is set, returns the last assignment BEFORE that position.
func (c *Context) FuncLitAssignedTo(v *types.Var, beforePos token.Pos) *ast.FuncLit {
	var result *ast.FuncLit
	for _, assign := range c.assignStmtsBefore(v, beforePos) {
		if fl := c.funcLitInAssignment(assign, v); fl != nil {
			result = fl
		}
	}

	return result
}

// FuncLitsAssignedTo searches for ALL func literals assigned to the variable.
// If beforePos is token.NoPos, returns ALL assignments found.
// If beforePos is set, returns all assignments BEFORE that position.
// This is needed for conditional reassignment patterns.
func (c *Context) FuncLitsAssignedTo(v *types.Var, beforePos token.Pos) []*ast.FuncLit {
	var results []*ast.FuncLit
	for _, assign := range c.assignStmtsBefore(v, beforePos) {
		if fl := c.funcLitInAssignment(assign, v); fl != nil {
			results = append(results, fl)
		}
	}

	return results
}

// FuncLitAssignmentsTo searches for ALL func literal assignments with conditionality info.
func (c *Context) FuncLitAssignmentsTo(v *types.Var, beforePos token.Pos) []FuncLitAssignment {
	var results []FuncLitAssignment
	for cur, assign := range c.assignStmtsBefore(v, beforePos) {
		fl := c.funcLitInAssignment(assign, v)
		if fl == nil {
			continue
		}

		// Check if assignment is inside a control structure
		conditional := assignedInControlStructure(cur)

		results = append(results, FuncLitAssignment{
			Lit:         fl,
			Conditional: conditional,
		})
	}

	return results
}

// assignedInControlStructure reports whether the assignment at cur sits inside
// a control structure.
func assignedInControlStructure(cur inspector.Cursor) bool {
	for range cur.Enclosing(
		(*ast.IfStmt)(nil),
		(*ast.ForStmt)(nil),
		(*ast.RangeStmt)(nil),
		(*ast.SwitchStmt)(nil),
		(*ast.TypeSwitchStmt)(nil),
		(*ast.SelectStmt)(nil),
	) {
		return true
	}
	return false
}

// funcLitInAssignment checks if the assignment assigns a func literal to v.
func (c *Context) funcLitInAssignment(assign *ast.AssignStmt, v *types.Var) *ast.FuncLit {
	for rhs := range c.assignValuesTo(assign, v) {
		if fl, ok := rhs.(*ast.FuncLit); ok {
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
	var result *ast.CallExpr
	for _, assign := range c.assignStmtsBefore(v, beforePos) {
		if call := c.callExprInAssignment(assign, v); call != nil {
			result = call
		}
	}

	return result
}

// callExprInAssignment checks if the assignment assigns a call expression to v.
func (c *Context) callExprInAssignment(assign *ast.AssignStmt, v *types.Var) *ast.CallExpr {
	for rhs := range c.assignValuesTo(assign, v) {
		if call, ok := rhs.(*ast.CallExpr); ok {
			return call
		}
	}
	return nil
}

// FuncLitAssignedToStructField finds a func literal assigned to a struct field.
func (c *Context) FuncLitAssignedToStructField(v *types.Var, fieldName string) *ast.FuncLit {
	for _, assign := range c.assignStmtsBefore(v, token.NoPos) {
		if result := c.funcLitOfFieldAssignment(assign, v, fieldName); result != nil {
			return result
		}
	}

	return nil
}

// FuncLitAssignedToIndex finds a func literal at a specific index in a composite literal.
func (c *Context) FuncLitAssignedToIndex(v *types.Var, indexExpr ast.Expr) *ast.FuncLit {
	for _, assign := range c.assignStmtsBefore(v, token.NoPos) {
		if result := c.funcLitOfIndexAssignment(assign, v, indexExpr); result != nil {
			return result
		}
	}

	return nil
}

// funcLitOfFieldAssignment extracts a func literal from a struct field assignment.
func (c *Context) funcLitOfFieldAssignment(assign *ast.AssignStmt, v *types.Var, fieldName string) *ast.FuncLit {
	for rhs := range c.assignValuesTo(assign, v) {
		compLit, ok := rhs.(*ast.CompositeLit)
		if !ok {
			continue
		}
		for _, elt := range compLit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != fieldName {
				continue
			}
			if fl, ok := kv.Value.(*ast.FuncLit); ok {
				return fl
			}
		}
	}
	return nil
}

// funcLitOfIndexAssignment extracts a func literal at a specific index from an assignment.
func (c *Context) funcLitOfIndexAssignment(assign *ast.AssignStmt, v *types.Var, indexExpr ast.Expr) *ast.FuncLit {
	for rhs := range c.assignValuesTo(assign, v) {
		compLit, ok := rhs.(*ast.CompositeLit)
		if !ok {
			continue
		}
		if lit, ok := indexExpr.(*ast.BasicLit); ok {
			return funcLitAssignedToLiteralKey(compLit, lit)
		}
	}
	return nil
}

// funcLitAssignedToLiteralKey extracts a func literal by literal index/key from a composite literal.
func funcLitAssignedToLiteralKey(compLit *ast.CompositeLit, lit *ast.BasicLit) *ast.FuncLit {
	switch lit.Kind {
	case token.INT:
		index := 0
		if _, err := fmt.Sscanf(lit.Value, "%d", &index); err != nil {
			return nil
		}
		if index < 0 || index >= len(compLit.Elts) {
			return nil
		}
		if fl, ok := compLit.Elts[index].(*ast.FuncLit); ok {
			return fl
		}

	case token.STRING:
		key := strings.Trim(lit.Value, `"`)
		for _, elt := range compLit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			keyLit, ok := kv.Key.(*ast.BasicLit)
			if !ok {
				continue
			}
			if strings.Trim(keyLit.Value, `"`) == key {
				if fl, ok := kv.Value.(*ast.FuncLit); ok {
					return fl
				}
			}
		}
	}

	return nil
}
