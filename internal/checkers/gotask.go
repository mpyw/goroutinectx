package checkers

import (
	"fmt"
	"go/ast"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	gossa "golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/checker"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/funcspec"
	"github.com/mpyw/goroutinectx/internal/probe"
	"github.com/mpyw/goroutinectx/internal/registry"
	"github.com/mpyw/goroutinectx/internal/ssa"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// gotaskConstructor defines how gotask tasks are created.
var gotaskConstructor = &gotaskConstructorConfig{
	pkgPath:        "github.com/siketyan/gotask",
	funcName:       "NewTask",
	callbackArgIdx: 0,
}

// gotaskConstructorConfig defines how tasks are created for task-based APIs.
type gotaskConstructorConfig struct {
	pkgPath        string
	funcName       string
	callbackArgIdx int
}

// GotaskChecker checks calls to the gotask entries of the registry.
type GotaskChecker struct {
	derivers *deriver.Matcher
}

// NewGotaskChecker creates a gotask checker.
func NewGotaskChecker(derivers *deriver.Matcher) *GotaskChecker {
	return &GotaskChecker{derivers: derivers}
}

// Name returns the checker name.
func (*GotaskChecker) Name() checker.Name {
	return checker.Gotask
}

// CheckCall checks the call expression. A variadic API may give a report
// for each argument.
func (c *GotaskChecker) CheckCall(cctx *probe.Context, call *ast.CallExpr) []analysis.Diagnostic {
	if c.derivers.IsEmpty() {
		return nil
	}

	fn := funcspec.ExtractFunc(cctx.Pass, call)
	if fn == nil {
		return nil
	}

	entry, ok := registry.MatchFor(checker.Gotask, fn)
	if !ok {
		return nil
	}

	switch {
	case entry.AlwaysSpawns:
		return c.checkDoAsync(cctx, call, entry)
	case entry.Variadic:
		// For variadic APIs, we report each failing argument separately
		return c.checkVariadic(cctx, call, entry)
	}
	return nil
}

func (c *GotaskChecker) checkDoAsync(cctx *probe.Context, call *ast.CallExpr, entry registry.Entry) []analysis.Diagnostic {
	instr := cctx.SSAProg.CallAt(call)
	if len(call.Args) == 0 || instr == nil {
		return nil
	}
	common := instr.Common()

	// Check 1: Is the ctx argument (first arg) a deriver call?
	if args := ssa.ResolvedArgs(common, call); args != nil && c.ctxIsDerived(args[0]) {
		return nil
	}

	// Check 2: Does the task's callback call the deriver?
	if recv := ssa.ResolvedReceiver(common); recv != nil && c.taskDerives(cctx, recv) {
		return nil
	}

	// Neither condition satisfied - report error with pointer receiver format,
	// at the method name.
	pos := call.Pos()
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		pos = sel.Sel.Pos()
	}
	return []analysis.Diagnostic{{Pos: pos, Message: formatGotaskMethodMessage(entry.Spec.FullName())}}
}

// formatGotaskMethodMessage formats a method name with pointer receiver.
// Input: "gotask.Task.DoAsync"
// Output: "gotask.(*Task).DoAsync() 1st argument should call goroutine deriver"
func formatGotaskMethodMessage(apiName string) string {
	parts := strings.Split(apiName, ".")
	if len(parts) == 3 {
		return parts[0] + ".(*" + parts[1] + ")." + parts[2] + "() 1st argument should call goroutine deriver"
	}
	return apiName + "() 1st argument should call goroutine deriver"
}

func (c *GotaskChecker) checkVariadic(cctx *probe.Context, call *ast.CallExpr, entry registry.Entry) []analysis.Diagnostic {
	startIdx := entry.CallbackArgIdx
	if startIdx >= len(call.Args) {
		return nil
	}

	// Check if this is a variadic expansion (e.g., DoAllFns(ctx, slice...))
	isVariadicExpansion := call.Ellipsis.IsValid()

	args := spawnedCallArgs(cctx, call)
	r := c.resolver(cctx)
	var diags []analysis.Diagnostic
	for i := startIdx; i < len(call.Args); i++ {
		var passes bool
		var v gossa.Value
		if args != nil {
			v = args[i]
		}
		if isVariadicExpansion {
			passes = c.expandedArgDerives(cctx, call.Args[i], v, r)
		} else {
			passes = spawnedFuncPasses(cctx, call.Args[i], v, r, false)
		}
		if passes {
			continue
		}

		var msg string
		if isVariadicExpansion {
			msg = fmt.Sprintf("%s() variadic argument should call goroutine deriver", entry.Spec.FullName())
		} else {
			// Report each failing argument with 1-based position
			msg = fmt.Sprintf("%s() %s argument should call goroutine deriver",
				entry.Spec.FullName(), gotaskArgOrdinal(i+1))
		}
		diags = append(diags, analysis.Diagnostic{Pos: call.Pos(), Message: msg})
	}
	return diags
}

