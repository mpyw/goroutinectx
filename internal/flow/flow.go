//declscope:core

package flow

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// Flow follows SSA values back to where they come from, for one pass. Every
// question asked of it, as [Flow.Sources], [Flow.CapturedSources],
// [Flow.Returns] or [Judge], shares
// one bound on the work it does, with the questions it asks in turn: a
// policy judging a closure that asks whether a context is fresh does not
// start a new bound. What it finds is remembered for the rest of the pass,
// unless the bound cut it short.
type Flow struct {
	// steps is the work done by the question in progress, toward
	// resolvedMaxSteps.
	steps int //declscope:shared // resolve.go counts the work of its walks
	// cuts counts the times a walk stopped at a bound. What is found while
	// it does not change is complete, and is remembered.
	cuts int //declscope:shared // resolve.go counts the walks it stops
	// sources remembers the values a value can be.
	sources map[ssa.Value]resolvedValuesResult //declscope:shared // resolve.go remembers its walks
	// parts remembers the part of the value a store stores, by the store
	// and the path into it.
	parts map[resolvedPartKey]resolvedValuesResult //declscope:shared // resolve.go remembers its walks
	// open is the number of questions in progress. The bound starts again
	// when one is asked while none is.
	open int
	// closures are the closures being judged by ClosureCapturesContext, up
	// the stack, each by its depth on it, from 1: a closure reached again
	// through a func value it captures adds nothing.
	closures map[*ssa.Function]int //declscope:shared // closure.go judges a closure once on the stack
	// closureLow is the lowest depth in closures that a closure being
	// judged was cut at, as it was on the stack or judged before, or
	// closureUnheld when none was. A verdict that depends on no closure
	// below its own is remembered in captures.
	closureLow int //declscope:shared // closure.go tracks what a verdict depends on
	// captures remembers the verdicts of ClosureCapturesContext, by the
	// closure and what it is judged with (#239).
	captures map[closureVerdictKey]bool //declscope:shared // closure.go remembers its verdicts
	// called are the factories whose results a judgement follows from a call
	// that gives them no func that passes, up the stack. A func parameter of
	// one holds what the call gives, so it does not pass for being a
	// parameter.
	called map[*ssa.Function]int //declscope:shared // judge.go follows the factories a call calls
}

// NewFlow gives the Flow of one pass.
func NewFlow() *Flow {
	return &Flow{
		sources:  make(map[ssa.Value]resolvedValuesResult),
		parts:    make(map[resolvedPartKey]resolvedValuesResult),
		closures: make(map[*ssa.Function]int),
		captures: make(map[closureVerdictKey]bool),
		called:   make(map[*ssa.Function]int),
	}
}

// ask starts a question, or joins the one in progress, and gives the
// function that ends it.
//
//declscope:shared // each question to the Flow asks, in resolve.go, judge.go, context.go and closure.go
func (f *Flow) ask() (done func()) {
	if f.open == 0 {
		f.steps = 0
	}
	f.open++
	return func() { f.open-- }
}

// SourceKind is what a [Source] is.
type SourceKind int

const (
	// SourceUnknown is a value of no kind below, as a constant, a type
	// assertion or a value received from a channel.
	SourceUnknown SourceKind = iota
	// SourceLiteral is a function literal, or a closure of one.
	SourceLiteral
	// SourceFunc is a named function with no receiver.
	SourceFunc
	// SourceMethod is a method, or a wrapper of one: a method value, a
	// method expression or a promoted method.
	SourceMethod
	// SourceCall is the result of a call: the value of a call with one
	// result, or one of the results of a call with several.
	SourceCall
	// SourceParam is a parameter of a function.
	SourceParam
	// SourceGlobal is the address of a package-level variable.
	SourceGlobal
)

// Source is a value another value can come from, as [Flow.Sources] gives it.
type Source struct {
	// Value is the value.
	Value ssa.Value
	// Kind is what Value is.
	Kind SourceKind
}

// Func gives the function the source is, or the function of the closure it
// is, or nil.
func (s Source) Func() *ssa.Function {
	switch v := s.Value.(type) {
	case *ssa.MakeClosure:
		fn, _ := v.Fn.(*ssa.Function)
		return fn
	case *ssa.Function:
		return v
	}
	return nil
}

