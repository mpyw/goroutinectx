package probe

import (
	"go/ast"

	"github.com/mpyw/goroutinectx/internal/directive/carrier"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// FuncLitCapturesContextSSA uses SSA analysis to check if a func literal captures context.
// Returns (result, true) if SSA analysis succeeded, or (false, false) if it failed.
func (c *Context) FuncLitCapturesContextSSA(lit *ast.FuncLit) (bool, bool) {
	if c.SSAProg == nil || c.Tracer == nil {
		return false, false
	}

	if c.FuncLitHasContextParam(lit) {
		return true, true
	}

	ssaFn := c.SSAProg.FindFuncLit(lit)
	if ssaFn == nil {
		return false, false
	}

	return c.Tracer.ClosureCapturesContext(ssaFn, c.Carriers), true
}

// FuncTypeHasContextParam checks if a function type has a context.Context parameter.
func (c *Context) FuncTypeHasContextParam(fnType *ast.FuncType) bool {
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
func (c *Context) FuncLitHasContextParam(lit *ast.FuncLit) bool {
	return c.FuncTypeHasContextParam(lit.Type)
}

// FuncLitCapturesContext checks if a func literal captures context (AST-based).
func (c *Context) FuncLitCapturesContext(lit *ast.FuncLit) bool {
	return c.FuncLitHasContextParam(lit) || c.FuncLitUsesContext(lit)
}

// FuncLitsAllCaptureContext checks if func literals properly capture context.
// Uses conditionality information to determine the correct check:
// - Find the last unconditional assignment
// - Check all assignments from that point onwards (including conditional ones)
// - ALL must capture context for the check to pass
func (c *Context) FuncLitsAllCaptureContext(assigns []FuncLitAssignment) bool {
	if len(assigns) == 0 {
		return true
	}

	// Find the index of the last unconditional assignment
	lastUnconditionalIdx := -1
	for i := len(assigns) - 1; i >= 0; i-- {
		if !assigns[i].Conditional {
			lastUnconditionalIdx = i
			break
		}
	}

	// Determine the starting point for checks
	startIdx := 0
	if lastUnconditionalIdx >= 0 {
		startIdx = lastUnconditionalIdx
	}

	// Check all assignments from startIdx onwards
	// ALL must capture context (because conditional assignments may override)
	for i := startIdx; i < len(assigns); i++ {
		if !c.FuncLitCapturesContext(assigns[i].Lit) {
			return false
		}
	}
	return true
}

// FuncLitUsesContext checks if a function literal references any context variable.
// Does NOT descend into nested func literals.
func (c *Context) FuncLitUsesContext(lit *ast.FuncLit) bool {
	return c.nodeReferencesContext(lit.Body, true)
}

// ArgUsesContext checks if an expression references a context variable.
// Unlike FuncLitUsesContext, this DOES descend into nested func literals.
func (c *Context) ArgUsesContext(expr ast.Expr) bool {
	return c.nodeReferencesContext(expr, false)
}

// ArgsUseContext checks if any argument references a context variable.
func (c *Context) ArgsUseContext(args []ast.Expr) bool {
	for _, arg := range args {
		if c.ArgUsesContext(arg) {
			return true
		}
	}
	return false
}

// nodeReferencesContext checks if a node references any context variable.
func (c *Context) nodeReferencesContext(node ast.Node, skipNestedFuncLit bool) bool {
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
		if typeutil.IsContextType(obj.Type()) || carrier.IsCarrierType(obj.Type(), c.Carriers) {
			found = true
			return false
		}
		return true
	})
	return found
}
