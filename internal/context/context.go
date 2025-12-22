// Package context provides CheckContext for pattern checking.
package context

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/directives/carrier"
	"github.com/mpyw/goroutinectx/internal/funcspec"
	internalssa "github.com/mpyw/goroutinectx/internal/ssa"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// CheckContext provides context for pattern checking.
type CheckContext struct {
	Pass    *analysis.Pass
	Tracer  *internalssa.Tracer
	SSAProg *internalssa.Program
	// CtxNames holds the context variable names from the enclosing scope (AST-based).
	// This is used when SSA-based context detection fails.
	CtxNames []string
	// Carriers holds the configured context carrier types.
	Carriers []carrier.Carrier
}

// Report reports a diagnostic at the given position.
func (c *CheckContext) Report(pos token.Pos, msg string) {
	c.Pass.Reportf(pos, "%s", msg)
}

// VarOf extracts *types.Var from an identifier.
// Returns nil if the identifier doesn't refer to a variable.
func (c *CheckContext) VarOf(ident *ast.Ident) *types.Var {
	obj := c.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return nil
	}
	v, ok := obj.(*types.Var)
	if !ok {
		return nil
	}
	return v
}

// FileOf finds the file that contains the given position.
// Returns nil if no file contains the position.
func (c *CheckContext) FileOf(pos token.Pos) *ast.File {
	for _, f := range c.Pass.Files {
		if f.Pos() <= pos && pos < f.End() {
			return f
		}
	}
	return nil
}

// FuncDeclOf finds the FuncDecl for a types.Func.
// Returns nil if the function declaration is not found in the analyzed files.
func (c *CheckContext) FuncDeclOf(fn *types.Func) *ast.FuncDecl {
	pos := fn.Pos()
	f := c.FileOf(pos)
	if f == nil {
		return nil
	}
	for _, decl := range f.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			if funcDecl.Name.Pos() == pos {
				return funcDecl
			}
		}
	}
	return nil
}

// FuncLitCapturesContextSSA uses SSA analysis to check if a func literal captures context.
// Returns (result, true) if SSA analysis succeeded, or (false, false) if it failed.
func (c *CheckContext) FuncLitCapturesContextSSA(lit *ast.FuncLit) (bool, bool) {
	if c.SSAProg == nil || c.Tracer == nil {
		return false, false
	}

	// Skip if closure has its own context parameter
	if c.FuncLitHasContextParam(lit) {
		return true, true
	}

	ssaFn := c.SSAProg.FindFuncLit(lit)
	if ssaFn == nil {
		return false, false // SSA lookup failed
	}

	return c.Tracer.ClosureCapturesContext(ssaFn, c.Carriers), true
}

// FuncTypeHasContextParam checks if a function type has a context.Context parameter.
func (c *CheckContext) FuncTypeHasContextParam(fnType *ast.FuncType) bool {
	if fnType == nil || fnType.Params == nil {
		return false
	}
	for _, field := range fnType.Params.List {
		typ := c.Pass.TypesInfo.TypeOf(field.Type)
		if typ == nil {
			continue
		}
		if typeutil.IsContextType(typ) {
			return true
		}
	}
	return false
}

// FuncLitHasContextParam checks if a function literal has a context.Context parameter.
func (c *CheckContext) FuncLitHasContextParam(lit *ast.FuncLit) bool {
	return c.FuncTypeHasContextParam(lit.Type)
}

// FuncLitCapturesContext checks if a func literal captures context (AST-based).
// Returns true if the func has its own context param, or if it uses a context from outer scope.
func (c *CheckContext) FuncLitCapturesContext(lit *ast.FuncLit) bool {
	if c.FuncLitHasContextParam(lit) {
		return true
	}
	return c.FuncLitUsesContext(lit)
}

// FuncLitUsesContext checks if a function literal references any context variable.
// It does NOT descend into nested func literals - they have their own scope and
// will be checked separately.
func (c *CheckContext) FuncLitUsesContext(lit *ast.FuncLit) bool {
	return c.nodeReferencesContext(lit.Body, true)
}

// FuncOf extracts the types.Func from a call expression.
func (c *CheckContext) FuncOf(call *ast.CallExpr) *types.Func {
	return funcspec.ExtractFunc(c.Pass, call)
}

// ArgUsesContext checks if an expression references a context variable.
func (c *CheckContext) ArgUsesContext(expr ast.Expr) bool {
	return c.nodeReferencesContext(expr, false)
}

// ArgsUseContext checks if any argument references a context variable.
func (c *CheckContext) ArgsUseContext(args []ast.Expr) bool {
	for _, arg := range args {
		if c.ArgUsesContext(arg) {
			return true
		}
	}
	return false
}

// nodeReferencesContext checks if a node references any context variable.
// If skipNestedFuncLit is true, nested function literals are not traversed.
func (c *CheckContext) nodeReferencesContext(node ast.Node, skipNestedFuncLit bool) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		if skipNestedFuncLit {
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
		}
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		obj := c.Pass.TypesInfo.ObjectOf(ident)
		if obj == nil {
			return true
		}
		if typeutil.IsContextOrCarrierType(obj.Type(), c.Carriers) {
			found = true
			return false
		}
		return true
	})
	return found
}

