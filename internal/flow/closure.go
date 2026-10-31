package flow

import (
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/ctxkind"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/funcspec"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// ClosureCapturesContext checks if a closure captures any context.Context variable
// or a configured carrier type. A variable that only ever holds a fresh
// context.Background() or TODO() does not count. A value whose type
// implements context.Context counts as a context. A context the closure
// reads from a field or an element of a variable it captures counts, unless
// it is fresh. So does a method called on a captured value that returns a
// context, and a call that takes a captured value that holds a context the
// analyzed code can have put there ([ctxkind.Holds]), as its receiver or as
// any argument, as a function or method spawned directly with it is left
// alone. A captured closure that captures a context counts too.
func (f *Flow) ClosureCapturesContext(closure *ssa.Function, kinds *ctxkind.Classifier) bool {
	defer f.ask()()
	return closureCaptures(f, closure, kinds, make(map[*ssa.Function]bool))
}

func closureCaptures(f *Flow, closure *ssa.Function, kinds *ctxkind.Classifier, seen map[*ssa.Function]bool) bool {
	if closure == nil || seen[closure] {
		return false
	}
	seen[closure] = true

	r := &closureReader{
		flow:  f,
		kinds: kinds.From(closurePackage(closure)),
		seen:  make(map[ssa.Value]bool),
	}
	return slices.ContainsFunc(closure.FreeVars, func(fv *ssa.FreeVar) bool {
		switch k := r.kinds.Kind(fv.Type()); {
		case k&ctxkind.Carrier != 0:
			return true
		case k&ctxkind.Context != 0:
			return !closureIsFresh(f, fv)
		case k&ctxkind.Implements != 0:
			return true
		}
		return r.reads(fv) || closureHoldsCapturing(f, fv, kinds, seen)
	})
}

// closureReader tells whether a closure reads a context from the values it
// captures.
type closureReader struct {
	// flow follows the parts read.
	flow *Flow
	// kinds judges Holds for the closure's package.
	kinds *ctxkind.Classifier
	seen  map[ssa.Value]bool
}

// reads reports whether a context or a carrier is read from v, through the
// fields and elements of what it holds and the local variables it is copied
// to, or a method is called on a value that returns one, or a call takes a
// value that holds one. A context
// that is only ever fresh does not count.
func (r *closureReader) reads(v ssa.Value) bool {
	if r.seen[v] {
		return false
	}
	r.seen[v] = true
	for _, ref := range *v.Referrers() {
		var part ssa.Value
		switch x := ref.(type) {
		case ssa.CallInstruction:
			if r.callsContextMethod(x.Common(), v) || r.callsWithHolder(x.Common(), v, v) {
				return true
			}
			continue
		case *ssa.MakeInterface:
			if r.givesHolderAsInterface(x, v) {
				return true
			}
			continue
		case *ssa.Store:
			// A copy to a local variable, as c := *p makes, is read the same.
			a, ok := x.Addr.(*ssa.Alloc)
			if !ok || x.Val != v || !r.reads(a) {
				continue
			}
			return true
		case *ssa.UnOp:
			if x.Op != token.MUL {
				continue
			}
			part = x
		case *ssa.FieldAddr:
			part = x
		case *ssa.Field:
			part = x
		case *ssa.IndexAddr:
			part = x
		case *ssa.Index:
			part = x
		default:
			continue
		}
		if r.carries(part) || r.reads(part) {
			return true
		}
	}
	return false
}

// carries reports whether part, read from a captured value, is a carrier, or
// a context that is not only ever fresh. The address of a context is judged
// by the loads from it, unless it is used some other way.
func (r *closureReader) carries(part ssa.Value) bool {
	t := part.Type()
	switch {
	case r.kinds.Is(t, ctxkind.Carrier):
		return true
	case !r.kinds.Is(t, ctxkind.Context|ctxkind.Implements):
		return false
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

// callsWithHolder reports whether call takes v, as the receiver of a method
// or as any argument, and held, past its pointers, holds a context or a
// carrier that the analyzed code can have put there ([ctxkind.Holds]). held
// is v, or the value v converts to an interface. Whether the callee reads
// it is not looked into, as for a function or method spawned directly with
// the same value. A builtin, as append or len, does not count.
func (r *closureReader) callsWithHolder(call *ssa.CallCommon, v, held ssa.Value) bool {
	if _, isBuiltin := call.Value.(*ssa.Builtin); isBuiltin || !slices.Contains(call.Args, v) {
		return false
	}
	return r.kinds.Is(typeutil.UnwrapPointer(held.Type()), ctxkind.Holds)
}

// givesHolderAsInterface reports whether conv, v converted to an interface,
// is given to a call as [closureReader.callsWithHolder] judges it.
func (r *closureReader) givesHolderAsInterface(conv *ssa.MakeInterface, v ssa.Value) bool {
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

// closureHoldsCapturing reports whether the func variable fv captures holds
// a closure that captures a context.
func closureHoldsCapturing(f *Flow, fv *ssa.FreeVar, kinds *ctxkind.Classifier, seen map[*ssa.Function]bool) bool {
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
		return closureCaptures(f, fn, kinds, seen)
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
// its start.
func ClosureCallsDeriver(closure *ssa.Function, matcher *deriver.Matcher, given ...*types.Func) ClosureDerivation {
	if closure == nil || matcher.IsEmpty() {
		return ClosureDerivation{FoundAtStart: true}
	}

	var calls []closureDeriverCall
	for _, fn := range given {
		calls = append(calls, closureDeriverCall{fn: fn})
	}
	for site := range Runs(closure, RunOptions{}) {
		if site.isGo {
			continue
		}
		if fn := ExtractCalledFunc(site.Call); fn != nil {
			calls = append(calls, closureDeriverCall{fn: fn, inDefer: site.inDefer})
		}
	}

	// Check if any OR group is satisfied at start
	for _, andGroup := range matcher.OrGroups {
		if closureCallsGroup(calls, andGroup, false) {
			return ClosureDerivation{FoundAtStart: true}
		}
	}

	// Check if deriver is only in defer
	for _, andGroup := range matcher.OrGroups {
		if closureCallsGroup(calls, andGroup, true) {
			return ClosureDerivation{FoundOnlyInDefer: true}
		}
	}

	return ClosureDerivation{}
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
