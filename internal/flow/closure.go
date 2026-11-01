package flow

import (
	"go/token"
	"go/types"
	"math"
	"slices"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/ctxkind"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/funcspec"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// ClosureCapturesContext checks if a closure captures any context.Context variable
// or a configured carrier type. A variable that only ever holds a fresh
// context.Background() or TODO() does not count, a carrier that is a context
// too included. A value whose type implements context.Context counts as a
// context, judged the same. A context the closure
// reads from a field or an element of a variable it captures counts, unless
// it is fresh: an element of a slice, a map or a channel, as m[k], <-ch or
// a range over one gives, and a value a type assertion gives, too (#253). So does a method called on a captured value that returns a
// context, and a call that takes a captured value that holds a context the
// analyzed code can have put there ([ctxkind.Holds]), as its receiver or as
// any argument, as a function or method spawned directly with it is left
// alone. A value of an interface type counts when one of the values it can
// be holds one, or one cannot be followed (#240). A captured closure that
// captures a context counts too.
//
// A func value the closure calls or passes on, read from a variable it
// captures or from a part of one, as handlers[k]() or a range over a
// channel of funcs reads it, counts when funcs passes it at
// [SiteCaptured], as it would pass the same value spawned directly: one
// that cannot be followed passes, as a parameter, a field of a parameter,
// an element at an index that is not constant or a result of a call with
// several results. funcs may be nil, and then only a captured closure that
// captures a context counts. The verdict is remembered by the closure,
// kinds and funcs, so funcs must judge the same each time it is asked.
func (f *Flow) ClosureCapturesContext(closure *ssa.Function, kinds *ctxkind.Classifier, funcs Policy) bool {
	defer f.ask()()
	return closureCaptures(f, closure, kinds, funcs, make(map[*ssa.Function]bool))
}

func closureCaptures(f *Flow, closure *ssa.Function, kinds *ctxkind.Classifier, funcs Policy, seen map[*ssa.Function]bool) bool {
	if closure == nil {
		return false
	}
	key := closureVerdictKey{closure: closure, kinds: kinds, funcs: funcs}
	if depth, onStack := f.closures[closure]; onStack {
		f.closureLow = min(f.closureLow, depth)
		return false
	}
	free := !closureFactoriesCalled(f)
	if seen[closure] {
		// Judged before in this question, and found false, or the question
		// would have ended. That is the verdict when it was remembered.
		if captures, ok := f.captures[key]; !ok || captures || !free {
			f.closureLow = 0
		}
		return false
	}
	if captures, ok := f.captures[key]; ok && free {
		return captures
	}
	seen[closure] = true

	depth := len(f.closures) + 1
	low, cuts := f.closureLow, f.cuts
	if depth == 1 {
		low = closureUnheld
	}
	f.closureLow = closureUnheld
	f.closures[closure] = depth
	captures := closureCapturesFreeVar(f, closure, kinds, funcs, seen) || closureRunsPassingResult(f, closure, funcs)
	delete(f.closures, closure)

	// A verdict cut only at this closure itself, with nothing outside it
	// changing what a parameter passes, is the same wherever it is asked,
	// unless the bound on the work cut it short.
	if f.closureLow >= depth {
		if free && f.cuts == cuts {
			f.captures[key] = captures
		}
		f.closureLow = low
	} else {
		f.closureLow = min(low, f.closureLow)
	}
	return captures
}

// closureVerdictKey is a closure and what [Flow.ClosureCapturesContext]
// judges it with.
//
//declscope:shared // flow.go holds the verdicts
type closureVerdictKey struct {
	closure *ssa.Function       //declscope:private
	kinds   *ctxkind.Classifier //declscope:private
	funcs   Policy              //declscope:private
}

// closureUnheld is the closureLow of a verdict cut at no closure.
const closureUnheld = math.MaxInt

// closureFactoriesCalled reports whether a judgement is following the
// result of a factory, up the stack, so that a parameter of it does not
// pass.
func closureFactoriesCalled(f *Flow) bool {
	for _, calls := range f.called {
		if len(calls) > 0 {
			return true
		}
	}
	return false
}

// closureCapturesFreeVar reports whether one of the variables closure
// captures gives it a context, as [Flow.ClosureCapturesContext] judges it.
func closureCapturesFreeVar(f *Flow, closure *ssa.Function, kinds *ctxkind.Classifier, funcs Policy, seen map[*ssa.Function]bool) bool {
	r := &closureReader{
		flow:  f,
		kinds: kinds.From(closurePackage(closure)),
		funcs: funcs,
		seen:  make(map[ssa.Value]bool),
	}
	return slices.ContainsFunc(closure.FreeVars, func(fv *ssa.FreeVar) bool {
		switch k := r.kinds.Kind(fv.Type()); {
		case k&(ctxkind.Context|ctxkind.Implements) != 0:
			// A carrier that is a context too is judged as a context, as
			// carries judges a part.
			return !closureIsFresh(f, fv)
		case k&ctxkind.Carrier != 0:
			return true
		}
		r.root = fv
		return r.reads(fv, false) || closureHoldsCapturing(f, fv, kinds, funcs, seen) || r.runsPassing(fv) || r.waitsDone(fv) || r.madeWithContext(fv)
	})
}

// closureRunsPassingResult reports whether closure, or a function it
// runs in place ([Runs]), calls the result of a call in place, as
// runFromChan(ch)() does, and funcs passes it at [SiteCaptured], as it
// would pass go runFromChan(ch)() (#300): a factory given a context, or a
// value that holds one, passes, and what it returns is followed. funcs
// may be nil, and then none counts. The iterator of a range-over-func loop
// is left out, as Runs goes into its body.
func closureRunsPassingResult(f *Flow, closure *ssa.Function, funcs Policy) bool {
	if funcs == nil {
		return false
	}
	for site := range Runs(closure, RunOptions{}) {
		if site.isGo || site.Call.IsInvoke() || runRangeBody(site.Call) != nil {
			continue
		}
		if inner, ok := site.Call.Value.(*ssa.Call); ok && judgeCaptured(f, funcs, inner).Passes() {
			return true
		}
	}
	return false
}

// closureReader tells whether a closure reads a context from the values it
// captures.
type closureReader struct {
	// flow follows the parts read.
	flow *Flow
	// kinds judges Holds for the closure's package.
	kinds *ctxkind.Classifier
	// funcs judges a func value the closure runs, or nil.
	funcs Policy
	// root is the captured variable the parts read are of.
	root ssa.Value
	seen map[ssa.Value]bool
}

// reads reports whether a context or a carrier is read from v, through the
// fields and elements of what it holds, slices, maps and channels alike,
// the values it asserts to and the local variables it is copied to, or a
// method is called on a value that returns one, or a call takes a value
// that holds one. A closure made inside that captures v counts when it
// uses v so (#268). A context that is only ever fresh does not count; one
// read from a map, a channel or a type assertion cannot be followed, so it
// counts.
//
// opaque tells that v is read from a map, a channel or a type assertion,
// which cannot be followed to what was put there.
func (r *closureReader) reads(v ssa.Value, opaque bool) bool {
	if r.seen[v] {
		return false
	}
	r.seen[v] = true
	for _, ref := range *v.Referrers() {
		var part ssa.Value
		partOpaque := opaque
		switch x := ref.(type) {
		case ssa.CallInstruction:
			if r.callsContextMethod(x.Common(), v) || r.callsWithHolder(x.Common(), v, v) {
				return true
			}
			continue
		case *ssa.MakeClosure:
			// A closure inside, as a deferred recover handler, that
			// captures v in turn (#268).
			if r.innerUses(x, v, opaque) {
				return true
			}
			continue
		case *ssa.MakeInterface:
			if r.givesHolderAsInterface(x, v) {
				return true
			}
			continue
		case *ssa.ChangeInterface:
			if r.givesHolderAsInterface(x, v) {
				return true
			}
			continue
		case *ssa.Store:
			// A copy to a local variable, as c := *p makes, is read the same.
			a, ok := x.Addr.(*ssa.Alloc)
			if !ok || x.Val != v || !r.reads(a, opaque) {
				continue
			}
			return true
		case *ssa.UnOp:
			// A load, or a receive from a channel, as <-ch and a range over
			// a channel make.
			if x.Op != token.MUL && x.Op != token.ARROW {
				continue
			}
			part, partOpaque = x, opaque || x.Op == token.ARROW
		case *ssa.FieldAddr, *ssa.Field, *ssa.IndexAddr, *ssa.Index,
			// A result of a part with several, as v, ok := m[k] and a
			// range give.
			*ssa.Extract:
			part = x.(ssa.Value)
		case *ssa.Lookup, *ssa.Range, *ssa.Next, *ssa.TypeAssert:
			// An element of a map, and of a range over one, and a type
			// assertion (#253).
			part, partOpaque = x.(ssa.Value), true
		default:
			continue
		}
		if r.carries(part) || r.runsPassingPart(part, partOpaque) || r.waitsDone(part) || r.reads(part, partOpaque) {
			return true
		}
	}
	return false
}

// innerUses reports whether the closure inner makes, inside the closure
// judged, uses v it captures, as the closure judged would use v itself:
// v read, run passing, waited for as ctx.Done(), or given to a call when
// made with ctx.
func (r *closureReader) innerUses(inner *ssa.MakeClosure, v ssa.Value, opaque bool) bool {
	fn := inner.Fn.(*ssa.Function)
	for i, b := range inner.Bindings {
		if b != v {
			continue
		}
		fv := fn.FreeVars[i]
		if r.reads(fv, opaque) || r.runsPassing(fv) || r.waitsDone(fv) || r.madeWithContext(fv) {
			return true
		}
	}
	return false
}

// carries reports whether part, read from a captured value, is a carrier, or
// a context that is not only ever fresh, as a free variable is judged. A
// carrier that is a context too, or a type that implements one, is judged
// as a context. The address of a context is judged by the loads from it,
// unless it is used some other way.
func (r *closureReader) carries(part ssa.Value) bool {
	t := part.Type()
	if !r.kinds.Is(t, ctxkind.Context|ctxkind.Implements) {
		return r.kinds.Is(t, ctxkind.Carrier)
	}
	if _, isPtr := t.Underlying().(*types.Pointer); isPtr {
		return slices.ContainsFunc(*part.Referrers(), func(ref ssa.Instruction) bool {
			load, ok := ref.(*ssa.UnOp)
			return !ok || load.Op != token.MUL
		})
	}
	return !closureIsFresh(r.flow, part)
}

// callsContextMethod reports whether call is a method called on v, an
// interface method included, that returns a context or a carrier, as
// req.Context() does.
func (r *closureReader) callsContextMethod(call *ssa.CallCommon, v ssa.Value) bool {
	switch {
	case call.IsInvoke():
		if call.Value != v {
			return false
		}
	case call.Signature().Recv() == nil || len(call.Args) == 0 || call.Args[0] != v:
		return false
	}
	results := call.Signature().Results()
	for i := range results.Len() {
		if r.kinds.Is(results.At(i).Type(), ctxkind.Captured) {
			return true
		}
	}
	return false
}

// callsWithHolder reports whether call takes v, as the receiver of a method,
// an interface method included, or as any argument, and held, past its
// pointers, holds a context or a carrier that the analyzed code can have
// put there ([ctxkind.Holds]). held is v, or the value v converts to an
// interface. A held of an interface type other than a context or a carrier
// is judged by the values it can be, as [closureReader.interfaceMayHold]
// judges it. Whether the callee
// reads it is not looked into, as for a function or method spawned
// directly with the same value. A builtin, as append or len, does not
// count.
func (r *closureReader) callsWithHolder(call *ssa.CallCommon, v, held ssa.Value) bool {
	if _, isBuiltin := call.Value.(*ssa.Builtin); isBuiltin {
		return false
	}
	if !slices.Contains(call.Args, v) && (!call.IsInvoke() || call.Value != v) {
		return false
	}
	if types.IsInterface(held.Type()) {
		// A context or a carrier is judged where it is read, by carries.
		return !r.kinds.Is(held.Type(), ctxkind.Captured) && r.interfaceMayHold(held)
	}
	return r.kinds.Is(typeutil.UnwrapPointer(held.Type()), ctxkind.Holds)
}

// interfaceMayHold reports whether v, of an interface type and read from
// what the closure captures, may have a dynamic value that holds a context
// or a carrier (#240). Each value v can be, as [Flow.CapturedSources] and
// [Flow.Sources] give them, is judged by the conversion that makes it: a
// value of a concrete type counts when the type, past its pointers, is
// [ctxkind.Captured] or [ctxkind.Holds], as var r Runner = ctxRunner{ctx}
// is. A value of another interface type is judged the same way, a context
// counting unless it is fresh, and a nil interface holds nothing. A value
// that cannot be followed, as a parameter, a field of one or the result of
// a call, counts only when its interface type is declared in the analyzed
// module ([ctxkind.Classifier.DeclaredInModule]), as only there can the
// analyzed code have put a context. One of the standard library, as error,
// net.Conn or io.Reader, does not count.
func (r *closureReader) interfaceMayHold(v ssa.Value) bool {
	captured, complete := r.flow.CapturedSources(v)
	if !complete {
		return r.kinds.DeclaredInModule(v.Type())
	}
	return slices.ContainsFunc(captured, func(c Source) bool {
		sources, complete := r.flow.Sources(c.Value)
		if !complete {
			return r.kinds.DeclaredInModule(c.Value.Type())
		}
		return slices.ContainsFunc(sources, func(s Source) bool {
			switch x := s.Value.(type) {
			case *ssa.MakeInterface:
				return r.kinds.Is(typeutil.UnwrapPointer(x.X.Type()), ctxkind.Captured|ctxkind.Holds)
			case *ssa.ChangeInterface:
				if r.kinds.Is(x.X.Type(), ctxkind.Context) {
					return !r.flow.IsFreshContext(x.X)
				}
				return r.interfaceMayHold(x.X)
			case *ssa.Const:
				return false
			}
			return r.kinds.DeclaredInModule(s.Value.Type())
		})
	})
}

// givesHolderAsInterface reports whether conv, v converted to an interface,
// is given to a call as [closureReader.callsWithHolder] judges it.
func (r *closureReader) givesHolderAsInterface(conv ssa.Value, v ssa.Value) bool {
	return slices.ContainsFunc(*conv.Referrers(), func(ref ssa.Instruction) bool {
		call, ok := ref.(ssa.CallInstruction)
		return ok && r.callsWithHolder(call.Common(), conv, v)
	})
}

// closurePackage gives the package of closure, or nil when SSA has none, as
// for a wrapper.
func closurePackage(closure *ssa.Function) *types.Package {
	if closure.Pkg == nil {
		return nil
	}
	return closure.Pkg.Pkg
}

// runsPassing reports whether v, a func value read from what the closure
// captures, or a captured variable that holds one, is called or given to a
// call in the closure, and every function it can hold passes, as r.funcs
// judges a captured value (judgeCaptured). A func value
// only compared, as with nil, does not count, nor does one that can be only
// named functions and is only called with a fresh context, as
// [closureReader.callsNamedOnlyFresh] tells (#324).
func (r *closureReader) runsPassing(v ssa.Value) bool {
	if r.funcs == nil {
		return false
	}
	t := v.Type()
	if _, isFV := v.(*ssa.FreeVar); isFV {
		t = typeutil.UnwrapPointer(t)
	}
	if _, isFunc := t.Underlying().(*types.Signature); !isFunc || !closureRuns(v, true) {
		return false
	}
	return !r.callsNamedOnlyFresh(v) && judgeCaptured(r.flow, r.funcs, v).Passes()
}

// callsNamedOnlyFresh reports whether every value v can read, as
// [Flow.CapturedSources] reads it, is a named function or method, of the
// analyzed package or of another, a method expression, or a method value
// whose receiver cannot hold a context (#337), and v, or a value loaded
// from it, is
// used only as the function of calls that each give only a fresh context,
// as [Flow.GivesOnlyFreshContext] judges them. A named function is not
// looked into, but one given only a fresh context does not get ctx, as for
// the same function called in the closure by its name.
func (r *closureReader) callsNamedOnlyFresh(v ssa.Value) bool {
	return closureHoldsOnlyNamed(r.flow, v, r.kinds) && closureCallsOnlyFresh(r.flow, v, r.kinds, true)
}

// closureHoldsOnlyNamed reports whether every value v can read, as
// [Flow.CapturedSources] reads it, out to the function that makes the
// closure, is a named function or method, of the analyzed package or of
// another, a method expression, or a method value whose receiver cannot
// hold a context, as [Flow.IsNamedFuncWithoutContext] tells, each followed
// by [Flow.Sources] in turn, as through a Phi. A
// value loaded from a variable captured by reference reads what is stored
// to the variable.
func closureHoldsOnlyNamed(f *Flow, v ssa.Value, kinds *ctxkind.Classifier) bool {
	if load, ok := v.(*ssa.UnOp); ok && load.Op == token.MUL {
		if _, isFV := load.X.(*ssa.FreeVar); isFV {
			v = load.X
		}
	}
	captured, complete := f.CapturedSources(v)
	if !complete || len(captured) == 0 {
		return false
	}
	return !slices.ContainsFunc(captured, func(c Source) bool {
		if c.Value == v {
			return true
		}
		sources, complete := f.Sources(c.Value)
		return !complete || len(sources) == 0 || slices.ContainsFunc(sources, func(s Source) bool {
			return !f.IsNamedFuncWithoutContext(s.Value, kinds)
		})
	})
}

// closureCallsOnlyFresh reports whether v is used only as the function of
// calls that give only a fresh context, as [Flow.GivesOnlyFreshContext]
// judges them. With throughLoads, a value loaded from v, as v is a variable
// captured by reference, is judged so too. A use of another kind fails.
func closureCallsOnlyFresh(f *Flow, v ssa.Value, kinds *ctxkind.Classifier, throughLoads bool) bool {
	return !slices.ContainsFunc(*v.Referrers(), func(ref ssa.Instruction) bool {
		switch x := ref.(type) {
		case ssa.CallInstruction:
			call := x.Common()
			return call.Value != v || slices.Contains(call.Args, v) || !f.GivesOnlyFreshContext(call, kinds)
		case *ssa.UnOp:
			return !throughLoads || x.Op != token.MUL || !closureCallsOnlyFresh(f, x, kinds, false)
		}
		return true
	})
}

// ClosureArgCapturesContext reports whether arg, which a go statement gives
// the parameter param of the closure it runs, ties the closure to a
// context as the same value captured would, as [Flow.ClosureCapturesContext]
// judges one (#296). A func param the closure runs counts when funcs passes
// every function arg can hold at [SiteCaptured], so one that cannot be
// followed, as a parameter, passes. An interface param the closure calls a
// method of, or gives to a call, counts when arg may hold a context, as
// [closureReader.interfaceMayHold] judges it. A use in a closure the
// closure makes, which captures param, counts the same, as the value
// loaded from the variable SSA spills param to ([spilledUses], #322). A
// slice, an array, a map or a channel of funcs whose elements the closure
// runs counts when the funcs arg holds pass, as [closureElemsPass] judges
// them, as the elements of the same value captured would (#325).
func (f *Flow) ClosureArgCapturesContext(param *ssa.Parameter, arg ssa.Value, kinds *ctxkind.Classifier, funcs Policy) bool {
	defer f.ask()()
	t := param.Type()
	uses := spilledUses(param)
	if _, isFunc := t.Underlying().(*types.Signature); isFunc {
		runs := slices.ContainsFunc(uses, func(u ssa.Value) bool { return closureRuns(u, false) })
		return runs && judgeCaptured(f, funcs, arg).Passes()
	}
	r := &closureReader{flow: f, kinds: kinds.From(closurePackage(param.Parent()))}
	if closureHoldsFuncs(t) {
		r.funcs, r.root, r.seen = funcs, param, make(map[ssa.Value]bool)
		runs := slices.ContainsFunc(uses, func(u ssa.Value) bool { return r.reads(u, false) })
		return runs && closureElemsPass(f, funcs, arg)
	}
	if !types.IsInterface(t) || kinds.Is(t, ctxkind.Captured) {
		return false
	}
	return slices.ContainsFunc(uses, func(u ssa.Value) bool {
		return slices.ContainsFunc(*u.Referrers(), func(ref ssa.Instruction) bool {
			call, ok := ref.(ssa.CallInstruction)
			return ok && r.callsWithHolder(call.Common(), u, arg)
		})
	})
}

// closureHoldsFuncs reports whether t is a slice, an array, a map or a
// channel of funcs.
func closureHoldsFuncs(t types.Type) bool {
	var elem types.Type
	switch u := t.Underlying().(type) {
	case *types.Slice:
		elem = u.Elem()
	case *types.Array:
		elem = u.Elem()
	case *types.Map:
		elem = u.Elem()
	case *types.Chan:
		elem = u.Elem()
	default:
		return false
	}
	_, isFunc := elem.Underlying().(*types.Signature)
	return isFunc
}

// closureElemsPass reports whether the funcs v, a slice, an array, a map
// or a channel of funcs, holds may pass, as funcs judges each at
// [SiteCaptured]. Those of a slice made from a local array, as
// [Flow.ResolvedSliceElems] gives them, pass when one of them does. Those
// of a map or a channel made by the analyzed function, with make or a
// literal, do not pass for that alone: what is put there is judged by the
// arguments' rules, as [Flow.InvolvesContext]. Any other value cannot be
// followed, as a parameter, a field or the result of a call, so it passes.
func closureElemsPass(f *Flow, funcs Policy, v ssa.Value) bool {
	switch v.Type().Underlying().(type) {
	case *types.Slice, *types.Array:
		elems, ok := f.ResolvedSliceElems(v)
		return !ok || slices.ContainsFunc(elems, func(e ssa.Value) bool { return judgeCaptured(f, funcs, e).Passes() })
	}
	sources, complete := f.Sources(v)
	return !complete || slices.ContainsFunc(sources, func(s Source) bool {
		switch s.Value.(type) {
		case *ssa.MakeMap, *ssa.MakeChan:
			return false
		}
		return true
	})
}

// runsPassingPart reports whether part, read from what the closure
// captures, runs passing, as [closureReader.runsPassing] judges it. One
// read from a map, a channel or a type assertion (opaque) cannot be
// followed, so it passes, unless the captured variable is a parameter of a
// factory whose result is followed from a call: the call gives it no func
// that passes, as the parameter itself does not pass then. A parameter
// that is a slice, an array, a map or a channel of funcs is followed to
// what the call gives it, and passes when [closureElemsPass] passes that,
// as when it cannot be followed (#327).
func (r *closureReader) runsPassingPart(part ssa.Value, opaque bool) bool {
	return r.runsPassing(part) && (!opaque || !closureGivenToFactory(r.flow, r.root, r.funcs))
}

// closureGivenToFactory reports whether every value the captured v can be
// is a parameter of a factory whose result a judgement follows from a
// call, and the call gives it no funcs that pass, as
// [closureFactoryArgFails] tells.
func closureGivenToFactory(f *Flow, v ssa.Value, funcs Policy) bool {
	sources, complete := f.CapturedSources(v)
	return complete && len(sources) > 0 && !slices.ContainsFunc(sources, func(s Source) bool {
		p, ok := s.Value.(*ssa.Parameter)
		return !ok || !closureFactoryArgFails(f, p, funcs)
	})
}

// closureFactoryArgFails reports whether p is a parameter of a factory
// whose result a judgement follows from a call, and does not pass for what
// the innermost such call gives it. A slice, an array, a map or a channel
// of funcs passes when [closureElemsPass] passes the argument, as one that
// cannot be followed does. A parameter of any other type does not pass.
func closureFactoryArgFails(f *Flow, p *ssa.Parameter, funcs Policy) bool {
	calls := f.called[p.Parent()]
	if len(calls) == 0 {
		return false
	}
	if !closureHoldsFuncs(p.Type()) {
		return true
	}
	args := calls[len(calls)-1].Args
	i := slices.Index(p.Parent().Params, p)
	return i < len(args) && !closureElemsPass(f, funcs, args[i])
}

// waitsDone reports whether v, a channel read from what the closure
// captures, or a captured variable that holds one, can be the channel of
// the Done method of a context that is not fresh, as ctx.Done() gives
// (#229). A channel that cannot be followed does not count.
func (r *closureReader) waitsDone(v ssa.Value) bool {
	t := v.Type()
	if _, isFV := v.(*ssa.FreeVar); isFV {
		t = typeutil.UnwrapPointer(t)
	}
	if _, isChan := t.Underlying().(*types.Chan); !isChan {
		return false
	}
	read := make(map[ssa.Value]bool)
	var done func(v ssa.Value) bool
	done = func(v ssa.Value) bool {
		if read[v] {
			return false
		}
		read[v] = true
		sources, _ := r.flow.CapturedSources(v)
		return slices.ContainsFunc(sources, func(s Source) bool {
			if fv, ok := s.Value.(*ssa.FreeVar); ok && fv != v {
				return done(fv)
			}
			return r.flow.SignalsContext(s.Value, r.kinds)
		})
	}
	return done(v)
}

// madeWithContext reports whether the captured fv is given to a call in
// the closure, as the receiver of cmd.Run() or an argument ([closureRuns]),
// and a value it can be is made by a call of the standard library given a
// context other than a fresh one, in an argument or as the receiver, as
// [Flow.InvolvesContext] judges each. The value must be of a type with a
// context field, exported or not ([contextMadeByStd]), as
// exec.CommandContext(ctx, ...), http.NewRequestWithContext(ctx, ...) and
// req.WithContext(ctx) make (#257): the call has put ctx there, in a field
// the analyzed code cannot read. A call of many results is judged by each
// one taken from it. A go statement's argument counts such a result too
// ([Flow.InvolvesContext]). A value of an interface or a func type is left
// to the other rules, as a net.Conn a dial given ctx makes holds no ctx
// after the dial (#240). A result of another type, as a string made from
// ctx, holds none, and a call of another package, as zerolog.Ctx(ctx), may
// take the value out of ctx.
func (r *closureReader) madeWithContext(fv *ssa.FreeVar) bool {
	if !closureRuns(fv, true) {
		return false
	}
	captured, _ := r.flow.CapturedSources(fv)
	return slices.ContainsFunc(captured, func(c Source) bool {
		sources, _ := r.flow.Sources(c.Value)
		return slices.ContainsFunc(sources, func(s Source) bool {
			call := contextMakingCall(s.Value)
			return call != nil && contextMadeByStd(call, s.Value.Type(), r.kinds) &&
				slices.ContainsFunc(call.Args, func(arg ssa.Value) bool { return r.flow.InvolvesContext(arg, r.kinds) })
		})
	})
}

// closureRuns reports whether v is called, or given to a call that is not
// of a builtin. With throughLoads, a value loaded from v, as v is a variable
// captured by reference, counts too.
func closureRuns(v ssa.Value, throughLoads bool) bool {
	return slices.ContainsFunc(*v.Referrers(), func(ref ssa.Instruction) bool {
		switch x := ref.(type) {
		case ssa.CallInstruction:
			call := x.Common()
			if _, isBuiltin := call.Value.(*ssa.Builtin); isBuiltin {
				return false
			}
			return call.Value == v || slices.Contains(call.Args, v)
		case *ssa.UnOp:
			return throughLoads && x.Op == token.MUL && closureRuns(x, false)
		}
		return false
	})
}

// closureHoldsCapturing reports whether the func variable fv captures holds
// a closure that captures a context.
func closureHoldsCapturing(f *Flow, fv *ssa.FreeVar, kinds *ctxkind.Classifier, funcs Policy, seen map[*ssa.Function]bool) bool {
	if _, isFunc := typeutil.UnwrapPointer(fv.Type()).Underlying().(*types.Signature); !isFunc {
		return false
	}
	sources, ok := f.CapturedSources(fv)
	return ok && slices.ContainsFunc(sources, func(s Source) bool {
		mc, ok := s.Value.(*ssa.MakeClosure)
		if !ok {
			return false
		}
		fn, _ := mc.Fn.(*ssa.Function)
		return closureCaptures(f, fn, kinds, funcs, seen)
	})
}

// closureIsFresh reports whether every value the captured v can read is a
// fresh context, as [Flow.IsFreshContext] judges it.
func closureIsFresh(f *Flow, v ssa.Value) bool {
	sources, ok := f.CapturedSources(v)
	return ok && len(sources) > 0 && !slices.ContainsFunc(sources, func(s Source) bool { return !f.IsFreshContext(s.Value) })
}

// ClosureDerivation is where a closure calls the deriver.
type ClosureDerivation struct {
	FoundAtStart     bool
	FoundOnlyInDefer bool
}

// ClosureCallsDeriver checks if a closure calls any of the required deriver
// functions. given are the functions called for it before it starts, as in
// the arguments of the go statement that runs it; they count as called at
// its start. A func value the closure calls is not looked into: ask
// [Flow.ClosureDerives] for that.
func ClosureCallsDeriver(closure *ssa.Function, matcher *deriver.Matcher, given ...*types.Func) ClosureDerivation {
	return closureDerivation(nil, closure, matcher, nil, given)
}

// ClosureDerives checks if a closure calls any of the required deriver
// functions, as [ClosureCallsDeriver] does, or calls a func value that
// funcs passes as it would pass the same value spawned directly (#299): a
// func value read from a variable the closure captures or from a part of
// one, as fn() or s.fn() reads it, or the result of a call, as factory()()
// or decorate(fn)() gives it, each judged at [SiteCaptured]. One called at
// the start that passes, or cannot be followed, makes the closure call the
// deriver at its start. One that fails with [FailDeferOnly], or passes
// called only in defer, makes it call the deriver only in defer. A call of
// a named function or of an interface method is not looked into. funcs may
// be nil, and then no func value is.
func (f *Flow) ClosureDerives(closure *ssa.Function, matcher *deriver.Matcher, funcs Policy, given ...*types.Func) ClosureDerivation {
	return closureDerivation(f, closure, matcher, funcs, given)
}

// closureDerivation is [Flow.ClosureDerives], with funcs nil for
// [ClosureCallsDeriver].
func closureDerivation(f *Flow, closure *ssa.Function, matcher *deriver.Matcher, funcs Policy, given []*types.Func) ClosureDerivation {
	if closure == nil || matcher.IsEmpty() {
		return ClosureDerivation{FoundAtStart: true}
	}

	var calls []closureDeriverCall
	for _, fn := range given {
		calls = append(calls, closureDeriverCall{fn: fn})
	}
	var values []RunCall
	for site := range Runs(closure, RunOptions{}) {
		if site.isGo {
			continue
		}
		if fn := ExtractCalledFunc(site.Call); fn != nil {
			calls = append(calls, closureDeriverCall{fn: fn, inDefer: site.inDefer})
		} else if closureCallsFuncValue(site.Call) {
			values = append(values, site)
		}
	}

	// Check if any OR group is satisfied at start
	for _, andGroup := range matcher.OrGroups {
		if closureCallsGroup(calls, andGroup, false) {
			return ClosureDerivation{FoundAtStart: true}
		}
	}

	deferOnly := false
	if funcs != nil {
		for _, site := range values {
			switch v := judgeCaptured(f, funcs, site.Call.Value); {
			case !site.inDefer && v.Passes():
				return ClosureDerivation{FoundAtStart: true}
			case v == FailDeferOnly, site.inDefer && v == Pass:
				deferOnly = true
			}
		}
	}

	// Check if deriver is only in defer
	for _, andGroup := range matcher.OrGroups {
		if closureCallsGroup(calls, andGroup, true) {
			return ClosureDerivation{FoundOnlyInDefer: true}
		}
	}

	return ClosureDerivation{FoundOnlyInDefer: deferOnly}
}

// closureCallsFuncValue reports whether call calls a func value, not a
// static callee, an interface method or a builtin. The iterator of a
// range-over-func loop is left out, as [Runs] goes into its body.
func closureCallsFuncValue(call *ssa.CallCommon) bool {
	_, isBuiltin := call.Value.(*ssa.Builtin)
	return !isBuiltin && !call.IsInvoke() && call.StaticCallee() == nil && runRangeBody(call) == nil
}

type closureDeriverCall struct {
	fn      *types.Func
	inDefer bool
}

func closureCallsGroup(calls []closureDeriverCall, andGroup []funcspec.Spec, includeDefer bool) bool {
	for _, spec := range andGroup {
		matches := func(call closureDeriverCall) bool {
			return (includeDefer || !call.inDefer) && call.fn != nil && spec.Matches(call.fn)
		}
		if !slices.ContainsFunc(calls, matches) {
			return false
		}
	}
	return true
}
