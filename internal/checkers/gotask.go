package checkers

import (
	"fmt"
	"go/ast"
	"go/token"
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
	"github.com/mpyw/goroutinectx/internal/site"
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

// CheckCall checks a call to a gotask API. A variadic API may give a report
// for each argument.
func (c *GotaskChecker) CheckCall(e *env.Env, _ string, s *site.Site) []analysis.Diagnostic {
	if c.derivers.IsEmpty() || s.Callee == nil {
		return nil
	}

	entry, ok := registry.MatchFor(checkername.Gotask, s.Callee)
	if !ok {
		return nil
	}

	switch {
	case entry.AlwaysSpawns:
		return c.checkDoAsync(e, s, entry)
	case entry.Variadic:
		// For variadic APIs, we report each failing argument separately
		return c.checkVariadic(e, s, entry)
	}
	return nil
}

// checkDoAsync checks a DoAsync call, whose receiver is the task.
func (c *GotaskChecker) checkDoAsync(e *env.Env, s *site.Site, entry registry.Entry) []analysis.Diagnostic {
	params := s.Params()
	if len(params) == 0 || s.Instr == nil {
		return nil
	}

	// Check 1: Is the ctx argument (first arg) a deriver call?
	if ctx := params[0].Value; ctx != nil && c.ctxIsDerived(e, ctx) {
		return nil
	}

	// Check 2: Does the task's callback call the deriver?
	if s.Recv != nil && c.policy(e).taskDerives(s.Recv) {
		return nil
	}

	// Neither condition satisfied - report at the method name.
	pos := s.Call.Pos()
	if sel, ok := s.Call.Fun.(*ast.SelectorExpr); ok {
		pos = sel.Sel.Pos()
	}
	return []analysis.Diagnostic{{Pos: pos, Message: fmt.Sprintf(messageTaskCtx, entry.Spec.FullName())}}
}

func (c *GotaskChecker) checkVariadic(e *env.Env, s *site.Site, entry registry.Entry) []analysis.Diagnostic {
	if entry.CallbackArgIdx >= len(s.Args) {
		return nil
	}

	// Check if this is a variadic expansion (e.g., DoAllFns(ctx, slice...))
	isVariadicExpansion := s.Call.Ellipsis.IsValid()

	p := c.policy(e)
	var diags []analysis.Diagnostic
	for _, arg := range s.Args[entry.CallbackArgIdx:] {
		var passes bool
		if isVariadicExpansion {
			passes = c.expandedArgDerives(e, arg, p)
		} else {
			passes = e.Index.Judge(e.Flow, p, arg).Passes()
		}
		if passes {
			continue
		}

		var msg string
		if isVariadicExpansion {
			msg = fmt.Sprintf(messageTaskVariadic, entry.Spec.FullName())
		} else {
			msg = fmt.Sprintf(messageTaskArg, entry.Spec.FullName(), messageOrdinal(arg.Ordinal))
		}
		diags = append(diags, analysis.Diagnostic{Pos: arg.Expr.Pos(), Message: msg})
	}
	return diags
}

// expandedArgDerives judges the slice of a variadic expansion, arg. The elements of a slice literal, of gotask.TasksFrom, or of the
// slices a function returns are judged one by one. Otherwise a slice made by
// a call, as lo.Map makes one, passes when calling one of the call's
// callbacks gives a task function that derives. A slice variable whose
// elements are not traced, as a parameter, is reported. A slice SSA has no
// value for passes, as nothing is known of it. A slice a closure captures is
// judged by the slices stored to it, and passes when they are not traced.
func (c *GotaskChecker) expandedArgDerives(e *env.Env, spawned site.Spawned, p *gotaskPolicy) bool {
	v := spawned.Value
	if v == nil {
		return true
	}
	if elems, ok := gotaskSliceElems(e, v); ok {
		return !slices.ContainsFunc(elems, func(elem ssa.Value) bool { return !p.passes(elem) })
	}
	if _, captured := gotaskCapturedRead(v); captured {
		// A captured slice that is not traced passes, as nothing is known
		// of what the closure reads (#263).
		return true
	}
	switch arg := ast.Unparen(spawned.Expr).(type) {
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
		return p.verdictOf(p.taskDerives(site.Receiver(call)))
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
// functions it is given. A slice read from a variable a closure captures
// gives the elements of every slice stored to it, as
// [flow.Flow.CapturedSources] reads it (#263). ok is false when any of them
// is not traced.
func gotaskSliceElems(e *env.Env, v ssa.Value) ([]ssa.Value, bool) {
	return gotaskSliceElemsOf(e, v, make(map[ssa.Value]bool))
}

func gotaskSliceElemsOf(e *env.Env, v ssa.Value, seen map[ssa.Value]bool) ([]ssa.Value, bool) {
	if v == nil || seen[v] {
		return nil, false
	}
	seen[v] = true
	if read, ok := gotaskCapturedRead(v); ok {
		sources, complete := e.Flow.CapturedSources(read)
		if !complete || len(sources) == 0 {
			return nil, false
		}
		var elems []ssa.Value
		for _, s := range sources {
			sourceElems, ok := gotaskSliceElemsOf(e, s.Value, seen)
			if !ok {
				return nil, false
			}
			elems = append(elems, sourceElems...)
		}
		return elems, true
	}
	if call, ok := v.(*ssa.Call); ok {
		fn := flow.ExtractCalledFunc(call.Common())
		if fn != nil && (funcspec.Spec{PkgPath: gotaskConstructor.pkgPath, FuncName: "TasksFrom"}).Matches(fn) {
			if len(call.Call.Args) != 1 {
				return nil, false
			}
			return gotaskSliceElemsOf(e, call.Call.Args[0], seen)
		}
	}
	return e.Flow.ResolvedSliceElems(v)
}

// gotaskCapturedRead gives the free variable v reads, when v is a free
// variable captured by value or a load of one captured by reference.
func gotaskCapturedRead(v ssa.Value) (*ssa.FreeVar, bool) {
	switch x := v.(type) {
	case *ssa.FreeVar:
		return x, true
	case *ssa.UnOp:
		if x.Op == token.MUL {
			fv, ok := x.X.(*ssa.FreeVar)
			return fv, ok
		}
	}
	return nil, false
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
