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

// expandedArgDerives judges the slice of a variadic expansion, arg, as
// gotaskSliceDerives judges its SSA value. A slice SSA has no value for
// passes, as nothing is known of it.
func (c *GotaskChecker) expandedArgDerives(e *env.Env, spawned site.Spawned, p *gotaskPolicy) bool {
	if spawned.Value == nil {
		return true
	}
	return gotaskSliceDerives(e, spawned.Value, p, map[*ssa.Phi]bool{})
}

// gotaskSliceDerives judges a slice of tasks or task functions, v. The
// elements of a slice literal, of gotask.TasksFrom, or of the slices a
// function returns are judged one by one. A slice a closure captures is
// judged by the slices stored to it, each as this judges it (#304, #320).
// Otherwise a slice made by a call, as lo.Map
// makes one, passes when calling one of the call's callbacks gives a task
// function that derives, whether the call is written in place or held in a
// variable, or when the call has no callback and its callee cannot be
// followed (#301). A slice a variable holds on several paths, a Phi, passes
// when each slice it may hold does. Any other slice, as a parameter, passes,
// as nothing is known of its elements, the same as a func value that cannot
// be followed (#316).
func gotaskSliceDerives(e *env.Env, v ssa.Value, p *gotaskPolicy, seen map[*ssa.Phi]bool) bool {
	if elems, ok := gotaskSliceElems(e, v); ok {
		return !slices.ContainsFunc(elems, func(elem ssa.Value) bool { return !p.passes(elem) })
	}
	if fv, captured := gotaskCapturedRead(v); captured {
		return gotaskCapturedSliceDerives(e, fv, p, seen)
	}
	switch x := v.(type) {
	case *ssa.Call:
		// The SSA value is judged, not the form written, so a slice a
		// variable holds is judged as the call that made it (#270).
		return gotaskCallSliceDerives(e, x, p)
	case *ssa.Phi:
		if seen[x] {
			return true
		}
		seen[x] = true
		return !slices.ContainsFunc(x.Edges, func(edge ssa.Value) bool { return !gotaskSliceDerives(e, edge, p, seen) })
	}
	return true
}

// gotaskCapturedSliceDerives judges the slice a closure reads from fv, whose
// elements are not traced, by the slices stored to it, each as
// gotaskSliceDerives judges it, as the edges of a Phi are: one a call makes
// is judged as the call (#304), and one known not to derive is reported
// (#320). When they cannot all be followed, it passes, as nothing is known
// of what the closure reads (#263).
func gotaskCapturedSliceDerives(e *env.Env, fv *ssa.FreeVar, p *gotaskPolicy, seen map[*ssa.Phi]bool) bool {
	sources, complete := e.Flow.CapturedSources(fv)
	return !complete || !slices.ContainsFunc(sources, func(s flow.Source) bool {
		return !gotaskSliceDerives(e, s.Value, p, seen)
	})
}

// gotaskCallSliceDerives judges a slice call makes, whose elements are not
// traced. It passes when calling one of the call's callbacks gives a task
// function that derives, as lo.Map makes one. A call given no callback
// passes when its callee cannot be followed, as a func variable, a
// parameter, an interface method or a function with no body, as a factory
// that cannot be followed does (#301).
func gotaskCallSliceDerives(e *env.Env, call *ssa.Call, p *gotaskPolicy) bool {
	given := false
	for _, arg := range call.Call.Args {
		if _, isFunc := arg.Type().Underlying().(*types.Signature); !isFunc {
			continue
		}
		given = true
		if flow.JudgeResult(e.Flow, p, arg).Passes() {
			return true
		}
	}
	return !given && !gotaskCalleeFollowed(e, call.Common())
}

// gotaskCalleeFollowed reports whether every function call can call is
// known and has a body: not an interface method, a func value that cannot
// be followed, or a function with no body.
func gotaskCalleeFollowed(e *env.Env, call *ssa.CallCommon) bool {
	if call.IsInvoke() {
		return false
	}
	callees, complete := e.Flow.Sources(call.Value)
	return complete && len(callees) > 0 && !slices.ContainsFunc(callees, func(s flow.Source) bool {
		fn := s.Func()
		if fn == nil {
			return true
		}
		if fn.Origin() != nil {
			// An instantiation of a generic function has no body of its own.
			fn = fn.Origin()
		}
		return fn.Blocks == nil
	})
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
// calls the deriver, as Call judges it, or as factoryDerives judges a task
// another function returns. A task traced to nothing does not.
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
		call := s.Call()
		if call == nil {
			return true
		}
		v := p.Call(call.Common())
		if v == flow.Unknown {
			return !p.factoryDerives(call.Common(), gotaskResultIndex(s.Value))
		}
		return !v.Passes()
	})
	p.judged[v] = derives
	return derives
}

// gotaskResultIndex gives which result of its call v is: the index of an
// extract of a tuple, as t of t, err := mk() (#290), or 0.
func gotaskResultIndex(v ssa.Value) int {
	if x, ok := v.(*ssa.Extract); ok {
		return x.Index
	}
	return 0
}

// factoryDerives reports whether every task a call of a factory, a function
// that is not of gotask, can give as its result at index derives, as
// taskDerives judges each value the factory returns there (#287, #290).
// What cannot be followed passes, as a func value of a parameter or a
// function with no body does.
func (p *gotaskPolicy) factoryDerives(call *ssa.CallCommon, index int) bool {
	callees, complete := p.e.Flow.Sources(call.Value)
	if !complete {
		return true
	}
	return !slices.ContainsFunc(callees, func(s flow.Source) bool {
		fn := s.Func()
		if fn == nil {
			return false
		}
		if fn.Origin() != nil {
			// An instantiation of a generic factory has no body of its own.
			fn = fn.Origin()
		}
		return slices.ContainsFunc(p.e.Flow.ReturnsAt(fn, index), func(r flow.Return) bool {
			return !p.taskDerives(r.Value)
		})
	})
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
// [flow.Flow.CapturedSources] reads it (#263). Each of those is in a
// function around the closure, so the reads end. ok is false when any of
// them is not traced.
func gotaskSliceElems(e *env.Env, v ssa.Value) ([]ssa.Value, bool) {
	if fv, ok := gotaskCapturedRead(v); ok {
		sources, complete := e.Flow.CapturedSources(fv)
		if !complete || len(sources) == 0 {
			return nil, false
		}
		var elems []ssa.Value
		for _, s := range sources {
			sourceElems, ok := gotaskSliceElems(e, s.Value)
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
			return gotaskSliceElems(e, call.Call.Args[0])
		}
	}
	return e.Flow.ResolvedSliceElems(v)
}

// gotaskCapturedRead gives the free variable v reads, when v is a free
// variable captured by value or a load of one captured by reference.
func gotaskCapturedRead(v ssa.Value) (*ssa.FreeVar, bool) {
	if load, ok := v.(*ssa.UnOp); ok && load.Op == token.MUL {
		v = load.X
	}
	fv, ok := v.(*ssa.FreeVar)
	return fv, ok
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
