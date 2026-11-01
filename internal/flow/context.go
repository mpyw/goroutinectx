package flow

import (
	"go/types"
	"slices"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/ctxkind"
)

// IsFreshContext reports whether every source of v is a call that
// [ctxkind.IsFreshCall] accepts, as context.Background(): a context that
// carries nothing of the caller's.
func (f *Flow) IsFreshContext(v ssa.Value) bool {
	sources, complete := f.Sources(v)
	if !complete || len(sources) == 0 {
		return false
	}
	return !slices.ContainsFunc(sources, func(s Source) bool {
		call, ok := s.Value.(*ssa.Call)
		return !ok || !ctxkind.IsFreshCall(ExtractCalledFunc(call.Common()))
	})
}

// InvolvesContext reports whether v is built from a context other than a
// fresh one, as [ctxkind.Classifier.InvolvesUnless] follows it with
// [ctxkind.Given], and the parts [contextLeftOut] tells left out. A go
// statement's argument for a parameter that is not a context counts so
// (#252), and so does a call that makes a value a closure captures (#257).
// A variable a closure captures, or a load of one, is followed to the
// values it is made with, as a go statement inside a closure gives one
// (#269).
func (f *Flow) InvolvesContext(v ssa.Value, kinds *ctxkind.Classifier) bool {
	defer f.ask()()
	return kinds.InvolvesUnless(ctxkind.Given, func(part ssa.Value) bool { return contextLeftOut(f, part, kinds) }, f.CapturedValues(v)...)
}

// contextLeftOut reports whether part, a part of a value
// [Flow.InvolvesContext] looks into, is not to count toward ctx nor to be
// looked into:
//
//   - a context.Context made by a fresh context.Background() or TODO(), as
//     [Flow.IsFreshContext] tells;
//   - the result of a call that cannot hold what the call was given, as
//     [contextCallResultMayHold] tells. Such a result is data the call made,
//     as the brokers a call given ctx lists. A call of many results is
//     judged by each one taken from it.
func contextLeftOut(f *Flow, part ssa.Value, kinds *ctxkind.Classifier) bool {
	if kinds.Is(part.Type(), ctxkind.Context) {
		return f.IsFreshContext(part)
	}
	if x, ok := part.(*ssa.Call); ok {
		if _, many := x.Type().(*types.Tuple); many {
			return false
		}
	}
	call := contextMakingCall(part)
	return call != nil && !contextCallResultMayHold(call, part.Type(), kinds)
}

// contextCallResultMayHold reports whether a result of type t of call may
// hold a context the call was given: t is of [ctxkind.Given], an interface
// or a func, or call is of the standard library and t has a context field
// of any package, exported or not ([contextMadeByStd]).
func contextCallResultMayHold(call *ssa.CallCommon, t types.Type, kinds *ctxkind.Classifier) bool {
	switch t.Underlying().(type) {
	case *types.Interface, *types.Signature:
		return true
	}
	return kinds.Is(t, ctxkind.Given) || contextMadeByStd(call, t, kinds)
}

// contextMadeByStd reports whether call, a function or a method of the
// standard library and not an interface method, makes a result of type t
// that has a context field, exported or not
// ([ctxkind.Classifier.HasContextField]), as the *exec.Cmd that
// exec.CommandContext(ctx, ...) makes (#257). Such a call given ctx puts it
// in the field. A call of another package may instead take a value out of
// ctx, as zerolog.Ctx(ctx) gives the *zerolog.Logger stored in ctx, whose
// context field is not ctx. A type of the module whose field the analyzed
// code can set is judged by [ctxkind.Holds] instead.
//
//declscope:shared // closure.go judges a captured value a call made with ctx
func contextMadeByStd(call *ssa.CallCommon, t types.Type, kinds *ctxkind.Classifier) bool {
	if call.IsInvoke() {
		return false
	}
	fn := ExtractCalledFunc(call)
	return fn != nil && fn.Pkg() != nil && ctxkind.IsStdPackage(fn.Pkg()) && kinds.HasContextField(t)
}

// contextMakingCall gives the call v is the result of, alone or taken from
// its results, or nil.
//
//declscope:shared // closure.go finds the call that made a captured value
func contextMakingCall(v ssa.Value) *ssa.CallCommon {
	if x, ok := v.(*ssa.Extract); ok {
		v = x.Tuple
	}
	call, ok := v.(*ssa.Call)
	if !ok {
		return nil
	}
	return call.Common()
}

