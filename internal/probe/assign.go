package probe

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"iter"
	"strings"

	"golang.org/x/tools/go/ast/inspector"
)

// FuncLitAssignedToStructField finds a func literal assigned to a struct field.
func (c *Context) FuncLitAssignedToStructField(v *types.Var, fieldName string) *ast.FuncLit {
	for assign := range c.assignStmts(v) {
		if result := c.funcLitOfFieldAssignment(assign, v, fieldName); result != nil {
			return result
		}
	}

	return nil
}

// FuncLitAssignedToIndex finds a func literal at a specific index in a composite literal.
func (c *Context) FuncLitAssignedToIndex(v *types.Var, indexExpr ast.Expr) *ast.FuncLit {
	for assign := range c.assignStmts(v) {
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

// assignStmts yields each assignment in the file that declares v.
func (c *Context) assignStmts(v *types.Var) iter.Seq[*ast.AssignStmt] {
	return func(yield func(*ast.AssignStmt) bool) {
		file, ok := c.assignFileCursor(v.Pos())
		if !ok {
			return
		}
		for cur := range file.Preorder((*ast.AssignStmt)(nil)) {
			if !yield(cur.Node().(*ast.AssignStmt)) {
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
