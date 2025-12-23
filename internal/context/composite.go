package context

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// SelectorExprCapturesContext checks if a struct field func captures context.
// Handles patterns like: s.handler where s is a struct with a func field.
//
// Example (struct field captures context - returns true):
//
//	func example(ctx context.Context) {
//	    s := struct{ handler func() }{
//	        handler: func() { doWork(ctx) },
//	    }
//	    go s.handler()  // s.handler captures ctx
//	}
//
// Example (struct field doesn't capture - returns false):
//
//	func example(ctx context.Context) {
//	    s := struct{ handler func() }{
//	        handler: func() { doWork() },  // no ctx
//	    }
//	    go s.handler()
//	}
func (c *CheckContext) SelectorExprCapturesContext(sel *ast.SelectorExpr) bool {
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return true // Can't trace, assume OK
	}

	v := c.VarOf(ident)
	if v == nil {
		return true // Can't trace, assume OK
	}

	fieldName := sel.Sel.Name
	funcLit := c.FuncLitOfStructField(v, fieldName)
	if funcLit == nil {
		return true // Can't trace, assume OK
	}

	return c.FuncLitUsesContext(funcLit)
}

// IndexExprCapturesContext checks if a slice/map indexed func captures context.
// Handles patterns like: handlers[0] or handlers["key"].
//
// Example (slice element captures context - returns true):
//
//	func example(ctx context.Context) {
//	    handlers := []func(){
//	        func() { doWork(ctx) },
//	    }
//	    go handlers[0]()  // handlers[0] captures ctx
//	}
//
// Example (map value captures context - returns true):
//
//	func example(ctx context.Context) {
//	    handlers := map[string]func(){
//	        "work": func() { doWork(ctx) },
//	    }
//	    go handlers["work"]()  // handlers["work"] captures ctx
//	}
//
// Example (no context capture - returns false):
//
//	func example(ctx context.Context) {
//	    handlers := []func(){
//	        func() { doWork() },  // no ctx
//	    }
//	    go handlers[0]()
//	}
func (c *CheckContext) IndexExprCapturesContext(idx *ast.IndexExpr) bool {
	ident, ok := idx.X.(*ast.Ident)
	if !ok {
		return true // Can't trace, assume OK
	}

	v := c.VarOf(ident)
	if v == nil {
		return true // Can't trace, assume OK
	}

	funcLit := c.FuncLitOfIndex(v, idx.Index)
	if funcLit == nil {
		return true // Can't trace, assume OK
	}

	return c.FuncLitUsesContext(funcLit)
}

// FuncLitOfStructField finds a func literal assigned to a struct field.
//
// Example:
//
//	s := struct{ handler func() }{
//	    handler: func() { ... },  // <-- returns this FuncLit
//	}
//	// FuncLitOfStructField(v_of_s, "handler") returns the func literal
func (c *CheckContext) FuncLitOfStructField(v *types.Var, fieldName string) *ast.FuncLit {
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
//
// Example (slice index):
//
//	handlers := []func(){
//	    func() { ... },  // index 0
//	    func() { ... },  // index 1
//	}
//	// FuncLitOfIndex(v_of_handlers, ast_of_0) returns the first func literal
//
// Example (map key):
//
//	handlers := map[string]func(){
//	    "work": func() { ... },
//	}
//	// FuncLitOfIndex(v_of_handlers, ast_of_"work") returns the func literal
func (c *CheckContext) FuncLitOfIndex(v *types.Var, indexExpr ast.Expr) *ast.FuncLit {
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
func (c *CheckContext) funcLitOfFieldAssignment(assign *ast.AssignStmt, v *types.Var, fieldName string) *ast.FuncLit {
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
func (c *CheckContext) funcLitOfIndexAssignment(assign *ast.AssignStmt, v *types.Var, indexExpr ast.Expr) *ast.FuncLit {
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
			return c.funcLitOfLiteralKey(compLit, lit)
		}
	}
	return nil
}

// funcLitOfLiteralKey extracts a func literal by literal index/key from a composite literal.
func (*CheckContext) funcLitOfLiteralKey(compLit *ast.CompositeLit, lit *ast.BasicLit) *ast.FuncLit {
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