// GivesOnlyFreshContext reports whether call gives a fresh
// context.Background() or TODO(), and every other argument is sure to hold
// no value of [ctxkind.Given], as [ctxkind.Classifier.MayHold] judges it,
// and is
// no signal of a context, as [Flow.SignalsContext] judges it: the channel of
// ctx.Done() ties the callee to ctx (#241).
func (f *Flow) GivesOnlyFreshContext(call *ssa.CallCommon, kinds *ctxkind.Classifier) bool {
	defer f.ask()()
	given := false
	for _, arg := range call.Args {
		if kinds.Is(arg.Type(), ctxkind.Context) && f.IsFreshContext(arg) {
			given = true
			continue
		}
		if kinds.MayHold(arg, ctxkind.Given) || f.SignalsContext(arg, kinds) {
			return false
		}
	}
	return given
}

// IsNamedFuncWithoutContext reports whether v is a named function or
// method, generic or not, of the analyzed package or of another, a method
// expression, as T.M, or a method value whose receiver cannot hold a
// context, as [Flow.GivesOnlyFreshContext] judges an argument. Such a
// function holds no context of its own: a method expression takes its
// receiver as the first argument of the call, which is judged with the
// others (#274, #323, #324).
func (f *Flow) IsNamedFuncWithoutContext(v ssa.Value, kinds *ctxkind.Classifier) bool {
	bound, ok := contextNamedFunc(v)
	return ok && !slices.ContainsFunc(bound, func(recv ssa.Value) bool {
		return kinds.MayHold(recv, ctxkind.Given) || f.SignalsContext(recv, kinds)
	})
}

// contextNamedFunc reports whether v is a named function or method, a
// method expression or a method value, as [Flow.IsNamedFuncWithoutContext]
// tells one before it looks at the receiver. It gives the receiver a method
// value holds, and nothing for the others.
func contextNamedFunc(v ssa.Value) (bound []ssa.Value, ok bool) {
	if mc, ok := v.(*ssa.MakeClosure); ok {
		fn, _ := mc.Fn.(*ssa.Function)
		return mc.Bindings, Classify(fn) == ClassifiedBound
	}
	fn, ok := v.(*ssa.Function)
	if !ok {
		return nil, false
	}
	switch Classify(fn) {
	case ClassifiedNamed, ClassifiedMethod, ClassifiedThunk, ClassifiedExternal:
		// A function or a method of another package is built from type
		// information.
		return nil, true
	}
	return nil, false
}

// NamedFuncsWithoutContext reports whether call calls only named functions
// or methods, as [Flow.IsNamedFuncWithoutContext] tells them, that cannot get
// a context from it. No parameter of the signature is of [ctxkind.Given], and
// no argument, receiver or receiver of a method value is tied to a context,
// as [contextTiedTo] judges one. It gives the functions with a body that the
// callees are, a method value or a method expression as its method, for the
// caller to judge further. With -report-callee-without-context, a go
// statement that calls them is reported.
func (f *Flow) NamedFuncsWithoutContext(call *ssa.CallCommon, kinds *ctxkind.Classifier) ([]*ssa.Function, bool) {
	defer f.ask()()
	for param := range call.Signature().Params().Variables() {
		if kinds.Is(param.Type(), ctxkind.Given) {
			return nil, false
		}
	}
	sources, complete := f.Sources(call.Value)
	if !complete || len(sources) == 0 {
		return nil, false
	}
	given := slices.Clone(call.Args)
	var bodies []*ssa.Function
	for _, s := range sources {
		bound, ok := contextNamedFunc(s.Value)
		if !ok || contextAbstractMethod(s.Func()) {
			return nil, false
		}
		given = append(given, bound...)
		if body := contextFuncBody(s.Func()); body != nil {
			bodies = append(bodies, body)
		}
	}
	if slices.ContainsFunc(given, func(v ssa.Value) bool { return contextTiedTo(f, v, kinds) }) {
		return nil, false
	}
	return bodies, true
}

// contextInvolvesOrSignals reports whether v is built from a context that
// is not fresh, as [Flow.InvolvesContext] follows it, or from a signal of
// one, as [Flow.SignalsContext] tells. A signal counts as a part too, as the
// channel of ctx.Done() set in a field of &T{done: ctx.Done()}.
func contextInvolvesOrSignals(f *Flow, v ssa.Value, kinds *ctxkind.Classifier) bool {
	signals := false
	involves := kinds.InvolvesUnless(ctxkind.Given, func(part ssa.Value) bool {
		if f.SignalsContext(part, kinds) {
			signals = true
			return true
		}
		return contextLeftOut(f, part, kinds)
	}, f.CapturedValues(v)...)
	return involves || signals
}

// contextAbstractMethod reports whether fn is the wrapper of a method value
// or a method expression of an interface method, as c.Close or
// io.Closer.Close. A call of it runs the method of the dynamic type, as
// go c.Close() does, so [Flow.NamedFuncsWithoutContext] leaves it alone too
// (#385).
func contextAbstractMethod(fn *ssa.Function) bool {
	switch Classify(fn) {
	case ClassifiedBound, ClassifiedThunk:
		return types.IsInterface(fn.Object().(*types.Func).Signature().Recv().Type())
	}
	return false
}