// Call gives the call whose result the source is, or nil when it is not
// the result of a call.
func (s Source) Call() *ssa.Call {
	switch v := s.Value.(type) {
	case *ssa.Call:
		return v
	case *ssa.Extract:
		call, _ := v.Tuple.(*ssa.Call)
		return call
	}
	return nil
}

// Sources gives the values v can come from, through Phi nodes, type
// changes, loads of local variables and of their parts, and map lookups at
// constant keys. complete is false when some of them cannot be followed, as
// a variable whose address is passed on, or past the bound on the work.
func (f *Flow) Sources(v ssa.Value) (sources []Source, complete bool) {
	defer f.ask()()
	values, complete := resolvedSources(f, v)
	return sourcesOf(values), complete
}

// CapturedSources gives what v reads when it reads a variable a closure
// captures, each as a [Source]. For a free variable that is the value the
// closure was made with, or for a variable captured by reference, every
// value stored to it, as the closure may run after any of those stores. For
// a field, or an element at a constant index, of a value, it is that part
// of each value the value can be. Any other v is given as it is. Each is
// given as it was stored, not followed further: ask [Flow.Sources] of it.
// complete is false when they cannot all be followed.
//
// [Judge] does not read values this way: a load it follows sees only
// the stores that reach it.
func (f *Flow) CapturedSources(v ssa.Value) (sources []Source, complete bool) {
	defer f.ask()()
	values, complete := resolvedCaptured(f, v)
	return sourcesOf(values), complete
}

// capturedValuesMax bounds the values [Flow.CapturedValues] gives.
const capturedValuesMax = 64

// CapturedValues gives v, and for a free variable, or a load of one, the
// values it is captured with, through each closure it is captured by in
// turn (#269), and for a Phi, the values it joins, so that a Phi of loads of
// free variables is followed too (#297). A value that cannot be followed is
// left out.
func (f *Flow) CapturedValues(v ssa.Value) []ssa.Value {
	defer f.ask()()
	vs := []ssa.Value{v}
	seen := map[ssa.Value]bool{v: true}
	for i := 0; i < len(vs) && len(vs) < capturedValuesMax; i++ {
		values := capturedValuesOf(f, vs[i])
		for _, c := range values {
			if !seen[c] {
				seen[c] = true
				vs = append(vs, c)
			}
		}
	}
	return vs
}

// capturedValuesOf gives the values [Flow.CapturedValues] follows x to: for
// a free variable, or a load of one, the values it is captured with, and
// for a Phi, the values it joins. A Phi is not followed to its sources, as
// [Flow.Sources] gives them, since a load of a variable captured by
// reference resolves to what is stored to it, without the variable that
// what is put into it is put into, as ch <- f is into a captured ch.
func capturedValuesOf(f *Flow, x ssa.Value) []ssa.Value {
	if phi, ok := x.(*ssa.Phi); ok {
		return phi.Edges
	}
	if load, ok := x.(*ssa.UnOp); ok && load.Op == token.MUL {
		x = load.X
	}
	fv, ok := x.(*ssa.FreeVar)
	if !ok {
		return nil
	}
	values, _ := resolvedCaptured(f, fv)
	if _, byRef := fv.Type().Underlying().(*types.Pointer); byRef {
		// The variable itself, so that what is put into a value
		// loaded from it counts, as ch <- f does to a captured ch.
		if binding, ok := resolvedBinding(fv); ok {
			values = append(values, binding)
		}
	}
	return values
}

// Return is a value a function returns, as [Flow.Returns] and
// [Flow.ReturnsAt] give it.
type Return struct {
	// Value is the value returned.
	Value ssa.Value
	// Sources are the sources of Value, as [Flow.Sources] gives them.
	Sources []Source
}

// Returns gives what fn returns as its first result, one for each return in
// block order, with the sources of each: what a call of fn can give, as
// func() (func(), error) gives the function first. [Flow.Sources] never
// looks through a call, so a check that follows a call to what it gives,
// as of a factory, asks this. A function with no body gives nothing.
func (f *Flow) Returns(fn *ssa.Function) []Return {
	return f.ReturnsAt(fn, int(returnedFirst))
}

