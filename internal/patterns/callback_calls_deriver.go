package patterns

import (
	"go/ast"
	"go/types"

	"github.com/mpyw/goroutinectx/internal/context"
	"github.com/mpyw/goroutinectx/internal/directives/deriver"
	"github.com/mpyw/goroutinectx/internal/directives/ignore"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// CallbackCallsDeriver checks that a callback body calls a deriver function.
// Used by: gotask.DoAllFns, etc.
//
// This is the strict version - callback MUST call deriver.
// For a flexible version that also accepts derived ctx argument,
// see CallbackCallsDeriverOrCtxDerived.
type CallbackCallsDeriver struct {
	// Matcher is the deriver function matcher (OR/AND semantics).
	Matcher *deriver.Matcher
}

func (p *CallbackCallsDeriver) OrCtxDerived() *CallbackCallsDeriverOrCtxDerived {
	return &CallbackCallsDeriverOrCtxDerived{
		CallbackCallsDeriver: *p,
	}
}

func (*CallbackCallsDeriver) Name() string {
	return "CallbackCallsDeriver"
}

func (*CallbackCallsDeriver) CheckerName() ignore.CheckerName {
	return ignore.Gotask
}

func (p *CallbackCallsDeriver) Check(cctx *context.CheckContext, arg ast.Expr, constructor *TaskConstructorConfig) bool {
	if p.Matcher == nil || p.Matcher.IsEmpty() {
		return true // No deriver configured
	}

	// For function literals, try SSA-based check first
	if lit, ok := arg.(*ast.FuncLit); ok {
		if result, ok := p.checkFromSSA(cctx, lit); ok {
			return result
		}
		// Fall back to AST-based check
		return p.Matcher.SatisfiesAnyGroup(cctx.Pass, lit.Body)
	}

	// For other expressions, use AST-based check
	return p.checkFromAST(cctx, arg, constructor)
}

// checkFromSSA uses SSA analysis to check deriver calls.
// Returns (result, true) if SSA analysis succeeded, or (false, false) if it failed.
func (p *CallbackCallsDeriver) checkFromSSA(cctx *context.CheckContext, lit *ast.FuncLit) (bool, bool) {
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
func (p *CallbackCallsDeriver) checkFromAST(cctx *context.CheckContext, expr ast.Expr, constructor *TaskConstructorConfig) bool {
	switch e := expr.(type) {
	case *ast.FuncLit:
		return p.Matcher.SatisfiesAnyGroup(cctx.Pass, e.Body)

	case *ast.Ident:
		return p.checkIdent(cctx, e, constructor)

	case *ast.CallExpr:
		return p.checkCallExpr(cctx, e, constructor)

	default:
		// Can't trace, assume OK (zero false positives)
		return true
	}
}

// checkIdent checks if a variable contains a deriver by tracing its assignment.
func (p *CallbackCallsDeriver) checkIdent(cctx *context.CheckContext, ident *ast.Ident, constructor *TaskConstructorConfig) bool {
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
		return p.checkCallExpr(cctx, callExpr, constructor)
	}

	// Can't trace
	return true
}

// checkCallExpr checks if a call expression contains a deriver.
func (p *CallbackCallsDeriver) checkCallExpr(cctx *context.CheckContext, call *ast.CallExpr, constructor *TaskConstructorConfig) bool {
	// Case 1: Task constructor (e.g., NewTask(fn)) - check fn
	if constructor != nil {
		if isTaskConstructorCall(cctx, call, constructor) {
			argIdx := constructor.CallbackArgIdx
			if argIdx >= 0 && argIdx < len(call.Args) {
				return p.checkFromAST(cctx, call.Args[argIdx], constructor)
			}
		}
	}

	// Case 2: Factory function - trace return statements
	if p.factoryReturnCalls(cctx, call, constructor) {
		return true
	}

	// Case 3: Higher-order callback returns deriver-calling func
	if p.callbackReturnCalls(cctx, call, constructor) {
		return true
	}

	// Case 4: Check if the call itself calls deriver
	return p.Matcher.SatisfiesAnyGroup(cctx.Pass, call)
}

// factoryReturnCalls traces a factory call to its FuncLit and checks returns.
func (p *CallbackCallsDeriver) factoryReturnCalls(cctx *context.CheckContext, call *ast.CallExpr, constructor *TaskConstructorConfig) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}

	funcLit := cctx.FuncLitOfIdent(ident, call.Pos())
	if funcLit == nil {
		return false
	}

	return p.funcLitReturnCalls(cctx, funcLit, constructor)
}

// callbackReturnCalls checks if any FuncLit argument returns a deriver-calling func.
func (p *CallbackCallsDeriver) callbackReturnCalls(cctx *context.CheckContext, call *ast.CallExpr, constructor *TaskConstructorConfig) bool {
	for _, arg := range call.Args {
		funcLit, ok := arg.(*ast.FuncLit)
		if !ok {
			continue
		}
		if p.funcLitReturnCalls(cctx, funcLit, constructor) {
			return true
		}
	}
	return false
}

// funcLitReturnCalls checks if any return statement returns a deriver-calling expr.
func (p *CallbackCallsDeriver) funcLitReturnCalls(cctx *context.CheckContext, funcLit *ast.FuncLit, constructor *TaskConstructorConfig) bool {
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
				if p.checkCallExpr(cctx, r, constructor) {
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

// isTaskConstructorCall checks if call matches the given task constructor.
func isTaskConstructorCall(cctx *context.CheckContext, call *ast.CallExpr, tc *TaskConstructorConfig) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	if sel.Sel.Name != tc.Name {
		return false
	}

	// Check for package-level function (Type is empty)
	if tc.Type == "" {
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

		return typeutil.MatchPkg(pkgName.Imported().Path(), tc.Pkg)
	}

	// Method-style constructors (Type is set) are not supported.
	// They don't work with generics and are rarely used in practice.
	return false
}

func (p *CallbackCallsDeriver) Message(apiName string, _ string) string {
	return apiName + "() callback should call goroutine deriver"
}
