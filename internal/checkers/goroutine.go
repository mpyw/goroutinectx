package checkers

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/checker"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/probe"
	"github.com/mpyw/goroutinectx/internal/ssa"
)

// Goroutine checks that go statements propagate context.
type Goroutine struct{}

// Name returns the checker name for ignore directive matching.
func (*Goroutine) Name() checker.Name {
	return checker.Goroutine
}

// CheckGoStmt checks a go statement for context propagation.
func (c *Goroutine) CheckGoStmt(cctx *probe.Context, stmt *ast.GoStmt) []analysis.Diagnostic {
	if c.propagates(cctx, stmt) {
		return nil
	}
	return []analysis.Diagnostic{{
		Pos:     stmt.Pos(),
		Message: "goroutine does not propagate context \"" + cctx.CtxNames[0] + "\"",
	}}
}

func (c *Goroutine) propagates(cctx *probe.Context, stmt *ast.GoStmt) bool {
	// Try SSA-based check first
	if lit, ok := stmt.Call.Fun.(*ast.FuncLit); ok {
		if result, ok := cctx.SSAProg.FuncLitCapturesContext(lit, cctx.Carriers); ok {
			return result
		}
	}

	// Fall back to AST-based check
	return c.checkFromAST(cctx, stmt)
}

// checkFromAST falls back to AST-based analysis for go statements.
func (*Goroutine) checkFromAST(cctx *probe.Context, stmt *ast.GoStmt) bool {
	call := stmt.Call

	if lit, ok := call.Fun.(*ast.FuncLit); ok {
		return cctx.FuncLitCapturesContext(lit)
	}

	if innerCall, ok := call.Fun.(*ast.CallExpr); ok {
		return cctx.FactoryCallReturnsContextUsingFunc(innerCall)
	}

	if ident, ok := call.Fun.(*ast.Ident); ok {
		assigns := cctx.FuncLitAssignmentsOfIdent(ident)
		if len(assigns) == 0 {
			return true
		}
		return cctx.FuncLitsAllCaptureContext(assigns)
	}

	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		return cctx.SelectorExprCapturesContext(sel)
	}

	if idx, ok := call.Fun.(*ast.IndexExpr); ok {
		return cctx.IndexExprCapturesContext(idx)
	}

	return true
}

// GoroutineDerive checks that go statements call a deriver function.
type GoroutineDerive struct {
	derivers *deriver.Matcher
}

// NewGoroutineDerive creates a new GoroutineDerive checker.
func NewGoroutineDerive(derivers *deriver.Matcher) *GoroutineDerive {
	return &GoroutineDerive{derivers: derivers}
}

// Name returns the checker name for ignore directive matching.
func (*GoroutineDerive) Name() checker.Name {
	return checker.GoroutineDerive
}

// CheckGoStmt checks a go statement for deriver function calls.
func (c *GoroutineDerive) CheckGoStmt(cctx *probe.Context, stmt *ast.GoStmt) []analysis.Diagnostic {
	msg := c.failure(cctx, stmt)
	if msg == "" {
		return nil
	}
	return []analysis.Diagnostic{{Pos: stmt.Pos(), Message: msg}}
}

// failure gives the message to report for stmt, or "" when it derives.
func (c *GoroutineDerive) failure(cctx *probe.Context, stmt *ast.GoStmt) string {
	if c.derivers.IsEmpty() {
		return ""
	}

	call := stmt.Call

	if lit, ok := call.Fun.(*ast.FuncLit); ok {
		if cctx.FuncLitHasContextParam(lit) {
			return ""
		}

		if msg, ok := c.checkFromSSA(cctx, lit); ok {
			return msg
		}

		if c.derivers.SatisfiesAnyGroup(cctx.Pass, lit.Body) {
			return ""
		}
		return c.message()
	}

	if innerCall, ok := call.Fun.(*ast.CallExpr); ok {
		if c.checkHigherOrder(cctx, innerCall) {
			return ""
		}
		return c.message()
	}

	if ident, ok := call.Fun.(*ast.Ident); ok {
		if c.checkIdent(cctx, ident) {
			return ""
		}
		return c.message()
	}

	return ""
}

