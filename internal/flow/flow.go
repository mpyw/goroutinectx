//declscope:core

package flow

import (
	"golang.org/x/tools/go/ssa"
)

// Flow follows SSA values back to where they come from, for one pass. Every
// question asked of it, as [Flow.Sources] or [FuncResolver.Passes], shares
// one bound on the work it does, with the questions it asks in turn: a
// resolver judging a closure that asks whether a context is fresh does not
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
}

// NewFlow gives the Flow of one pass.
func NewFlow() *Flow {
	return &Flow{
		sources: make(map[ssa.Value]resolvedValuesResult),
		parts:   make(map[resolvedPartKey]resolvedValuesResult),
	}
}

// ask starts a question, or joins the one in progress, and gives the
// function that ends it.
//
//declscope:shared // each question to the Flow asks, in resolve.go, context.go and closure.go
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
	sources = make([]Source, len(values))
	for i, v := range values {
		sources[i] = Source{Value: v, Kind: sourceKindOf(v)}
	}
	return sources, complete
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
