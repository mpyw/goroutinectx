package patterns

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/mpyw/goroutinectx/internal/context"
)

// ClosureCapturesCtx checks that a closure captures the outer context.
// Used by: errgroup.Group.Go, sync.WaitGroup.Go, sourcegraph/conc.Pool.Go, etc.
type ClosureCapturesCtx struct{}

func (*ClosureCapturesCtx) Name() string {
	return "ClosureCapturesCtx"
}

func (*ClosureCapturesCtx) Check(cctx *context.CheckContext, call *ast.CallExpr, callbackArg ast.Expr) bool {
	// If no context names in scope (from AST), nothing to check
	if len(cctx.CtxNames) == 0 {
		return true
	}

	// Try SSA-based check first (more accurate, includes nested closures)
	if lit, ok := callbackArg.(*ast.FuncLit); ok {
		if result, ok := cctx.FuncLitCapturesContextSSA(lit); ok {
			return result
		}
	}

	// Fall back to AST-based check when SSA fails
	return closureCheckFromAST(cctx, callbackArg)
}

func (*ClosureCapturesCtx) Message(apiName string, ctxName string) string {
	return fmt.Sprintf("%s() closure should use context %q", apiName, ctxName)
}

// closureCheckFromAST falls back to AST-based analysis when SSA tracing fails.
// Design principle: "prove safety" - if we can't prove ctx is used, report error.
func closureCheckFromAST(cctx *context.CheckContext, callbackArg ast.Expr) bool {
	// For function literals, check if they reference context
	if lit, ok := callbackArg.(*ast.FuncLit); ok {
		return cctx.FuncLitCapturesContext(lit)
	}

	// For identifiers, try to find the function literal assignment
	if ident, ok := callbackArg.(*ast.Ident); ok {
		funcLit := cctx.FindIdentFuncLitAssignment(ident, token.NoPos)
		if funcLit == nil {
			return false // Can't trace
		}
		return cctx.FuncLitCapturesContext(funcLit)
	}

	// For call expressions, check if ctx is passed as argument
	if call, ok := callbackArg.(*ast.CallExpr); ok {
		return closureCheckFactoryCall(cctx, call)
	}

	// For selector expressions (struct field access), check the field's func
	if sel, ok := callbackArg.(*ast.SelectorExpr); ok {
		return closureCheckSelectorFunc(cctx, sel)
	}

	// For index expressions (slice/map access), check the indexed func
	if idx, ok := callbackArg.(*ast.IndexExpr); ok {
		return closureCheckIndexFunc(cctx, idx)
	}

	return false // Can't analyze - report error to catch potential issues
}

// closureCheckSelectorFunc checks if a struct field func uses context.
func closureCheckSelectorFunc(cctx *context.CheckContext, sel *ast.SelectorExpr) bool {
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	v := cctx.VarFromIdent(ident)
	if v == nil {
		return false
	}

	fieldName := sel.Sel.Name
	funcLit := closureFindStructFieldFuncLit(cctx, v, fieldName)
	if funcLit == nil {
		return false
	}

	return cctx.FuncLitUsesContext(funcLit)
}

// closureFindStructFieldFuncLit finds a func literal assigned to a struct field.
func closureFindStructFieldFuncLit(cctx *context.CheckContext, v *types.Var, fieldName string) *ast.FuncLit {
	f := cctx.FindFileContaining(v.Pos())
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
		result = closureFindFieldInAssignment(cctx, assign, v, fieldName)
		return result == nil
	})

	return result
}

// closureFindFieldInAssignment looks for a func literal in a struct field assignment.
func closureFindFieldInAssignment(cctx *context.CheckContext, assign *ast.AssignStmt, v *types.Var, fieldName string) *ast.FuncLit {
	for i, lhs := range assign.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}
		if cctx.Pass.TypesInfo.ObjectOf(ident) != v {
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

// closureCheckIndexFunc checks if a slice/map indexed func uses context.
func closureCheckIndexFunc(cctx *context.CheckContext, idx *ast.IndexExpr) bool {
	ident, ok := idx.X.(*ast.Ident)
	if !ok {
		return false
	}

	v := cctx.VarFromIdent(ident)
	if v == nil {
		return false
	}

	funcLit := closureFindIndexedFuncLit(cctx, v, idx.Index)
	if funcLit == nil {
		return false
	}

	return cctx.FuncLitUsesContext(funcLit)
}

// closureFindIndexedFuncLit finds a func literal at a specific index in a composite literal.
func closureFindIndexedFuncLit(cctx *context.CheckContext, v *types.Var, indexExpr ast.Expr) *ast.FuncLit {
	f := cctx.FindFileContaining(v.Pos())
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
		result = closureFindFuncLitAtIndex(cctx, assign, v, indexExpr)
		return result == nil
	})

	return result
}

// closureFindFuncLitAtIndex looks for a func literal at a specific index.
func closureFindFuncLitAtIndex(cctx *context.CheckContext, assign *ast.AssignStmt, v *types.Var, indexExpr ast.Expr) *ast.FuncLit {
	for i, lhs := range assign.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}
		if cctx.Pass.TypesInfo.ObjectOf(ident) != v {
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
			return closureFindFuncLitByLiteral(compLit, lit)
		}
	}
	return nil
}

// closureFindFuncLitByLiteral finds func literal by literal index/key.
func closureFindFuncLitByLiteral(compLit *ast.CompositeLit, lit *ast.BasicLit) *ast.FuncLit {
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

// closureCheckFactoryCall checks if a factory call passes context.
// Handles patterns like:
//   - g.Go(makeWorkerWithCtx(ctx)) - ctx passed as argument
//   - g.Go(makeWorker()) where makeWorker is a closure that captures ctx
func closureCheckFactoryCall(cctx *context.CheckContext, call *ast.CallExpr) bool {
	// Check if ctx is passed as an argument to the call
	for _, arg := range call.Args {
		if cctx.ArgUsesContext(arg) {
			return true
		}
	}

	// Check if the factory function itself is a closure that captures ctx
	// and returns a function that uses it
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		// e.g., makeWorker() where makeWorker := func() func() error { ... }
		funcLit := cctx.FindIdentFuncLitAssignment(fun, token.NoPos)
		if funcLit == nil {
			return false
		}
		// Check if the factory's return statements return a func that uses ctx
		return cctx.FactoryReturnsContextUsingFunc(funcLit)

	case *ast.FuncLit:
		// e.g., g.Go((func() func() error { return func() error { _ = ctx; return nil } })())
		return cctx.FactoryReturnsContextUsingFunc(fun)
	}

	return false
}
