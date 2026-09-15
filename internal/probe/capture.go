package probe

import (
	"go/ast"
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

	// ALL must capture context (because conditional assignments may override)
	for _, assign := range EffectiveFuncLitAssignments(assigns) {
		if !c.FuncLitCapturesContext(assign.Lit) {
			return false
		}
	}
	return true
}

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
	funcLit := c.FuncLitAssignedToStructField(v, fieldName)
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

	funcLit := c.FuncLitAssignedToIndex(v, idx.Index)
	if funcLit == nil {
		return true
	}

	return c.FuncLitUsesContext(funcLit)
}