func (c *GoroutineDerive) message() string {
	return "goroutine should call " + c.derivers.Original + " to derive context"
}

func (c *GoroutineDerive) deferMessage() string {
	return "goroutine calls " + c.derivers.Original + " in defer, but it should be called at goroutine start"
}

// checkFromSSA gives the message to report for lit, or "" when it derives.
// ok is false when lit has no SSA function.
func (c *GoroutineDerive) checkFromSSA(cctx *probe.Context, lit *ast.FuncLit) (msg string, ok bool) {
	ssaFn := cctx.SSAProg.FindFuncLit(lit)
	if ssaFn == nil {
		return "", false
	}

	result := ssa.ClosureCallsDeriver(ssaFn, c.derivers)

	if result.FoundAtStart {
		return "", true
	}

	if result.FoundOnlyInDefer {
		return c.deferMessage(), true
	}

	return c.message(), true
}

func (c *GoroutineDerive) checkIdent(cctx *probe.Context, ident *ast.Ident) bool {
	assigns := cctx.FuncLitAssignmentsOfIdent(ident)
	if len(assigns) == 0 {
		return true
	}

	for _, assign := range probe.EffectiveFuncLitAssignments(assigns) {
		if !c.funcLitDerives(cctx, assign.Lit) {
			return false
		}
	}

	return true
}

func (c *GoroutineDerive) checkHigherOrder(cctx *probe.Context, innerCall *ast.CallExpr) bool {
	fun := innerCall.Fun
	if paren, ok := fun.(*ast.ParenExpr); ok {
		fun = paren.X
	}

	if lit, ok := fun.(*ast.FuncLit); ok {
		if cctx.FuncLitHasContextParam(lit) {
			return true
		}
		return c.factoryReturnsCallingFunc(cctx, lit)
	}

	if ident, ok := fun.(*ast.Ident); ok {
		assigns := cctx.FuncLitAssignmentsOfIdent(ident)
		if len(assigns) == 0 {
			return true
		}

		for _, assign := range probe.EffectiveFuncLitAssignments(assigns) {
			lit := assign.Lit
			if !cctx.FuncLitHasContextParam(lit) && !c.factoryReturnsCallingFunc(cctx, lit) {
				return false
			}
		}
		return true
	}

	return true
}

func (c *GoroutineDerive) factoryReturnsCallingFunc(cctx *probe.Context, factory *ast.FuncLit) bool {
	callsDeriver := false

	ast.Inspect(factory.Body, func(n ast.Node) bool {
		if callsDeriver {
			return false
		}
		if fl, ok := n.(*ast.FuncLit); ok && fl != factory {
			if c.funcLitDerives(cctx, fl) {
				callsDeriver = true
			}
			return false
		}

		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		for _, result := range ret.Results {
			if c.returnedValueCalls(cctx, result) {
				callsDeriver = true
				return false
			}
		}
		return true
	})

	return callsDeriver
}

func (c *GoroutineDerive) returnedValueCalls(cctx *probe.Context, result ast.Expr) bool {
	if innerFuncLit, ok := result.(*ast.FuncLit); ok {
		return c.funcLitDerives(cctx, innerFuncLit)
	}

	ident, ok := result.(*ast.Ident)
	if !ok {
		return false
	}

	assigns := cctx.FuncLitAssignmentsOfIdent(ident)
	if len(assigns) == 0 {
		return false
	}

	for _, assign := range probe.EffectiveFuncLitAssignments(assigns) {
		if !c.funcLitDerives(cctx, assign.Lit) {
			return false
		}
	}
	return true
}

// funcLitDerives reports whether lit takes a context parameter or calls the
// deriver.
func (c *GoroutineDerive) funcLitDerives(cctx *probe.Context, lit *ast.FuncLit) bool {
	return cctx.FuncLitHasContextParam(lit) || c.derivers.SatisfiesAnyGroup(cctx.Pass, lit.Body)
}
