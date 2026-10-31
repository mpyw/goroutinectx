package checkers

import (
	"fmt"
	"go/ast"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/checker"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/env"
	"github.com/mpyw/goroutinectx/internal/flow"
	"github.com/mpyw/goroutinectx/internal/funcspec"
	"github.com/mpyw/goroutinectx/internal/registry"
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
func (c *GotaskChecker) CheckCall(e *env.Env, _ string, call *ast.CallExpr) []analysis.Diagnostic {
	if c.derivers.IsEmpty() {
		return nil
	}

	fn, recvArgs := spawnedCallee(e, call)
	if fn == nil {
		return nil
	}

	entry, ok := registry.MatchFor(checker.Gotask, fn)
	if !ok {
		return nil
	}

	switch {
	case entry.AlwaysSpawns:
		return c.checkDoAsync(e, call, entry, recvArgs)
	case entry.Variadic:
		// For variadic APIs, we report each failing argument separately
		return c.checkVariadic(e, call, entry)
	}
	return nil
}

// checkDoAsync checks a DoAsync call. recvArgs is 1 for a method
// expression, whose first argument is the task.
func (c *GotaskChecker) checkDoAsync(e *env.Env, call *ast.CallExpr, entry registry.Entry, recvArgs int) []analysis.Diagnostic {
	instr := e.SSA.CallAt(call)
	if len(call.Args) <= recvArgs || instr == nil {
		return nil
	}
	common := instr.Common()
	args := flow.ResolvedArgs(common, call)

	// Check 1: Is the ctx argument (first arg) a deriver call?
	if args != nil && c.ctxIsDerived(args[recvArgs]) {
		return nil
	}

	// Check 2: Does the task's callback call the deriver?
	recv := gotaskReceiver(common)
	if recvArgs == 1 && args != nil {
		recv = args[0]
	}
	if recv != nil && c.taskDerives(e, recv) {
		return nil
	}

	// Neither condition satisfied - report at the method name.
	pos := call.Pos()
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		pos = sel.Sel.Pos()
	}
	return []analysis.Diagnostic{{Pos: pos, Message: entry.Spec.FullName() + "() 1st argument should call goroutine deriver"}}
}

