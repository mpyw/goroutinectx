package check

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// SelectorExprCapturesContext checks if a struct field func captures context.
func (c *Context) SelectorExprCapturesContext(sel *ast.SelectorExpr) bool {
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return true
	}

	v := c.VarOf(ident)
	if v == nil {
		return true
	}

	fieldName := sel.Sel.Name
	funcLit := c.FuncLitOfStructField(v, fieldName)
	if funcLit == nil {
		return true
	}

	return c.FuncLitUsesContext(funcLit)
}

// IndexExprCapturesContext checks if a slice/map indexed func captures context.
func (c *Context) IndexExprCapturesContext(idx *ast.IndexExpr) bool {
	ident, ok := idx.X.(*ast.Ident)
	if !ok {
		return true
	}

	v := c.VarOf(ident)
	if v == nil {
		return true
	}

	funcLit := c.FuncLitOfIndex(v, idx.Index)
	if funcLit == nil {
		return true
	}

	return c.FuncLitUsesContext(funcLit)
}

// FuncLitOfStructField finds a func literal assigned to a struct field.
func (c *Context) FuncLitOfStructField(v *types.Var, fieldName string) *ast.FuncLit {
	f := c.FileOf(v.Pos())
	if f == nil {
		return nil
	}

	var result *ast.FuncLit
	ast.Inspect(f, func(n ast.Node) bool {
		if result != nil {
			return false
		}
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		result = c.funcLitOfFieldAssignment(assign, v, fieldName)
		return result == nil
	})

	return result
}

// FuncLitOfIndex finds a func literal at a specific index in a composite literal.
func (c *Context) FuncLitOfIndex(v *types.Var, indexExpr ast.Expr) *ast.FuncLit {
	f := c.FileOf(v.Pos())
	if f == nil {
		return nil
	}

	var result *ast.FuncLit
	ast.Inspect(f, func(n ast.Node) bool {
		if result != nil {
			return false
		}
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		result = c.funcLitOfIndexAssignment(assign, v, indexExpr)
		return result == nil
	})

	return result
}

// funcLitOfFieldAssignment extracts a func literal from a struct field assignment.
func (c *Context) funcLitOfFieldAssignment(assign *ast.AssignStmt, v *types.Var, fieldName string) *ast.FuncLit {
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
		compLit, ok := assign.Rhs[i].(*ast.CompositeLit)
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
		compLit, ok := assign.Rhs[i].(*ast.CompositeLit)
		if !ok {
			continue
		}
		if lit, ok := indexExpr.(*ast.BasicLit); ok {
			return funcLitOfLiteralKey(compLit, lit)
		}
	}
	return nil
}

// funcLitOfLiteralKey extracts a func literal by literal index/key from a composite literal.
func funcLitOfLiteralKey(compLit *ast.CompositeLit, lit *ast.BasicLit) *ast.FuncLit {
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
