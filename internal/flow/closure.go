package flow

import (
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/carrier"
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
// context, and a method called on a captured value that holds a context the
// analyzed code can have put there, as a method spawned directly is left
// alone. A captured closure that captures a context counts too.
//
// module is the path of the analyzed package's module, or "" when the driver
// does not give it. A field the analyzed code can set is exported, or is
// declared in the analyzed package or in a package of module. When module is
// "", any field outside the standard library may be, so it counts.
func ClosureCapturesContext(closure *ssa.Function, carriers []carrier.Carrier, module string) bool {
	return closureCaptures(closure, carriers, module, make(map[*ssa.Function]bool))
}

func closureCaptures(closure *ssa.Function, carriers []carrier.Carrier, module string, seen map[*ssa.Function]bool) bool {
	if closure == nil || seen[closure] {
		return false
	}
	seen[closure] = true

	r := &closureReader{
		carriers: carriers,
		iface:    closureContextInterface(closure.Prog),
		pkg:      closurePackage(closure),
		module:   module,
		seen:     make(map[ssa.Value]bool),
	}
	return slices.ContainsFunc(closure.FreeVars, func(fv *ssa.FreeVar) bool {
		switch {
		case carrier.IsCarrierType(fv.Type(), carriers):
			return true
		case typeutil.IsContextType(fv.Type()):
			return !closureFreeVarIsFresh(fv)
		case r.implementsContext(fv.Type()):
			return true
		}
		return r.reads(fv) || closureHoldsCapturing(fv, carriers, module, seen)
	})
}

// closureReader tells whether a closure reads a context from the values it
// captures.
type closureReader struct {
	carriers []carrier.Carrier
	// iface is context.Context, or nil when the program has no context
	// package, so that nothing implements it.
	iface *types.Interface
	// pkg is the analyzed package, and module the path of its module, or "".
	pkg    *types.Package
	module string
	seen   map[ssa.Value]bool
}

// reads reports whether a context or a carrier is read from v, through the
// fields and elements of what it holds and the local variables it is copied
// to, or a method is called on a value that returns or holds one. A context
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
			if r.callsContextMethod(x.Common(), v) || r.callsHolderMethod(x.Common(), v) {
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
	case carrier.IsCarrierType(t, r.carriers):
		return true
	case !typeutil.IsContextType(t) && !r.implementsContext(t):
		return false
	}
	if _, isPtr := t.Underlying().(*types.Pointer); isPtr {
		return slices.ContainsFunc(*part.Referrers(), func(ref ssa.Instruction) bool {
			load, ok := ref.(*ssa.UnOp)
			return !ok || load.Op != token.MUL
		})
	}
	values, ok := resolvedPartValues(part)
	return !ok || len(values) == 0 || slices.ContainsFunc(values, func(v ssa.Value) bool { return !IsFreshContext(v) })
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
		t := results.At(i).Type()
		if typeutil.IsContextType(t) || carrier.IsCarrierType(t, r.carriers) || r.implementsContext(t) {
			return true
		}
	}
	return false
}

// callsHolderMethod reports whether call is a method called on v, and v
// holds a context or a carrier that the analyzed code can have put there.
// Whether the method reads it is not looked into, as for a method spawned
// directly.
func (r *closureReader) callsHolderMethod(call *ssa.CallCommon, v ssa.Value) bool {
	if call.IsInvoke() || call.Signature().Recv() == nil || len(call.Args) == 0 || call.Args[0] != v {
		return false
	}
	return r.holdsContext(typeutil.UnwrapPointer(v.Type()), make(map[types.Type]bool))
}

// holdsContext reports whether a value of type t has a context or a carrier
// in a field, or in a field of a struct it holds by value. A struct it
// points to is not looked into. Only a field the analyzed code can set
// counts, so the unexported context of a *testing.T or an *http.Request
// does not (#191).
func (r *closureReader) holdsContext(t types.Type, seen map[types.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for f := range st.Fields() {
		if !r.canSet(f) {
			continue
		}
		ft := f.Type()
		if typeutil.IsContextType(ft) || carrier.IsCarrierType(ft, r.carriers) || r.implementsContext(ft) ||
			r.holdsContext(ft, seen) {
			return true
		}
	}
	return false
}

// canSet reports whether the analyzed code can set the field f: it is
// exported, or declared in the analyzed package or a package of its module.
// When the module is not known, a field outside the standard library may be
// of the module, so it counts, as a report there could be false.
func (r *closureReader) canSet(f *types.Var) bool {
	if f.Exported() || f.Pkg() == r.pkg {
		return true
	}
	// An unexported field always has a package.
	path := f.Pkg().Path()
	if r.module == "" {
		return !closureIsStdPath(path)
	}
	return path == r.module || strings.HasPrefix(path, r.module+"/")
}

// closureIsStdPath reports whether path is of the standard library: its
// first element has no dot, as "testing" and "net/http".
func closureIsStdPath(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}

// implementsContext reports whether t, past its pointers, implements
// context.Context, as a struct that embeds one does.
func (r *closureReader) implementsContext(t types.Type) bool {
	if r.iface == nil {
		return false
	}
	t = typeutil.UnwrapPointer(t)
	return types.Implements(t, r.iface) || types.Implements(types.NewPointer(t), r.iface)
}

// closurePackage gives the package of closure, or nil when SSA has none, as
// for a wrapper.
func closurePackage(closure *ssa.Function) *types.Package {
	if closure.Pkg == nil {
		return nil
	}
	return closure.Pkg.Pkg
}

// closureContextInterface gives the context.Context interface, or nil when
// prog has no context package.
func closureContextInterface(prog *ssa.Program) *types.Interface {
	pkg := prog.ImportedPackage("context")
	if pkg == nil {
		return nil
	}
	obj := pkg.Pkg.Scope().Lookup("Context")
	if obj == nil {
		return nil
	}
	iface, _ := obj.Type().Underlying().(*types.Interface)
	return iface
}

// closureHoldsCapturing reports whether the func variable fv captures holds
// a closure that captures a context.
func closureHoldsCapturing(fv *ssa.FreeVar, carriers []carrier.Carrier, module string, seen map[*ssa.Function]bool) bool {
	if _, isFunc := typeutil.UnwrapPointer(fv.Type()).Underlying().(*types.Signature); !isFunc {
		return false
	}
	values, ok := resolvedFreeVarValues(fv)
	return ok && slices.ContainsFunc(values, func(v ssa.Value) bool {
		mc, ok := v.(*ssa.MakeClosure)
		if !ok {
			return false
		}
		fn, _ := mc.Fn.(*ssa.Function)
		return closureCaptures(fn, carriers, module, seen)
	})
}

// closureFreeVarIsFresh reports whether every value fv can hold is a fresh
// context, as IsFreshContext judges it.
func closureFreeVarIsFresh(fv *ssa.FreeVar) bool {
	values, ok := resolvedFreeVarValues(fv)
	return ok && len(values) > 0 && !slices.ContainsFunc(values, func(v ssa.Value) bool { return !IsFreshContext(v) })
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
