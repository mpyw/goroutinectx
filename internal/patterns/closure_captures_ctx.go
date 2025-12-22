package patterns

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/mpyw/goroutinectx/internal/directives/carrier"
	"github.com/mpyw/goroutinectx/internal/typeutil"
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
	if result, ok := checkClosureFromSSA(cctx, callbackArg); ok {
		return result
	}

	// Fall back to AST-based check when SSA fails
	return checkClosureFromAST(cctx, callbackArg)
}

func (*ClosureCapturesCtx) Message(apiName string, ctxName string) string {
	return fmt.Sprintf("%s() closure should use context %q", apiName, ctxName)
}

// checkClosureFromSSA uses SSA analysis to check if a closure captures context.
// Returns (result, true) if SSA analysis succeeded, or (false, false) if it failed.
func checkClosureFromSSA(cctx *CheckContext, callbackArg ast.Expr) (bool, bool) {
	if cctx.SSAProg == nil || cctx.Tracer == nil {
		return false, false
	}

	// For function literals, find the SSA function and check FreeVars
	if lit, ok := callbackArg.(*ast.FuncLit); ok {
		// Skip if closure has its own context parameter
		if funcLitHasContextParam(cctx, lit) {
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

// checkClosureFromAST falls back to AST-based analysis when SSA tracing fails.
// Design principle: "prove safety" - if we can't prove ctx is used, report error.
// This may cause false positives for LIMITATION cases (channel, type assertion),
// but it's better to catch real issues than miss them.
func checkClosureFromAST(cctx *CheckContext, callbackArg ast.Expr) bool {
	// For function literals, check if they reference context
	if lit, ok := callbackArg.(*ast.FuncLit); ok {
		// Skip if closure has its own context parameter
		if funcLitHasContextParam(cctx, lit) {
			return true
		}
		return funcLitUsesContext(cctx, lit)
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
		funcLit := findFuncLitAssignment(cctx, v)
		if funcLit == nil {
			return false // Can't trace (channel receive, type assertion, etc.)
		}
		// Skip if closure has its own context parameter
		if funcLitHasContextParam(cctx, funcLit) {
			return true
		}
		return funcLitUsesContext(cctx, funcLit)
	}

	// For call expressions, check if ctx is passed as argument
	if call, ok := callbackArg.(*ast.CallExpr); ok {
		return checkFactoryCallUsesContext(cctx, call)
	}

	// For selector expressions (struct field access), check the field's func
	if sel, ok := callbackArg.(*ast.SelectorExpr); ok {
		return checkSelectorFuncUsesContext(cctx, sel)
	}

	// For index expressions (slice/map access), check the indexed func
	if idx, ok := callbackArg.(*ast.IndexExpr); ok {
		return checkIndexFuncUsesContext(cctx, idx)
	}

	return false // Can't analyze - report error to catch potential issues
}

// checkSelectorFuncUsesContext checks if a struct field func uses context.
func checkSelectorFuncUsesContext(cctx *CheckContext, sel *ast.SelectorExpr) bool {
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
	funcLit := findStructFieldFuncLit(cctx, v, fieldName)
	if funcLit == nil {
		return false
	}

	return funcLitUsesContext(cctx, funcLit)
}

// findStructFieldFuncLit finds a func literal assigned to a struct field.
func findStructFieldFuncLit(cctx *CheckContext, v *types.Var, fieldName string) *ast.FuncLit {
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
			result = findFieldInAssignment(cctx, assign, v, fieldName)
			return result == nil
		})
		break
	}

	return result
}

// findFieldInAssignment looks for a func literal in a struct field assignment.
func findFieldInAssignment(cctx *CheckContext, assign *ast.AssignStmt, v *types.Var, fieldName string) *ast.FuncLit {
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

// checkIndexFuncUsesContext checks if a slice/map indexed func uses context.
func checkIndexFuncUsesContext(cctx *CheckContext, idx *ast.IndexExpr) bool {
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

	funcLit := findIndexedFuncLit(cctx, v, idx.Index)
	if funcLit == nil {
		return false
	}

	return funcLitUsesContext(cctx, funcLit)
}

// findIndexedFuncLit finds a func literal at a specific index in a composite literal.
func findIndexedFuncLit(cctx *CheckContext, v *types.Var, indexExpr ast.Expr) *ast.FuncLit {
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
			result = findFuncLitAtIndex(cctx, assign, v, indexExpr)
			return result == nil
		})
		break
	}

	return result
}

// findFuncLitAtIndex looks for a func literal at a specific index.
func findFuncLitAtIndex(cctx *CheckContext, assign *ast.AssignStmt, v *types.Var, indexExpr ast.Expr) *ast.FuncLit {
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
			return findFuncLitByLiteral(compLit, lit)
		}
	}
	return nil
}

// findFuncLitByLiteral finds func literal by literal index/key.
func findFuncLitByLiteral(compLit *ast.CompositeLit, lit *ast.BasicLit) *ast.FuncLit {
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

// findFuncLitAssignment searches for the func literal assigned to the variable.
func findFuncLitAssignment(cctx *CheckContext, v *types.Var) *ast.FuncLit {
	var result *ast.FuncLit
	declPos := v.Pos()

	for _, f := range cctx.Pass.Files {
		if f.Pos() > declPos || declPos >= f.End() {
			continue
		}

		ast.Inspect(f, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			if fl := findFuncLitInAssignment(cctx, assign, v); fl != nil {
				result = fl // Keep updating - we want the LAST assignment
			}
			return true
		})
		break
	}

	return result
}

