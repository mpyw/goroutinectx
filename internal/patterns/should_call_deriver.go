package patterns

import (
	"go/ast"
	"go/types"
	"strings"

	"github.com/mpyw/goroutinectx/internal/context"
	"github.com/mpyw/goroutinectx/internal/directives/deriver"
)

// ShouldCallDeriver checks that a callback body calls a deriver function.
// Used by: gotask.NewTask, etc.
type ShouldCallDeriver struct {
	// Matcher is the deriver function matcher (OR/AND semantics).
	Matcher *deriver.Matcher
}

func (*ShouldCallDeriver) Name() string {
	return "ShouldCallDeriver"
}

func (p *ShouldCallDeriver) Check(cctx *context.CheckContext, call *ast.CallExpr, callbackArg ast.Expr) bool {
	if p.Matcher == nil || p.Matcher.IsEmpty() {
		return true // No deriver configured
	}

	// For function literals, try SSA-based check first
	if lit, ok := callbackArg.(*ast.FuncLit); ok {
		if result, ok := p.checkFromSSA(cctx, lit); ok {
			return result
		}
		// Fall back to AST-based check
		return p.Matcher.SatisfiesAnyGroup(cctx.Pass, lit.Body)
	}

	// For other expressions, use AST-based check
	return p.checkFromAST(cctx, callbackArg)
}

// checkFromSSA uses SSA analysis to check deriver calls.
// Returns (result, true) if SSA analysis succeeded, or (false, false) if it failed.
func (p *ShouldCallDeriver) checkFromSSA(cctx *context.CheckContext, lit *ast.FuncLit) (bool, bool) {
	if cctx.SSAProg == nil || cctx.Tracer == nil {
		return false, false
	}

	ssaFn := cctx.SSAProg.FindFuncLit(lit)
	if ssaFn == nil {
		return false, false
	}

	result := cctx.Tracer.ClosureCallsDeriver(ssaFn, p.Matcher)
	// Only accept deriver calls at start, not in defer
	return result.FoundAtStart, true
}

// checkFromAST falls back to AST-based analysis.
// Handles: Ident (variable), CallExpr (NewTask, factory functions).
func (p *ShouldCallDeriver) checkFromAST(cctx *context.CheckContext, expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.FuncLit:
		return p.Matcher.SatisfiesAnyGroup(cctx.Pass, e.Body)

	case *ast.Ident:
		return p.checkIdent(cctx, e)

	case *ast.CallExpr:
		return p.checkCallExpr(cctx, e)

	default:
		// Can't trace, assume OK (zero false positives)
		return true
	}
}

// checkIdent checks if a variable contains a deriver by tracing its assignment.
func (p *ShouldCallDeriver) checkIdent(cctx *context.CheckContext, ident *ast.Ident) bool {
	v := cctx.VarOf(ident)
	if v == nil {
		return true // Can't trace (not a variable)
	}

	// Check if this is a slice type - we can't trace slice contents
	if _, isSlice := v.Type().Underlying().(*types.Slice); isSlice {
		return false // Can't trace slice contents, report error
	}

	// Try to find a FuncLit assignment
	funcLit := cctx.FuncLitAssignedTo(v, ident.Pos())
	if funcLit != nil {
		return p.Matcher.SatisfiesAnyGroup(cctx.Pass, funcLit.Body)
	}

	// Try to find a CallExpr assignment (e.g., task := NewTask(fn))
	callExpr := cctx.CallExprAssignedTo(v, ident.Pos())
	if callExpr != nil {
		return p.checkCallExpr(cctx, callExpr)
	}

	// Can't trace
	return true
}

// checkCallExpr checks if a call expression contains a deriver.
func (p *ShouldCallDeriver) checkCallExpr(cctx *context.CheckContext, call *ast.CallExpr) bool {
	// Case 1: gotask.NewTask(fn) - check fn
	if p.isGotaskConstructor(cctx, call) && len(call.Args) > 0 {
		return p.checkFromAST(cctx, call.Args[0])
	}

	// Case 2: Factory function - trace return statements
	if p.factoryReturnCalls(cctx, call) {
		return true
	}

	// Case 3: Higher-order callback returns deriver-calling func
	if p.callbackReturnCalls(cctx, call) {
		return true
	}

	// Case 4: Check if the call itself calls deriver
	return p.Matcher.SatisfiesAnyGroup(cctx.Pass, call)
}

// isGotaskConstructor checks if call is gotask.NewTask or similar.
func (p *ShouldCallDeriver) isGotaskConstructor(cctx *context.CheckContext, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	if !strings.HasPrefix(sel.Sel.Name, "New") {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	obj := cctx.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return false
	}

	pkgName, ok := obj.(*types.PkgName)
	if !ok {
		return false
	}

	return strings.HasPrefix(pkgName.Imported().Path(), "github.com/siketyan/gotask")
}

// factoryReturnCalls traces a factory call to its FuncLit and checks returns.
func (p *ShouldCallDeriver) factoryReturnCalls(cctx *context.CheckContext, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}

	funcLit := cctx.FuncLitOfIdent(ident, call.Pos())
	if funcLit == nil {
		return false
	}

	return p.funcLitReturnCalls(cctx, funcLit)
}

// callbackReturnCalls checks if any FuncLit argument returns a deriver-calling func.
func (p *ShouldCallDeriver) callbackReturnCalls(cctx *context.CheckContext, call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		funcLit, ok := arg.(*ast.FuncLit)
		if !ok {
			continue
		}
		if p.funcLitReturnCalls(cctx, funcLit) {
			return true
		}
	}
	return false
}

// funcLitReturnCalls checks if any return statement returns a deriver-calling expr.
func (p *ShouldCallDeriver) funcLitReturnCalls(cctx *context.CheckContext, funcLit *ast.FuncLit) bool {
	var found bool

	ast.Inspect(funcLit.Body, func(n ast.Node) bool {
		if found {
			return false
		}
		// Skip nested func literals
		if fl, ok := n.(*ast.FuncLit); ok && fl != funcLit {
			return false
		}

		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		for _, result := range ret.Results {
			switch r := result.(type) {
			case *ast.CallExpr:
				if p.checkCallExpr(cctx, r) {
					found = true
					return false
				}
			case *ast.FuncLit:
				if p.Matcher.SatisfiesAnyGroup(cctx.Pass, r.Body) {
					found = true
					return false
				}
			}
		}
		return true
	})

	return found
}

func (p *ShouldCallDeriver) Message(apiName string, _ string) string {
	return apiName + "() callback should call goroutine deriver"
}
