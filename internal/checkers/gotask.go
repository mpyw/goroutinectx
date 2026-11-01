package checkers

import (
	"fmt"
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/checkername"
	"github.com/mpyw/goroutinectx/internal/ctxkind"
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

// newGotaskChecker creates a gotask checker.
//
//declscope:shared // Defs in checkers.go builds the checker with it
func newGotaskChecker(derivers *deriver.Matcher) *GotaskChecker {
	return &GotaskChecker{derivers: derivers}
}

// Name returns the checker name.
func (*GotaskChecker) Name() checkername.Name {
	return checkername.Gotask
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

	entry, ok := registry.MatchFor(checkername.Gotask, fn)
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
	if args != nil && c.ctxIsDerived(e, args[recvArgs]) {
		return nil
	}

	// Check 2: Does the task's callback call the deriver?
	recv := gotaskReceiver(common)
	if recvArgs == 1 && args != nil {
		recv = args[0]
	}
	if recv != nil && c.policy(e).taskDerives(recv) {
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
	p := c.policy(e)
	var diags []analysis.Diagnostic
	for i := startIdx; i < len(call.Args); i++ {
		var passes bool
		var v ssa.Value
		if args != nil {
			v = args[i]
		}
		if isVariadicExpansion {
			passes = c.expandedArgDerives(e, call.Args[i], v, p)
		} else {
			passes = spawnedFuncVerdict(e, call.Args[i], v, p).Passes()
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
func (c *GotaskChecker) expandedArgDerives(e *env.Env, arg ast.Expr, v ssa.Value, p *gotaskPolicy) bool {
	if elems, ok := gotaskSliceElems(e, v); ok {
		return !slices.ContainsFunc(elems, func(elem ssa.Value) bool { return !p.passes(elem) })
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
			return isFunc && flow.JudgeResult(e.Flow, p, arg).Passes()
		})
	}
	return true
}

// policy gives the policy of the task functions of one call.
func (c *GotaskChecker) policy(e *env.Env) *gotaskPolicy {
	return &gotaskPolicy{e: e, derivers: c.derivers, judged: make(map[ssa.Value]bool)}
}

// gotaskPolicy judges a task function by whether it calls the deriver at
// its start. A task built by NewTask, or by a method of a task, is judged by
// the function it wraps. A task built by Parallel, ParallelSettled or Race
// passes when every task it is given does, and one built by Then or Catch
// when its task or its function does.
type gotaskPolicy struct {
	e        *env.Env
	derivers *deriver.Matcher
	// judged are the tasks judged by taskDerives, each by its verdict.
	judged map[ssa.Value]bool
}

// passes reports whether every function v can hold passes.
func (p *gotaskPolicy) passes(v ssa.Value) bool {
	return flow.Judge(p.e.Flow, p, v).Passes()
}

// verdictOf gives the verdict of passes.
func (*gotaskPolicy) verdictOf(passes bool) flow.Verdict {
	if passes {
		return flow.Pass
	}
	return flow.Fail
}

// Literal judges a closure by whether it calls the deriver at its start.
func (p *gotaskPolicy) Literal(fn *ssa.Function, _ flow.Site) flow.Verdict {
	return p.verdictOf(flow.ClosureCallsDeriver(fn, p.derivers).FoundAtStart)
}

// Call judges a task a gotask function or method gives, as
// [gotaskPolicy] tells, and fails the result of an interface method.
func (p *gotaskPolicy) Call(call *ssa.CallCommon) flow.Verdict {
	if call.IsInvoke() {
		// An interface method is not traced, so the task it gives is
		// reported.
		return flow.Fail
	}
	fn := flow.ExtractCalledFunc(call)
	switch {
	case fn == nil:
		return flow.Unknown
	case gotaskIsConstructor(fn):
		return p.verdictOf(len(call.Args) > 0 && p.passes(call.Args[0]))
	case gotaskIsTaskMethod(fn):
		return p.verdictOf(p.taskDerives(gotaskReceiver(call)))
	case gotaskIsCombinator(fn, gotaskSpawningCombinators):
		return p.verdictOf(gotaskSpawningDerives(p.e, call, p))
	case gotaskIsCombinator(fn, gotaskSerialCombinators):
		return p.verdictOf(gotaskSerialDerives(call, p))
	}
	return flow.Unknown
}

// FactoryCall follows every factory a call calls.
func (*gotaskPolicy) FactoryCall(*ssa.CallCommon) bool {
	return false
}

// Factory follows what every factory returns.
func (*gotaskPolicy) Factory(*ssa.Function) bool {
	return false
}

// taskDerives reports whether every task v can be wraps a function that
// calls the deriver, as Call judges it. A task traced to nothing does not.
// DoAsync of a CancelableTask runs the Task it embeds. Each task is judged
// once, and a task made from itself, as t = t.Cancelable() in a loop, by
// what else it can be.
func (p *gotaskPolicy) taskDerives(v ssa.Value) bool {
	if v == nil {
		return false
	}
	if derives, ok := p.judged[v]; ok {
		return derives
	}
	p.judged[v] = true
	sources, complete := p.e.Flow.Sources(v)
	derives := complete && len(sources) > 0 && !slices.ContainsFunc(sources, func(s flow.Source) bool {
		if f, ok := s.Value.(*ssa.Field); ok && gotaskIsTaskType(f.X.Type()) {
			return !p.taskDerives(f.X)
		}
		call, ok := s.Value.(*ssa.Call)
		if !ok {
			return true
		}
		v := p.Call(call.Common())
		return v == flow.Unknown || !v.Passes()
	})
	p.judged[v] = derives
	return derives
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
func gotaskSpawningDerives(e *env.Env, call *ssa.CallCommon, p *gotaskPolicy) bool {
	return !slices.ContainsFunc(call.Args, func(arg ssa.Value) bool {
		elems, ok := gotaskSliceElems(e, arg)
		return ok && slices.ContainsFunc(elems, func(elem ssa.Value) bool { return !p.passes(elem) })
	})
}

// gotaskSerialDerives reports whether the task or the function a serial
// combinator call is given derives. Both run in one goroutine, so one call
// of the deriver is enough, as it is in a go statement.
func gotaskSerialDerives(call *ssa.CallCommon, p *gotaskPolicy) bool {
	return slices.ContainsFunc(call.Args, p.passes)
}

// ctxIsDerived reports whether every source of v is a call of the
// deriver, or a context the context package's With functions, as
// context.WithCancel, make from one.
func (c *GotaskChecker) ctxIsDerived(e *env.Env, v ssa.Value) bool {
	return c.ctxIsDerivedFrom(e, v, make(map[ssa.Value]bool))
}

func (c *GotaskChecker) ctxIsDerivedFrom(e *env.Env, v ssa.Value, seen map[ssa.Value]bool) bool {
	if seen[v] {
		return false
	}
	seen[v] = true
	sources, complete := e.Flow.Sources(v)
	if !complete || len(sources) == 0 {
		return false
	}
	return !slices.ContainsFunc(sources, func(s flow.Source) bool {
		call := s.Call()
		if call == nil {
			return true
		}
		fn := flow.ExtractCalledFunc(call.Common())
		switch {
		case fn == nil:
			return true
		case c.derivers.MatchesFunc(fn):
			return false
		case ctxkind.IsWithCall(fn) && len(call.Call.Args) > 0:
			return !c.ctxIsDerivedFrom(e, call.Call.Args[0], seen)
		}
		return true
	})
}

// gotaskSliceElems gives the elements of a slice of tasks or task functions,
// as [flow.Flow.ResolvedSliceElems] does, and for gotask.TasksFrom, the
// functions it is given.
func gotaskSliceElems(e *env.Env, v ssa.Value) ([]ssa.Value, bool) {
	if call, ok := v.(*ssa.Call); ok {
		fn := flow.ExtractCalledFunc(call.Common())
		if fn != nil && (funcspec.Spec{PkgPath: gotaskConstructor.pkgPath, FuncName: "TasksFrom"}).Matches(fn) {
			if len(call.Call.Args) != 1 {
				return nil, false
			}
			return e.Flow.ResolvedSliceElems(call.Call.Args[0])
		}
	}
	if v == nil {
		return nil, false
	}
	return e.Flow.ResolvedSliceElems(v)
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
