// Package gotask checks that gotask tasks properly derive context
// for APM/telemetry purposes.
//
// Checks:
//   - gotask.Do* functions: task arguments (2nd+) must call the deriver
//   - Task.DoAsync / CancelableTask.DoAsync: ctx argument must be derived
package gotask

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/context"
	"github.com/mpyw/goroutinectx/internal/directives/deriver"
	"github.com/mpyw/goroutinectx/internal/directives/ignore"
)

const pkgPath = "github.com/siketyan/gotask"

// Argument position constants for Do* functions.
const (
	minArgsForDoFamily = 2 // ctx + at least one task
	taskArgStartIndex  = 1 // tasks start at index 1 (after ctx)
)

// Ordinal number constants.
const (
	ordinalSecond = 2
	ordinalThird  = 3
)

// Checker checks gotask calls for proper context derivation.
type Checker struct {
	derives *deriver.Matcher
}

// New creates a new gotask checker with the given derive function specification.
func New(deriveFuncsStr string) *Checker {
	return &Checker{
		derives: deriver.NewMatcher(deriveFuncsStr),
	}
}

// CheckCall implements checkers.CallChecker.
func (c *Checker) CheckCall(cctx *context.CheckContext, call *ast.CallExpr) {
	if c.derives.IsEmpty() {
		return
	}

	c.checkDoFamilyCalls(cctx, call)
	c.checkDoAsyncCall(cctx, call)
}

// checkDoFamilyCalls checks gotask.Do* function calls.
func (c *Checker) checkDoFamilyCalls(cctx *context.CheckContext, call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	if !strings.HasPrefix(sel.Sel.Name, "Do") {
		return
	}

	if !isGotaskPackageCall(cctx.Pass, sel) {
		return
	}

	if len(call.Args) < minArgsForDoFamily {
		return
	}

	methodName := sel.Sel.Name

	if call.Ellipsis != token.NoPos {
		if !c.exprContainsDeriver(cctx, call.Args[taskArgStartIndex]) {
			cctx.Reportf(
				ignore.Gotask,
				call.Pos(),
				"gotask.%s() variadic argument should call goroutine deriver (%s)",
				methodName,
				c.derives.Original,
			)
		}
	} else {
		for i, arg := range call.Args[taskArgStartIndex:] {
			if !c.exprContainsDeriver(cctx, arg) {
				cctx.Reportf(
					ignore.Gotask,
					call.Pos(),
					"gotask.%s() %s argument should call goroutine deriver (%s)",
					methodName,
					ordinal(i+minArgsForDoFamily),
					c.derives.Original,
				)
			}
		}
	}
}

// checkDoAsyncCall checks Task.DoAsync and CancelableTask.DoAsync calls.
func (c *Checker) checkDoAsyncCall(cctx *context.CheckContext, call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	if sel.Sel.Name != "DoAsync" {
		return
	}

	typ := cctx.Pass.TypesInfo.TypeOf(sel.X)
	if typ == nil {
		return
	}

	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}

	named, ok := typ.(*types.Named)
	if !ok {
		return
	}

	// For generic types, get the origin (uninstantiated) type
	if origin := named.Origin(); origin != nil {
		named = origin
	}

	pkg := named.Obj().Pkg()
	if pkg == nil || !strings.HasPrefix(pkg.Path(), pkgPath) {
		return
	}

	typeName := named.Obj().Name()
	if typeName != "Task" && typeName != "CancelableTask" {
		return
	}

	if len(call.Args) == 0 {
		return
	}

	if !c.exprContainsDeriver(cctx, call.Args[0]) {
		cctx.Reportf(
			ignore.Gotask,
			call.Args[0].Pos(),
			"(*gotask.%s).DoAsync() 1st argument should call goroutine deriver (%s)",
			typeName,
			c.derives.Original,
		)
	}
}

// isGotaskPackageCall checks if the selector expression is a call to gotask package.
func isGotaskPackageCall(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	obj := pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return false
	}

	pkgName, ok := obj.(*types.PkgName)
	if !ok {
		return false
	}

	return strings.HasPrefix(pkgName.Imported().Path(), pkgPath)
}

// exprContainsDeriver checks if an expression contains a call to the deriver function.
func (c *Checker) exprContainsDeriver(cctx *context.CheckContext, expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.FuncLit:
		return c.derives.SatisfiesAnyGroup(cctx.Pass, e.Body)

	case *ast.CallExpr:
		return c.callExprContainsDeriver(cctx, e)

	case *ast.Ident:
		return c.identContainsDeriver(cctx, e)

	default:
		// Can't trace, assume okay (zero false positives)
		return true
	}
}