// expandedArgDerives judges the slice of a variadic expansion, v being its
// SSA value. A slice made by a call, as lo.Map makes one, passes when calling
// one of the call's callbacks gives a task function that derives. The
// elements of a slice variable are not traced, so it is reported.
func (c *GotaskChecker) expandedArgDerives(cctx *probe.Context, arg ast.Expr, v gossa.Value, r *ssa.FuncResolver) bool {
	switch arg := arg.(type) {
	case *ast.Ident:
		obj, ok := cctx.Pass.TypesInfo.ObjectOf(arg).(*types.Var)
		if !ok {
			return true
		}
		_, isSlice := obj.Type().Underlying().(*types.Slice)
		return !isSlice
	case *ast.CallExpr:
		call, ok := v.(*gossa.Call)
		if !ok {
			return true
		}
		return slices.ContainsFunc(call.Call.Args, func(arg gossa.Value) bool {
			_, isFunc := arg.Type().Underlying().(*types.Signature)
			return isFunc && r.ResultPasses(arg)
		})
	}
	return true
}

// resolver judges a task function by whether it calls the deriver at its
// start. A task built by NewTask, or by a method of a task, is judged by the
// function it wraps.
func (c *GotaskChecker) resolver(cctx *probe.Context) *ssa.FuncResolver {
	r := &ssa.FuncResolver{
		Closure: func(fn *gossa.Function) bool { return ssa.ClosureCallsDeriver(fn, c.derivers).FoundAtStart },
	}
	r.Call = func(call *gossa.CallCommon) (bool, bool) {
		if call.IsInvoke() {
			// An interface method is not traced, so the task it gives is
			// reported.
			return false, true
		}
		fn := ssa.ExtractCalledFunc(call)
		switch {
		case fn == nil:
			return false, false
		case gotaskIsConstructor(fn):
			return len(call.Args) > 0 && r.Passes(call.Args[0]), true
		case gotaskIsTaskMethod(fn):
			return c.taskDerives(cctx, ssa.ResolvedReceiver(call)), true
		}
		return false, false
	}
	return r
}

// ctxIsDerived reports whether every value v can be is a call of the deriver.
func (c *GotaskChecker) ctxIsDerived(v gossa.Value) bool {
	values, ok := ssa.ResolvedValues(v)
	if !ok || len(values) == 0 {
		return false
	}
	return !slices.ContainsFunc(values, func(v gossa.Value) bool {
		if ext, ok := v.(*gossa.Extract); ok {
			v = ext.Tuple
		}
		call, ok := v.(*gossa.Call)
		if !ok {
			return true
		}
		fn := ssa.ExtractCalledFunc(call.Common())
		return fn == nil || !c.derivers.MatchesFunc(fn)
	})
}

// taskDerives reports whether every task v can be wraps a function that
// calls the deriver. A task traced to nothing does not.
func (c *GotaskChecker) taskDerives(cctx *probe.Context, v gossa.Value) bool {
	if v == nil {
		return false
	}
	values, ok := ssa.ResolvedValues(v)
	if !ok || len(values) == 0 {
		return false
	}
	r := c.resolver(cctx)
	return !slices.ContainsFunc(values, func(v gossa.Value) bool {
		call, ok := v.(*gossa.Call)
		if !ok {
			return true
		}
		passes, handled := r.Call(call.Common())
		return !handled || !passes
	})
}

// gotaskIsConstructor reports whether fn is the task constructor.
func gotaskIsConstructor(fn *types.Func) bool {
	return funcspec.Spec{PkgPath: gotaskConstructor.pkgPath, FuncName: gotaskConstructor.funcName}.Matches(fn)
}

// gotaskIsTaskMethod reports whether fn is a method of a gotask task that
// gives a task, as Cancelable does.
func gotaskIsTaskMethod(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil || sig.Results().Len() != 1 {
		return false
	}
	return gotaskIsTaskType(sig.Recv().Type()) && gotaskIsTaskType(sig.Results().At(0).Type())
}

// gotaskIsTaskType reports whether t is a gotask Task or CancelableTask.
func gotaskIsTaskType(t types.Type) bool {
	named, ok := typeutil.UnwrapPointer(t).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || !typeutil.PkgPathMatches(named.Obj().Pkg().Path(), gotaskConstructor.pkgPath) {
		return false
	}
	name := named.Obj().Name()
	return name == "Task" || name == "CancelableTask"
}

// gotaskArgOrdinal converts an argument number to its ordinal string (1st, 2nd, 3rd, etc.).
func gotaskArgOrdinal(n int) string {
	suffix := "th"
	switch n % 10 {
	case 1:
		if n%100 != 11 {
			suffix = "st"
		}
	case 2:
		if n%100 != 12 {
			suffix = "nd"
		}
	case 3:
		if n%100 != 13 {
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", n, suffix)
}
