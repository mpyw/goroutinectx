package flow

import (
	"strings"

	"golang.org/x/tools/go/ssa"
)

// ClassifiedKind is the kind of function an SSA function is.
type ClassifiedKind int

const (
	// ClassifiedOther is nil, or a function of no kind below, as a package
	// initializer.
	ClassifiedOther ClassifiedKind = iota
	// ClassifiedLiteral is a function literal.
	ClassifiedLiteral
	// ClassifiedRangeBody is the body of a range-over-func loop, which SSA
	// builds as a synthetic yield closure inside the function of the loop.
	ClassifiedRangeBody
	// ClassifiedNamed is a function declared in the analyzed package, with
	// no receiver.
	ClassifiedNamed
	// ClassifiedMethod is a method declared in the analyzed package.
	ClassifiedMethod
	// ClassifiedBound is the wrapper of a method value, as x.M not called,
	// which holds the receiver.
	ClassifiedBound
	// ClassifiedThunk is the wrapper of a method expression, as T.M, which
	// takes the receiver first.
	ClassifiedThunk
	// ClassifiedWrapper is the wrapper of a promoted method, or of a method
	// called through an interface.
	ClassifiedWrapper
	// ClassifiedExternal is a function or method SSA has no syntax for, as
	// one of an imported package: it is built from type information.
	ClassifiedExternal
)

// Classify gives the kind of fn. An instantiation of a generic function, and
// a function literal inside one, is classified as its generic origin, so a
// generic function is the same kind as a plain one.
func Classify(fn *ssa.Function) ClassifiedKind {
	if fn == nil {
		return ClassifiedOther
	}
	if origin := fn.Origin(); origin != nil {
		fn = origin
	}
	switch syn := fn.Synthetic; {
	case syn == "range-over-func yield":
		return ClassifiedRangeBody
	case fn.Parent() != nil:
		return ClassifiedLiteral
	case syn == "" && fn.Signature.Recv() == nil:
		return ClassifiedNamed
	case syn == "":
		return ClassifiedMethod
	case strings.HasPrefix(syn, "bound method wrapper "):
		return ClassifiedBound
	case strings.HasPrefix(syn, "thunk "):
		return ClassifiedThunk
	case strings.HasPrefix(syn, "wrapper "):
		return ClassifiedWrapper
	case strings.HasPrefix(syn, "from type information"):
		return ClassifiedExternal
	}
	return ClassifiedOther
}