func (c *GotaskChecker) checkVariadic(e *env.Env, call *ast.CallExpr, entry registry.Entry) []analysis.Diagnostic {
	startIdx := entry.CallbackArgIdx
	if startIdx >= len(call.Args) {
		return nil
	}

	// Check if this is a variadic expansion (e.g., DoAllFns(ctx, slice...))
	isVariadicExpansion := call.Ellipsis.IsValid()

	args := spawnedCallArgs(e, call)
	r := c.resolver(e)
	var diags []analysis.Diagnostic
	for i := startIdx; i < len(call.Args); i++ {
		var passes bool
		var v ssa.Value
		if args != nil {
			v = args[i]
		}
		if isVariadicExpansion {
			passes = c.expandedArgDerives(e, call.Args[i], v, r)
		} else {
			passes = spawnedFuncPasses(e, call.Args[i], v, r)
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
		diags = append(diags, analysis.Diagnostic{Pos: call.Args[i].Pos(), Message: msg})
	}
	return diags
}

// expandedArgDerives judges the slice of a variadic expansion, v being its
// SSA value. The elements of a slice literal, of gotask.TasksFrom, or of the
// slices a function returns are judged one by one. Otherwise a slice made by
// a call, as lo.Map makes one, passes when calling one of the call's
// callbacks gives a task function that derives. A slice variable whose
// elements are not traced, as a parameter, is reported.
func (c *GotaskChecker) expandedArgDerives(e *env.Env, arg ast.Expr, v ssa.Value, r *flow.FuncResolver) bool {
	if elems, ok := gotaskSliceElems(v); ok {
		return !slices.ContainsFunc(elems, func(e ssa.Value) bool { return !r.Passes(e) })
	}
	switch arg := ast.Unparen(arg).(type) {
	case *ast.Ident:
		obj, ok := e.Pass.TypesInfo.ObjectOf(arg).(*types.Var)
		if !ok {
			return true
		}
		_, isSlice := obj.Type().Underlying().(*types.Slice)
		return !isSlice
	case *ast.CallExpr:
		call, ok := v.(*ssa.Call)
		if !ok {
			return true
		}
		return slices.ContainsFunc(call.Call.Args, func(arg ssa.Value) bool {
			_, isFunc := arg.Type().Underlying().(*types.Signature)
			return isFunc && r.ResultPasses(arg)
		})
	}
	return true
}

// resolver judges a task function by whether it calls the deriver at its
// start. A task built by NewTask, or by a method of a task, is judged by the
// function it wraps. A task built by Parallel, ParallelSettled or Race
// passes when every task it is given does, and one built by Then or Catch
// when its task or its function does.
func (c *GotaskChecker) resolver(e *env.Env) *flow.FuncResolver {
	r := &flow.FuncResolver{
		Closure: func(fn *ssa.Function) bool { return flow.ClosureCallsDeriver(fn, c.derivers).FoundAtStart },
	}
	r.Call = func(call *ssa.CallCommon) (bool, bool) {
		if call.IsInvoke() {
			// An interface method is not traced, so the task it gives is
			// reported.
			return false, true
		}
		fn := flow.ExtractCalledFunc(call)
		switch {
		case fn == nil:
			return false, false
		case gotaskIsConstructor(fn):
			return len(call.Args) > 0 && r.Passes(call.Args[0]), true
		case gotaskIsTaskMethod(fn):
			return c.taskDerives(e, gotaskReceiver(call)), true
		case gotaskIsCombinator(fn, gotaskSpawningCombinators):
			return gotaskSpawningDerives(call, r), true
		case gotaskIsCombinator(fn, gotaskSerialCombinators):
			return gotaskSerialDerives(call, r), true
		}
		return false, false
	}
	return r
}

// gotaskSpawningCombinators are the gotask functions that build a task
// running each task they are given in a goroutine of its own.
var gotaskSpawningCombinators = []string{"Parallel", "ParallelSettled", "Race"}

// gotaskSerialCombinators are the gotask functions that build a task running
// the task and the function they are given one after the other, in one
// goroutine.
var gotaskSerialCombinators = []string{"Then", "Catch"}

// gotaskIsCombinator reports whether fn is one of names in the gotask
// package.
func gotaskIsCombinator(fn *types.Func, names []string) bool {
	return slices.ContainsFunc(names, func(name string) bool {
		return funcspec.Spec{PkgPath: gotaskConstructor.pkgPath, FuncName: name}.Matches(fn)
	})
}

// gotaskSpawningDerives reports whether every task a spawning combinator call
// is given derives, as each runs in a goroutine of its own. The tasks come in
// its variadic slice. A slice whose elements are not traced, as a parameter,
// passes, as a task the resolver can't follow does.
func gotaskSpawningDerives(call *ssa.CallCommon, r *flow.FuncResolver) bool {
	return !slices.ContainsFunc(call.Args, func(arg ssa.Value) bool {
		elems, ok := gotaskSliceElems(arg)
		return ok && slices.ContainsFunc(elems, func(e ssa.Value) bool { return !r.Passes(e) })
	})
}

// gotaskSerialDerives reports whether the task or the function a serial
// combinator call is given derives. Both run in one goroutine, so one call
// of the deriver is enough, as it is in a go statement.
func gotaskSerialDerives(call *ssa.CallCommon, r *flow.FuncResolver) bool {
	return slices.ContainsFunc(call.Args, r.Passes)
}

// ctxIsDerived reports whether every value v can be is a call of the
// deriver, or a context the context package's With functions, as
// context.WithCancel, make from one.
func (c *GotaskChecker) ctxIsDerived(v ssa.Value) bool {
	return c.ctxIsDerivedFrom(v, make(map[ssa.Value]bool))
}

func (c *GotaskChecker) ctxIsDerivedFrom(v ssa.Value, seen map[ssa.Value]bool) bool {
	if seen[v] {
		return false
	}
	seen[v] = true
	values, ok := flow.ResolvedValues(v)
	if !ok || len(values) == 0 {
		return false
	}
	return !slices.ContainsFunc(values, func(v ssa.Value) bool {
		if ext, ok := v.(*ssa.Extract); ok {
			v = ext.Tuple
		}
		call, ok := v.(*ssa.Call)
		if !ok {
			return true
		}
		fn := flow.ExtractCalledFunc(call.Common())
		switch {
		case fn == nil:
			return true
		case c.derivers.MatchesFunc(fn):
			return false
		case gotaskIsContextWith(fn) && len(call.Call.Args) > 0:
			return !c.ctxIsDerivedFrom(call.Call.Args[0], seen)
		}
		return true
	})
}

// gotaskIsContextWith reports whether fn is a function of the context
// package that makes a context from the one it is given first, as
// context.WithCancel and context.WithValue do.
func gotaskIsContextWith(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	return ok && fn.Pkg() != nil && fn.Pkg().Path() == "context" && strings.HasPrefix(fn.Name(), "With") &&
		sig.Recv() == nil && sig.Params().Len() > 0 && typeutil.IsContextType(sig.Params().At(0).Type())
}

// taskDerives reports whether every task v can be wraps a function that
// calls the deriver. A task traced to nothing does not.
func (c *GotaskChecker) taskDerives(e *env.Env, v ssa.Value) bool {
	if v == nil {
		return false
	}
	values, ok := flow.ResolvedValues(v)
	if !ok || len(values) == 0 {
		return false
	}
	r := c.resolver(e)
	return !slices.ContainsFunc(values, func(v ssa.Value) bool {
		// DoAsync of a CancelableTask runs the Task it embeds.
		if f, ok := v.(*ssa.Field); ok && gotaskIsTaskType(f.X.Type()) {
			return !c.taskDerives(e, f.X)
		}
		call, ok := v.(*ssa.Call)
		if !ok {
			return true
		}
		passes, handled := r.Call(call.Common())
		return !handled || !passes
	})
}

// gotaskSliceElems gives the elements of a slice of tasks or task functions,
// as flow.ResolvedSliceElems does, and for gotask.TasksFrom, the functions it
// is given.
func gotaskSliceElems(v ssa.Value) ([]ssa.Value, bool) {
	if call, ok := v.(*ssa.Call); ok {
		fn := flow.ExtractCalledFunc(call.Common())
		if fn != nil && (funcspec.Spec{PkgPath: gotaskConstructor.pkgPath, FuncName: "TasksFrom"}).Matches(fn) {
			if len(call.Call.Args) != 1 {
				return nil, false
			}
			return flow.ResolvedSliceElems(call.Call.Args[0])
		}
	}
	if v == nil {
		return nil, false
	}
	return flow.ResolvedSliceElems(v)
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

// gotaskReceiver gives the receiver of a static method call, or of a call
// of a method value, which a bound method wrapper holds. It gives nil for
// any other call.
func gotaskReceiver(common *ssa.CallCommon) ssa.Value {
	if common.IsInvoke() {
		return nil
	}
	if mc, ok := common.Value.(*ssa.MakeClosure); ok {
		if fn, ok := mc.Fn.(*ssa.Function); ok && flow.Classify(fn) == flow.ClassifiedBound && len(mc.Bindings) == 1 {
			return mc.Bindings[0]
		}
		return nil
	}
	if common.Signature().Recv() == nil || len(common.Args) == 0 {
		return nil
	}
	return common.Args[0]
}