// identContainsDeriver checks if a variable contains a deriver by tracing its assignment.
func (c *Checker) identContainsDeriver(cctx *context.CheckContext, ident *ast.Ident) bool {
	obj := cctx.Pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return false // Can't trace, no deriver found
	}

	v, ok := obj.(*types.Var)
	if !ok {
		return false // Not a variable, no deriver found
	}

	// Try to find a FuncLit assignment (e.g., fn := func() {...})
	funcLit := cctx.FindFuncLitAssignmentBefore(v, ident.Pos())
	if funcLit != nil {
		return c.derives.SatisfiesAnyGroup(cctx.Pass, funcLit.Body)
	}

	// Try to find a NewTask call assignment (e.g., task := gotask.NewTask(fn))
	callExpr := findCallExprAssignmentBefore(cctx, v, ident.Pos())
	if callExpr != nil {
		return c.callExprContainsDeriver(cctx, callExpr)
	}

	// Can't trace (e.g., parameter, slice, etc.), no deriver found
	return false
}

// callExprContainsDeriver checks if a call expression contains a deriver.
func (c *Checker) callExprContainsDeriver(cctx *context.CheckContext, call *ast.CallExpr) bool {
	// Case 1: gotask.NewTask(fn) - check fn
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if isGotaskTaskConstructor(cctx.Pass, sel) && len(call.Args) > 0 {
			return c.exprContainsDeriver(cctx, call.Args[0])
		}
	}

	// Case 2: makeTask() - trace makeTask to FuncLit and check return statements
	if c.traceCallToFuncLitReturn(cctx, call) {
		return true
	}

	// Case 3: higherOrderFn(callback)... - check if callback returns a FuncLit with deriver
	// This handles patterns like lo.Map(items, func(p T, _ int) func(ctx) R { return func(ctx) R { deriver() } })
	if c.callbackReturnsDeriver(cctx, call) {
		return true
	}

	return c.derives.SatisfiesAnyGroup(cctx.Pass, call)
}

// traceCallToFuncLitReturn traces a call like makeTask() to its FuncLit assignment
// and checks if the return statement contains a deriver.
func (c *Checker) traceCallToFuncLitReturn(cctx *context.CheckContext, call *ast.CallExpr) bool {
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

	funcLit := cctx.FindFuncLitAssignmentBefore(v, call.Pos())
	if funcLit == nil {
		return false
	}

	return c.funcLitReturnContainsDeriver(cctx, funcLit)
}

// callbackReturnsDeriver checks if any FuncLit argument to the call returns
// a FuncLit that contains a deriver. This handles patterns like:
//
//	lo.Map(items, func(p T, _ int) func(ctx) R {
//	    return func(ctx context.Context) R { deriver(); ... }
//	})
func (c *Checker) callbackReturnsDeriver(cctx *context.CheckContext, call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		funcLit, ok := arg.(*ast.FuncLit)
		if !ok {
			continue
		}

		if c.funcLitReturnContainsDeriver(cctx, funcLit) {
			return true
		}
	}

	return false
}

// funcLitReturnContainsDeriver checks if any return statement in the func literal
// returns an expression that contains a deriver. This handles:
//   - return gotask.NewTask(func() { deriver() })
//   - return func(ctx) T { deriver(); ... }
func (c *Checker) funcLitReturnContainsDeriver(cctx *context.CheckContext, funcLit *ast.FuncLit) bool {
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
				if c.callExprContainsDeriver(cctx, r) {
					found = true

					return false
				}
			case *ast.FuncLit:
				// return func(ctx) T { deriver(); ... }
				if c.derives.SatisfiesAnyGroup(cctx.Pass, r.Body) {
					found = true

					return false
				}
			}
		}

		return true
	})

	return found
}

// isGotaskTaskConstructor checks if the selector is gotask.NewTask or similar.
func isGotaskTaskConstructor(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	if !strings.HasPrefix(sel.Sel.Name, "New") {
		return false
	}

	return isGotaskPackageCall(pass, sel)
}

// ordinal returns the ordinal string for a number (1st, 2nd, 3rd, etc.)
func ordinal(n int) string {
	switch n {
	case 1:
		return "1st"
	case ordinalSecond:
		return "2nd"
	case ordinalThird:
		return "3rd"
	default:
		return fmt.Sprintf("%dth", n)
	}
}

// findCallExprAssignmentBefore searches for the last call expression assigned to the variable
// before the given position. This is used to trace patterns like `task := gotask.NewTask(fn)`.
func findCallExprAssignmentBefore(cctx *context.CheckContext, v *types.Var, beforePos token.Pos) *ast.CallExpr {
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
			// Skip assignments after the usage point
			if beforePos != token.NoPos && assign.Pos() >= beforePos {
				return true
			}
			// Check if this assignment is to our variable
			if call := findCallExprInAssignment(cctx, assign, v); call != nil {
				result = call // Keep updating - we want the LAST assignment
			}

			return true
		})

		break
	}

	return result
}

// findCallExprInAssignment checks if the assignment assigns a call expression to v.
func findCallExprInAssignment(cctx *context.CheckContext, assign *ast.AssignStmt, v *types.Var) *ast.CallExpr {
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
			return call
		}
	}

	return nil
}
