package checkers

import (
	"go/ast"

	"github.com/mpyw/goroutinectx/internal"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/directive/ignore"
	"github.com/mpyw/goroutinectx/internal/probe"
)

// Goroutine checks that go statements propagate context.
type Goroutine struct{}

// Name returns the checker name for ignore directive matching.
func (*Goroutine) Name() ignore.CheckerName {
	return ignore.Goroutine
}

// CheckGoStmt checks a go statement for context propagation.
func (c *Goroutine) CheckGoStmt(cctx *probe.Context, stmt *ast.GoStmt) *internal.Result {
	if len(cctx.CtxNames) == 0 {
		return internal.OK()
	}

	// Try SSA-based check first
	if lit, ok := stmt.Call.Fun.(*ast.FuncLit); ok {
		if result, ok := cctx.FuncLitCapturesContextSSA(lit); ok {
			if result {
				return internal.OK()
			}
			return internal.Fail(c.message(cctx))
		}
	}

	// Fall back to AST-based check
	if c.checkFromAST(cctx, stmt) {
		return internal.OK()
	}
	return internal.Fail(c.message(cctx))
}

func (c *Goroutine) message(cctx *probe.Context) string {
	ctxName := "ctx"
	if len(cctx.CtxNames) > 0 {
		ctxName = cctx.CtxNames[0]
	}
	return "goroutine does not propagate context \"" + ctxName + "\""
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
		funcLit := cctx.FuncLitOfIdent(ident)
		if funcLit == nil {
			return true
		}
		return cctx.FuncLitCapturesContext(funcLit)
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
func (*GoroutineDerive) Name() ignore.CheckerName {
	return ignore.GoroutineDerive
}

// CheckGoStmt checks a go statement for deriver function calls.
func (c *GoroutineDerive) CheckGoStmt(cctx *probe.Context, stmt *ast.GoStmt) *internal.Result {
	if c.derivers == nil || c.derivers.IsEmpty() {
		return internal.OK()
	}

	call := stmt.Call

	if lit, ok := call.Fun.(*ast.FuncLit); ok {
		if cctx.FuncLitHasContextParam(lit) {
			return internal.OK()
		}

		if result, ok := c.checkFromSSA(cctx, lit); ok {
			return result
		}

		if c.derivers.SatisfiesAnyGroup(cctx.Pass, lit.Body) {
			return internal.OK()
		}
		return internal.Fail(c.message())
	}

	if innerCall, ok := call.Fun.(*ast.CallExpr); ok {
		if c.checkHigherOrder(cctx, innerCall) {
			return internal.OK()
		}
		return internal.Fail(c.message())
	}

	if ident, ok := call.Fun.(*ast.Ident); ok {
		if c.checkIdent(cctx, ident) {
			return internal.OK()
		}
		return internal.Fail(c.message())
	}

	return internal.OK()
}

func (c *GoroutineDerive) message() string {
	return "goroutine should call " + c.derivers.Original + " to derive context"
}

func (c *GoroutineDerive) deferMessage() string {
	return "goroutine calls " + c.derivers.Original + " in defer, but it should be called at goroutine start"
}

func (c *GoroutineDerive) checkFromSSA(cctx *probe.Context, lit *ast.FuncLit) (*internal.Result, bool) {
	if cctx.SSAProg == nil || cctx.Tracer == nil {
		return nil, false
	}

	ssaFn := cctx.SSAProg.FindFuncLit(lit)
	if ssaFn == nil {
		return nil, false
	}

	result := cctx.Tracer.ClosureCallsDeriver(ssaFn, c.derivers)

	if result.FoundAtStart {
		return internal.OK(), true
	}

	if result.FoundOnlyInDefer {
		return internal.FailWithDefer(c.message(), c.deferMessage()), true
	}

	return internal.Fail(c.message()), true
}

func (c *GoroutineDerive) checkIdent(cctx *probe.Context, ident *ast.Ident) bool {
	funcLit := cctx.FuncLitOfIdent(ident)
	if funcLit == nil {
		return true
	}

	if cctx.FuncLitHasContextParam(funcLit) {
		return true
	}

	return c.derivers.SatisfiesAnyGroup(cctx.Pass, funcLit.Body)
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
		funcLit := cctx.FuncLitOfIdent(ident)
		if funcLit == nil {
			return true
		}
		if cctx.FuncLitHasContextParam(funcLit) {
			return true
		}
		return c.factoryReturnsCallingFunc(cctx, funcLit)
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
			if cctx.FuncLitHasContextParam(fl) {
				callsDeriver = true
				return false
			}
			if c.derivers.SatisfiesAnyGroup(cctx.Pass, fl.Body) {
				callsDeriver = true
				return false
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
		if cctx.FuncLitHasContextParam(innerFuncLit) {
			return true
		}
		return c.derivers.SatisfiesAnyGroup(cctx.Pass, innerFuncLit.Body)
	}

	ident, ok := result.(*ast.Ident)
	if !ok {
		return false
	}

	innerFuncLit := cctx.FuncLitOfIdent(ident)
	if innerFuncLit == nil {
		return false
	}

	if cctx.FuncLitHasContextParam(innerFuncLit) {
		return true
	}
	return c.derivers.SatisfiesAnyGroup(cctx.Pass, innerFuncLit.Body)
}