// FuncLitOfIdent is a convenience method that combines VarOf and FuncLitAssignedTo.
// Returns nil if the identifier doesn't refer to a variable or no func literal assignment is found.
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
		if fl := c.findFuncLitInAssignment(assign, v); fl != nil {
			result = fl // Keep updating - we want the LAST assignment
		}
		return true
	})

	return result
}

// findFuncLitInAssignment checks if the assignment assigns a func literal to v.
func (c *CheckContext) findFuncLitInAssignment(assign *ast.AssignStmt, v *types.Var) *ast.FuncLit {
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
		if call := c.findCallExprInAssignment(assign, v); call != nil {
			result = call // Keep updating - we want the LAST assignment
		}
		return true
	})

	return result
}

// findCallExprInAssignment checks if the assignment assigns a call expression to v.
func (c *CheckContext) findCallExprInAssignment(assign *ast.AssignStmt, v *types.Var) *ast.CallExpr {
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

// BlockReturnsContextUsingFunc checks if a block's return statements
// return functions that use context. Recursively checks nested func literals.
// excludeFuncLit can be set to exclude a specific FuncLit from being counted (e.g., the parent).
func (c *CheckContext) BlockReturnsContextUsingFunc(body *ast.BlockStmt, excludeFuncLit *ast.FuncLit) bool {
	if body == nil {
		return true // No body to check
	}

	usesContext := false

	ast.Inspect(body, func(n ast.Node) bool {
		if usesContext {
			return false
		}
		// For nested func literals, check both direct usage and returned values
		if fl, ok := n.(*ast.FuncLit); ok && fl != excludeFuncLit {
			// Check if this nested func lit uses context directly
			if c.FuncLitUsesContext(fl) {
				usesContext = true
				return false
			}
			// Recursively check if it returns functions that use context
			if c.BlockReturnsContextUsingFunc(fl.Body, fl) {
				usesContext = true
				return false
			}
			return false // Don't descend into nested func literals
		}

		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		for _, result := range ret.Results {
			if c.returnedValueUsesContext(result) {
				usesContext = true
				return false
			}
		}
		return true
	})

	return usesContext
}

// FactoryReturnsContextUsingFunc checks if a factory FuncLit's return statements
// return functions that use context.
func (c *CheckContext) FactoryReturnsContextUsingFunc(factory *ast.FuncLit) bool {
	return c.BlockReturnsContextUsingFunc(factory.Body, factory)
}

// FactoryCallReturnsContextUsingFunc checks if a factory call returns a context-using func.
// Handles: fn(ctx), fn() where fn captures ctx, (func(){...})(), and nested calls.
func (c *CheckContext) FactoryCallReturnsContextUsingFunc(call *ast.CallExpr) bool {
	// Check if ctx is passed as an argument to the call
	if c.ArgsUseContext(call.Args) {
		return true
	}

	// Check if the factory function itself is a closure that captures ctx
	switch fun := call.Fun.(type) {
	case *ast.FuncLit:
		if c.FuncLitHasContextParam(fun) {
			return true
		}
		return c.FactoryReturnsContextUsingFunc(fun)

	case *ast.Ident:
		return c.IdentFactoryReturnsContextUsingFunc(fun)

	case *ast.CallExpr:
		// Handle nested CallExpr for deeper chains like fn()()()
		return c.FactoryCallReturnsContextUsingFunc(fun)
	}

	return true // Can't analyze, assume OK
}

// IdentFactoryReturnsContextUsingFunc checks if an identifier refers to a factory
// that returns a context-using func. Handles both local variables and package-level functions.
func (c *CheckContext) IdentFactoryReturnsContextUsingFunc(ident *ast.Ident) bool {
	obj := c.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return true // Can't trace, assume OK
	}

	// Handle local variable pointing to a func literal
	if v := c.VarOf(ident); v != nil {
		funcLit := c.FuncLitAssignedTo(v, token.NoPos)
		if funcLit == nil {
			return true // Can't trace, assume OK
		}
		// Skip if closure has its own context parameter
		if c.FuncLitHasContextParam(funcLit) {
			return true
		}
		return c.FactoryReturnsContextUsingFunc(funcLit)
	}

	// Handle package-level function declaration
	if fn, ok := obj.(*types.Func); ok {
		funcDecl := c.FuncDeclOf(fn)
		if funcDecl == nil {
			return true // Can't trace, assume OK
		}
		// Skip if function has context parameter
		if c.FuncTypeHasContextParam(funcDecl.Type) {
			return true
		}
		return c.BlockReturnsContextUsingFunc(funcDecl.Body, nil)
	}

	return true // Can't analyze, assume OK
}

// returnedValueUsesContext checks if a returned value is a func that uses context.
func (c *CheckContext) returnedValueUsesContext(result ast.Expr) bool {
	// If it's a func literal, check directly
	if innerFuncLit, ok := result.(*ast.FuncLit); ok {
		return c.FuncLitUsesContext(innerFuncLit)
	}

	// If it's an identifier (variable), find its assignment
	ident, ok := result.(*ast.Ident)
	if !ok {
		return false
	}

	innerFuncLit := c.FuncLitOfIdent(ident, token.NoPos)
	if innerFuncLit == nil {
		return false
	}

	return c.FuncLitUsesContext(innerFuncLit)
}

// SelectorExprCapturesContext checks if a struct field func captures context.
// Handles patterns like: s.handler where s is a struct with a func field.
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
