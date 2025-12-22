package patterns

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// ClosureCapturesCtx checks that a closure captures the outer context.
// Used by: errgroup.Group.Go, sync.WaitGroup.Go, sourcegraph/conc.Pool.Go, etc.
type ClosureCapturesCtx struct{}

func (*ClosureCapturesCtx) Name() string {
	return "ClosureCapturesCtx"
}

func (*ClosureCapturesCtx) Check(cctx *CheckContext, call *ast.CallExpr, callbackArg ast.Expr) bool {
	// If no context names in scope (from AST), nothing to check
	if len(cctx.CtxNames) == 0 {
		return true
	}

	// Try SSA-based check first (more accurate, includes nested closures)
	if result, ok := closureCheckFromSSA(cctx, callbackArg); ok {
		return result
	}

	// Fall back to AST-based check when SSA fails
	return closureCheckFromAST(cctx, callbackArg)
}

func (*ClosureCapturesCtx) Message(apiName string, ctxName string) string {
	return fmt.Sprintf("%s() closure should use context %q", apiName, ctxName)
}

// closureCheckFromSSA uses SSA analysis to check if a closure captures context.
// Returns (result, true) if SSA analysis succeeded, or (false, false) if it failed.
func closureCheckFromSSA(cctx *CheckContext, callbackArg ast.Expr) (bool, bool) {
	if cctx.SSAProg == nil || cctx.Tracer == nil {
		return false, false
	}

	// For function literals, find the SSA function and check FreeVars
	if lit, ok := callbackArg.(*ast.FuncLit); ok {
		// Skip if closure has its own context parameter
		if cctx.funcLitHasContextParam(lit) {
			return true, true
		}

		ssaFn := cctx.SSAProg.FindFuncLit(lit)
		if ssaFn == nil {
			return false, false // SSA lookup failed
		}

		return cctx.Tracer.ClosureCapturesContext(ssaFn, cctx.Carriers), true
	}

	// For other cases, fall back to AST
	return false, false
}

// closureCheckFromAST falls back to AST-based analysis when SSA tracing fails.
// Design principle: "prove safety" - if we can't prove ctx is used, report error.
func closureCheckFromAST(cctx *CheckContext, callbackArg ast.Expr) bool {
	// For function literals, check if they reference context
	if lit, ok := callbackArg.(*ast.FuncLit); ok {
		// Skip if closure has its own context parameter
		if cctx.funcLitHasContextParam(lit) {
			return true
		}
		return cctx.FuncLitUsesContext(lit)
	}

	// For identifiers, try to find the function literal assignment
	if ident, ok := callbackArg.(*ast.Ident); ok {
		obj := cctx.Pass.TypesInfo.ObjectOf(ident)
		if obj == nil {
			return false // Can't trace
		}
		v, ok := obj.(*types.Var)
		if !ok {
			return false // Can't trace
		}
		funcLit := cctx.FindFuncLitAssignment(v, token.NoPos)
		if funcLit == nil {
			return false // Can't trace (channel receive, type assertion, etc.)
		}
		// Skip if closure has its own context parameter
		if cctx.funcLitHasContextParam(funcLit) {
			return true
		}
		return cctx.FuncLitUsesContext(funcLit)
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
func closureCheckSelectorFunc(cctx *CheckContext, sel *ast.SelectorExpr) bool {
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	obj := cctx.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return false
	}

	v, ok := obj.(*types.Var)
	if !ok {
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
func closureFindStructFieldFuncLit(cctx *CheckContext, v *types.Var, fieldName string) *ast.FuncLit {
	var result *ast.FuncLit
	pos := v.Pos()

	for _, f := range cctx.Pass.Files {
		if f.Pos() > pos || pos >= f.End() {
			continue
		}

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
		break
	}

	return result
}

// closureFindFieldInAssignment looks for a func literal in a struct field assignment.
func closureFindFieldInAssignment(cctx *CheckContext, assign *ast.AssignStmt, v *types.Var, fieldName string) *ast.FuncLit {
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
func closureCheckIndexFunc(cctx *CheckContext, idx *ast.IndexExpr) bool {
	ident, ok := idx.X.(*ast.Ident)
	if !ok {
		return false
	}

	obj := cctx.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return false
	}

	v, ok := obj.(*types.Var)
	if !ok {
		return false
	}

	funcLit := closureFindIndexedFuncLit(cctx, v, idx.Index)
	if funcLit == nil {
		return false
	}

	return cctx.FuncLitUsesContext(funcLit)
}

// closureFindIndexedFuncLit finds a func literal at a specific index in a composite literal.
func closureFindIndexedFuncLit(cctx *CheckContext, v *types.Var, indexExpr ast.Expr) *ast.FuncLit {
	var result *ast.FuncLit
	pos := v.Pos()

	for _, f := range cctx.Pass.Files {
		if f.Pos() > pos || pos >= f.End() {
			continue
		}

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
		break
	}

	return result
}

// closureFindFuncLitAtIndex looks for a func literal at a specific index.
func closureFindFuncLitAtIndex(cctx *CheckContext, assign *ast.AssignStmt, v *types.Var, indexExpr ast.Expr) *ast.FuncLit {
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
func closureCheckFactoryCall(cctx *CheckContext, call *ast.CallExpr) bool {
	// Check if ctx is passed as an argument to the call
	for _, arg := range call.Args {
		if cctx.argUsesContext(arg) {
			return true
		}
	}

	// Check if the factory function itself is a closure that captures ctx
	// and returns a function that uses it
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		// e.g., makeWorker() where makeWorker := func() func() error { ... }
		obj := cctx.Pass.TypesInfo.ObjectOf(fun)
		if obj == nil {
			return false
		}
		v, ok := obj.(*types.Var)
		if !ok {
			return false
		}
		funcLit := cctx.FindFuncLitAssignment(v, token.NoPos)
		if funcLit == nil {
			return false
		}
		// Check if the factory's return statements return a func that uses ctx
		return cctx.factoryReturnsContextUsingFunc(funcLit)

	case *ast.FuncLit:
		// e.g., g.Go((func() func() error { return func() error { _ = ctx; return nil } })())
		return cctx.factoryReturnsContextUsingFunc(fun)
	}

	return false
}