// ReturnsAt gives what fn returns as its result at index, as [Flow.Returns]
// gives the first: what an [ssa.Extract] of a call of fn at index can give,
// as t of t, err := mk() (#290).
func (f *Flow) ReturnsAt(fn *ssa.Function, index int) []Return {
	defer f.ask()()
	values := returns(fn, returnedResults(index))
	returned := make([]Return, len(values))
	for i, v := range values {
		sources, _ := f.Sources(v)
		returned[i] = Return{Value: v, Sources: sources}
	}
	return returned
}

// sourcesOf gives each of values as a source.
func sourcesOf(values []ssa.Value) []Source {
	sources := make([]Source, len(values))
	for i, v := range values {
		sources[i] = Source{Value: v, Kind: sourceKindOf(v)}
	}
	return sources
}

// sourceKindOf tells what v is.
func sourceKindOf(v ssa.Value) SourceKind {
	if fn := (Source{Value: v}).Func(); fn != nil {
		return sourceFuncKind(fn)
	}
	switch x := v.(type) {
	case *ssa.Call:
		return SourceCall
	case *ssa.Extract:
		if _, ok := x.Tuple.(*ssa.Call); ok {
			return SourceCall
		}
	case *ssa.Parameter:
		return SourceParam
	case *ssa.Global:
		return SourceGlobal
	}
	return SourceUnknown
}

// sourceFuncKind tells what fn is, as [Classify] tells it.
func sourceFuncKind(fn *ssa.Function) SourceKind {
	switch Classify(fn) {
	case ClassifiedLiteral:
		return SourceLiteral
	case ClassifiedNamed:
		return SourceFunc
	case ClassifiedMethod, ClassifiedBound, ClassifiedThunk, ClassifiedWrapper:
		return SourceMethod
	case ClassifiedExternal:
		if fn.Signature.Recv() == nil {
			return SourceFunc
		}
		return SourceMethod
	}
	return SourceUnknown
}

// Verdict is what [Judge] gives for the functions a value can hold.
type Verdict uint8

const (
	// Unknown is the verdict of what cannot be followed. It counts as Pass,
	// so that only what is resolved is reported. A hook of a [Policy] that
	// gives it leaves the value to the rules after it.
	Unknown Verdict = iota
	// Pass is the verdict of a function that does what the policy asks.
	Pass
	// Fail is the verdict of a function that does not.
	Fail
	// FailDeferOnly is the verdict of a function that does it only in
	// defer, too late. It fails, but less than Fail does: of two values one
	// of which is enough, the one that fails this way wins over the one
	// that fails outright.
	FailDeferOnly
)

// Passes reports whether v counts as passing: Pass or Unknown.
func (v Verdict) Passes() bool {
	return v == Pass || v == Unknown
}

// Site is where the function a [Policy] judges runs.
type Site uint8

const (
	// SiteSpawned is a function a goroutine or a callback runs.
	SiteSpawned Site = iota
	// SiteCaptured is a func value a closure captures and runs, judged as
	// if it were spawned directly (#234), as [Flow.ClosureCapturesContext]
	// judges the func values a closure runs.
	SiteCaptured
)

// Policy is what [Judge] judges the functions a value can hold by. Judge
// asks its hooks in this order, each with a built-in rule after it:
//
//  1. Literal, for a function literal the value can be.
//  2. Call, for the result of a call. Unknown leaves it to the rules
//     below: a call of an interface method passes; FactoryCall; a call
//     given a function that passes passes too, as a decorator runs it;
//     what the callee returns is followed.
//  3. Factory, for a function whose result is followed. When it does not
//     pass the function, one of the values the function returns passing is
//     enough.
//
// A Policy is compared by identity, as [Flow.ClosureCapturesContext]
// remembers its verdicts by the policy too, so it must be comparable, as a
// pointer is.
type Policy interface {
	// Literal judges a function literal, run at site.
	Literal(fn *ssa.Function, site Site) Verdict
	// Call judges the result of call, or gives Unknown to leave it to the
	// rules after it.
	Call(call *ssa.CallCommon) Verdict
	// FactoryCall reports whether the result of call passes without being
	// followed, as when a context is passed to the factory it calls.
	FactoryCall(call *ssa.CallCommon) bool
	// Factory reports whether fn passes without what it returns being
	// followed, as when it takes a context.
	Factory(fn *ssa.Function) bool
}
