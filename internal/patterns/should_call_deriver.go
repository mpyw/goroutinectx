package patterns

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

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

func (p *ShouldCallDeriver) Check(cctx *CheckContext, call *ast.CallExpr, callbackArg ast.Expr) bool {
	if p.Matcher == nil || p.Matcher.IsEmpty() {
		return true // No deriver configured
	}
	return p.exprCallsDeriver(cctx, callbackArg)
}

// exprCallsDeriver checks if an expression contains a call to the deriver function.
// Handles: FuncLit, Ident (variable), CallExpr (NewTask, factory functions).
func (p *ShouldCallDeriver) exprCallsDeriver(cctx *CheckContext, expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.FuncLit:
		return p.Matcher.SatisfiesAnyGroup(cctx.Pass, e.Body)

	case *ast.Ident:
		return p.identCallsDeriver(cctx, e)

	case *ast.CallExpr:
		return p.callExprCallsDeriver(cctx, e)

	default:
		// Can't trace, assume OK (zero false positives)
		return true
	}
}

// identCallsDeriver checks if a variable contains a deriver by tracing its assignment.
func (p *ShouldCallDeriver) identCallsDeriver(cctx *CheckContext, ident *ast.Ident) bool {
	obj := cctx.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return true // Can't trace
	}

	v, ok := obj.(*types.Var)
	if !ok {
		return true // Not a variable
	}

	// Check if this is a slice type - we can't trace slice contents
	if _, isSlice := v.Type().Underlying().(*types.Slice); isSlice {
		return false // Can't trace slice contents, report error
	}

	// Try to find a FuncLit assignment
	funcLit := findFuncLitAssignmentBefore(cctx, v, ident.Pos())
	if funcLit != nil {
		return p.Matcher.SatisfiesAnyGroup(cctx.Pass, funcLit.Body)
	}

	// Try to find a CallExpr assignment (e.g., task := NewTask(fn))
	callExpr := findCallExprAssignmentBefore(cctx, v, ident.Pos())
	if callExpr != nil {
		return p.callExprCallsDeriver(cctx, callExpr)
	}

	// Can't trace
	return true
}

// callExprCallsDeriver checks if a call expression contains a deriver.
func (p *ShouldCallDeriver) callExprCallsDeriver(cctx *CheckContext, call *ast.CallExpr) bool {
	// Case 1: gotask.NewTask(fn) - check fn
	if p.isGotaskConstructor(cctx, call) && len(call.Args) > 0 {
		return p.exprCallsDeriver(cctx, call.Args[0])
	}

	// Case 2: Factory function - trace return statements
	if p.factoryReturnCallsDeriver(cctx, call) {
		return true
	}

	// Case 3: Higher-order callback returns deriver-calling func
	if p.callbackReturnCallsDeriver(cctx, call) {
		return true
	}

	// Case 4: Check if the call itself calls deriver
	return p.Matcher.SatisfiesAnyGroup(cctx.Pass, call)
}

// isGotaskConstructor checks if call is gotask.NewTask or similar.
func (p *ShouldCallDeriver) isGotaskConstructor(cctx *CheckContext, call *ast.CallExpr) bool {
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

// factoryReturnCallsDeriver traces a factory call to its FuncLit and checks returns.
func (p *ShouldCallDeriver) factoryReturnCallsDeriver(cctx *CheckContext, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
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

	funcLit := findFuncLitAssignmentBefore(cctx, v, call.Pos())
	if funcLit == nil {
		return false
	}

	return p.funcLitReturnCallsDeriver(cctx, funcLit)
}

// callbackReturnCallsDeriver checks if any FuncLit argument returns a deriver-calling func.
func (p *ShouldCallDeriver) callbackReturnCallsDeriver(cctx *CheckContext, call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		funcLit, ok := arg.(*ast.FuncLit)
		if !ok {
			continue
		}
		if p.funcLitReturnCallsDeriver(cctx, funcLit) {
			return true
		}
	}
	return false
}

// funcLitReturnCallsDeriver checks if any return statement returns a deriver-calling expr.
func (p *ShouldCallDeriver) funcLitReturnCallsDeriver(cctx *CheckContext, funcLit *ast.FuncLit) bool {
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
				if p.callExprCallsDeriver(cctx, r) {
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

// findFuncLitAssignmentBefore finds the last FuncLit assigned to variable before pos.
func findFuncLitAssignmentBefore(cctx *CheckContext, v *types.Var, beforePos token.Pos) *ast.FuncLit {
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
			if beforePos != token.NoPos && assign.Pos() >= beforePos {
				return true
			}

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
					result = fl
				}
			}
			return true
		})
		break
	}

	return result
}

// findCallExprAssignmentBefore finds the last CallExpr assigned to variable before pos.
func findCallExprAssignmentBefore(cctx *CheckContext, v *types.Var, beforePos token.Pos) *ast.CallExpr {
	var result *ast.CallExpr
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
			if beforePos != token.NoPos && assign.Pos() >= beforePos {
				return true
			}

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
				if call, ok := assign.Rhs[i].(*ast.CallExpr); ok {
					result = call
				}
			}
			return true
		})
		break
	}

	return result
}