// findFuncLitInAssignment checks if the assignment assigns a func literal to v.
func findFuncLitInAssignment(cctx *CheckContext, assign *ast.AssignStmt, v *types.Var) *ast.FuncLit {
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
		if fl, ok := assign.Rhs[i].(*ast.FuncLit); ok {
			return fl
		}
	}
	return nil
}

// checkFactoryCallUsesContext checks if a factory call passes context.
// Handles patterns like:
//   - g.Go(makeWorkerWithCtx(ctx)) - ctx passed as argument
//   - g.Go(makeWorker()) where makeWorker is a closure that captures ctx
func checkFactoryCallUsesContext(cctx *CheckContext, call *ast.CallExpr) bool {
	// Check if ctx is passed as an argument to the call
	for _, arg := range call.Args {
		if argUsesContext(cctx, arg) {
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
		funcLit := findFuncLitAssignment(cctx, v)
		if funcLit == nil {
			return false
		}
		// Check if the factory's return statements return a func that uses ctx
		return factoryReturnsContextUsingFunc(cctx, funcLit)

	case *ast.FuncLit:
		// e.g., g.Go((func() func() error { return func() error { _ = ctx; return nil } })())
		return factoryReturnsContextUsingFunc(cctx, fun)
	}

	return false
}

// factoryReturnsContextUsingFunc checks if a factory function's return statements
// return functions that use context.
// For nested factories (factories that return factories), this recursively checks
// if any deeply nested function uses context.
func factoryReturnsContextUsingFunc(cctx *CheckContext, factory *ast.FuncLit) bool {
	usesContext := false

	ast.Inspect(factory.Body, func(n ast.Node) bool {
		if usesContext {
			return false
		}
		// For nested func literals, check both direct usage and returned values
		if fl, ok := n.(*ast.FuncLit); ok && fl != factory {
			// Check if this nested func lit uses context directly
			if funcLitUsesContext(cctx, fl) {
				usesContext = true
				return false
			}
			// Recursively check if it returns functions that use context
			// This handles nested factories like: func() func() func() { return ... }
			if factoryReturnsContextUsingFunc(cctx, fl) {
				usesContext = true
				return false
			}
			return false // Don't descend into nested func literals (we handle them recursively)
		}

		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		for _, result := range ret.Results {
			if returnedValueUsesContext(cctx, result) {
				usesContext = true
				return false
			}
		}
		return true
	})

	return usesContext
}

// returnedValueUsesContext checks if a returned value is a func that uses context.
func returnedValueUsesContext(cctx *CheckContext, result ast.Expr) bool {
	// If it's a func literal, check directly
	if innerFuncLit, ok := result.(*ast.FuncLit); ok {
		return funcLitUsesContext(cctx, innerFuncLit)
	}

	// If it's an identifier (variable), find its assignment
	ident, ok := result.(*ast.Ident)
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

	innerFuncLit := findFuncLitAssignment(cctx, v)
	if innerFuncLit == nil {
		return false
	}

	return funcLitUsesContext(cctx, innerFuncLit)
}

// argUsesContext checks if an expression references a context variable.
func argUsesContext(cctx *CheckContext, expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if found {
			return false
		}
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		obj := cctx.Pass.TypesInfo.ObjectOf(ident)
		if obj == nil {
			return true
		}
		if isContextOrCarrierType(obj.Type(), cctx.Carriers) {
			found = true
			return false
		}
		return true
	})
	return found
}

// funcLitHasContextParam checks if a function literal has a context.Context parameter.
func funcLitHasContextParam(cctx *CheckContext, lit *ast.FuncLit) bool {
	if lit.Type == nil || lit.Type.Params == nil {
		return false
	}
	for _, field := range lit.Type.Params.List {
		typ := cctx.Pass.TypesInfo.TypeOf(field.Type)
		if typ == nil {
			continue
		}
		if isContextType(typ) {
			return true
		}
	}
	return false
}

// funcLitUsesContext checks if a function literal references any context variable.
// It does NOT descend into nested func literals - they have their own scope and
// will be checked separately. This matches the LIMITATION behavior where ctx
// used only in deferred nested closures is not counted.
func funcLitUsesContext(cctx *CheckContext, lit *ast.FuncLit) bool {
	usesCtx := false
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		if usesCtx {
			return false
		}
		// Skip nested function literals - they will be checked separately
		// This is intentional for LIMITATION behavior
		if nested, ok := n.(*ast.FuncLit); ok && nested != lit {
			return false
		}
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		obj := cctx.Pass.TypesInfo.ObjectOf(ident)
		if obj == nil {
			return true
		}
		if isContextOrCarrierType(obj.Type(), cctx.Carriers) {
			usesCtx = true
			return false
		}
		return true
	})
	return usesCtx
}

// isContextOrCarrierType checks if a type is context.Context or a configured carrier type.
func isContextOrCarrierType(t types.Type, carriers []carrier.Carrier) bool {
	return typeutil.IsContextOrCarrierType(t, carriers)
}