// contextFuncBody gives the function with a body that fn runs: fn itself, or
// the method a method value or a method expression wraps. An instance of a
// generic function or method, as waitAll[int], has no body of its own, so
// its generic origin is given (#384). It gives nil for a function built from
// type information.
func contextFuncBody(fn *ssa.Function) *ssa.Function {
	switch Classify(fn) {
	case ClassifiedBound, ClassifiedThunk:
		// SSA gives a wrapper the method it wraps as its object.
		fn = fn.Prog.FuncValue(fn.Object().(*types.Func).Origin())
	}
	if fn != nil && fn.Origin() != nil {
		fn = fn.Origin()
	}
	if fn == nil || fn.Blocks == nil {
		return nil
	}
	return fn
}

// contextTiedTo reports whether v, a value a named function is given, ties
// it to a context, by the rules a value a closure is given counts by:
//
//   - a value of [ctxkind.Given], as a context, a carrier or a struct that
//     holds one. A fresh context can only be given to a parameter of
//     [ctxkind.Given], which the caller has ruled out;
//   - a value built from a context that is not fresh, or from a signal of
//     one, as [contextInvolvesOrSignals] tells: the *exec.Cmd that
//     exec.CommandContext(ctx, ...) makes, the channel of ctx.Done(), or
//     &T{done: ctx.Done()};
//   - a func that may hold a context, as [ctxkind.Classifier.MayHold] judges
//     it, so a func value that cannot be followed counts;
//   - an interface that may hold a context, as
//     [closureInterfaceArgMayHold] judges it.
func contextTiedTo(f *Flow, v ssa.Value, kinds *ctxkind.Classifier) bool {
	t := v.Type()
	if kinds.Is(t, ctxkind.Given) || contextInvolvesOrSignals(f, v, kinds) {
		return true
	}
	if _, isFunc := t.Underlying().(*types.Signature); isFunc {
		return kinds.MayHold(v, ctxkind.Given)
	}
	return types.IsInterface(t) && closureInterfaceArgMayHold(f, v, kinds)
}

// SignalsContext reports whether one of the sources of v is a signal of a
// context that is not fresh, as [Flow.IsFreshContext] judges it: the
// channel of its Done method, or the func that cancels a context made from
// it, as context.WithCancel gives second ([ctxkind.IsCancelCall]). A
// goroutine that waits on the one or calls the other is tied to the context.
func (f *Flow) SignalsContext(v ssa.Value, kinds *ctxkind.Classifier) bool {
	defer f.ask()()
	sources, _ := f.Sources(v)
	return slices.ContainsFunc(sources, func(s Source) bool {
		parent, ok := contextSignalled(s.Value, kinds)
		return ok && !f.IsFreshContext(parent)
	})
}

// contextSignalled gives the context v is a signal of, as
// [contextDoneOf] or [contextCancelOf] gives it.
func contextSignalled(v ssa.Value, kinds *ctxkind.Classifier) (ssa.Value, bool) {
	if parent, ok := contextCancelOf(v); ok {
		return parent, true
	}
	return contextDoneOf(v, kinds)
}

// contextDoneOf gives the context whose Done method v is the result of.
func contextDoneOf(v ssa.Value, kinds *ctxkind.Classifier) (ssa.Value, bool) {
	x, ok := v.(*ssa.Call)
	if !ok {
		return nil, false
	}
	call := x.Common()
	fn := ExtractCalledFunc(call)
	// A method value, as done := ctx.Done, is called with no receiver.
	if fn == nil || fn.Name() != "Done" || fn.Signature().Recv() == nil || (!call.IsInvoke() && len(call.Args) == 0) {
		return nil, false
	}
	recv := call.Value
	if !call.IsInvoke() {
		recv = call.Args[0]
	}
	return recv, kinds.Is(recv.Type(), ctxkind.Context|ctxkind.Implements)
}

// contextCancelOf gives the context given to the call whose cancel func v
// is, the second result of a call that [ctxkind.IsCancelCall] accepts.
//
//declscope:shared // resolve.go judges a cancel func by the context it cancels
func contextCancelOf(v ssa.Value) (ssa.Value, bool) {
	x, ok := v.(*ssa.Extract)
	if !ok || x.Index != 1 {
		return nil, false
	}
	call, ok := x.Tuple.(*ssa.Call)
	if !ok || !ctxkind.IsCancelCall(ExtractCalledFunc(call.Common())) {
		return nil, false
	}
	return call.Common().Args[0], true
}
