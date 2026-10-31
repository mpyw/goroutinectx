package flow

import (
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/carrier"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// HasContextParam reports whether fn takes a context.Context or one of
// carriers. It reads the SSA parameters, so unlike [ContextParamNames] it
// counts a blank parameter, as func(_ context.Context) has, and the receiver
// of a method.
func HasContextParam(fn *ssa.Function, carriers []carrier.Carrier) bool {
	return slices.ContainsFunc(fn.Params, func(p *ssa.Parameter) bool {
		return typeutil.IsContextType(p.Type()) || carrier.IsCarrierType(p.Type(), carriers)
	})
}

// ContextParamNames gives the names of the parameters of fnType that are a
// context.Context or one of carriers, in order. It reads the syntax, so
// unlike [HasContextParam] it leaves out a blank parameter, which cannot be
// used, and a receiver, which is not in fnType. The runner checks the nodes
// of a function only when this gives a name, and reports the first.
func ContextParamNames(info *types.Info, fnType *ast.FuncType, carriers []carrier.Carrier) []string {
	if fnType == nil || fnType.Params == nil {
		return nil
	}
	var names []string
	for _, field := range fnType.Params.List {
		typ := info.TypeOf(field.Type)
		if typ == nil || (!typeutil.IsContextType(typ) && !carrier.IsCarrierType(typ, carriers)) {
			continue
		}
		for _, name := range field.Names {
			if name.Name != "_" {
				names = append(names, name.Name)
			}
		}
	}
	return names
}

// IsFreshContext reports whether every value v can be is a call of
// Background or TODO of package context or golang.org/x/net/context: a
// context that carries nothing of the caller's.
func IsFreshContext(v ssa.Value) bool {
	values, ok := ResolvedValues(v)
	if !ok || len(values) == 0 {
		return false
	}
	return !slices.ContainsFunc(values, func(v ssa.Value) bool {
		call, ok := v.(*ssa.Call)
		if !ok {
			return true
		}
		fn := ExtractCalledFunc(call.Common())
		return fn == nil || fn.Pkg() == nil ||
			(fn.Pkg().Path() != "context" && fn.Pkg().Path() != "golang.org/x/net/context") ||
			(fn.Name() != "Background" && fn.Name() != "TODO")
	})
}

// ArgsInvolveContext reports whether any argument of call is built from a
// context or a carrier: one is the argument, or among the values it is made
// from, as a closure that captures one.
func ArgsInvolveContext(call *ssa.CallCommon, carriers []carrier.Carrier) bool {
	seen := make(map[ssa.Value]bool)
	return slices.ContainsFunc(call.Args, func(arg ssa.Value) bool { return contextInvolved(arg, carriers, seen) })
}

// GivesOnlyFreshContext reports whether call gives a fresh
// context.Background() or TODO(), and every other argument is sure to hold
// no context. A value of a basic type, or a function that captures nothing,
// holds none. A pointer, a struct, an interface, a slice or a bound method
// may hold one, unless its type cannot (#192).
func GivesOnlyFreshContext(call *ssa.CallCommon, carriers []carrier.Carrier) bool {
	given := false
	for _, arg := range call.Args {
		if typeutil.IsContextType(arg.Type()) && IsFreshContext(arg) {
			given = true
			continue
		}
		if valueMayHoldContext(arg, carriers) {
			return false
		}
	}
	return given
}

// valueMayHoldContext reports whether v may hold a context or a carrier. A
// function holds what it captures: nothing for a function that is not a
// closure, and its bindings for a closure or a bound method. A function
// value that cannot be followed may hold one.
func valueMayHoldContext(v ssa.Value, carriers []carrier.Carrier) bool {
	if _, isFunc := v.Type().Underlying().(*types.Signature); !isFunc {
		return typeMayHoldContext(v.Type(), carriers, make(map[types.Type]bool))
	}
	switch v := v.(type) {
	case *ssa.Function, *ssa.Const:
		return false
	case *ssa.MakeClosure:
		return slices.ContainsFunc(v.Bindings, func(b ssa.Value) bool { return valueMayHoldContext(b, carriers) })
	}
	return true
}

// typeMayHoldContext reports whether a value of type t may hold a context or
// a carrier: t is one, an interface, a function, a type parameter or an
// unsafe.Pointer, or is made of one of these.
func typeMayHoldContext(t types.Type, carriers []carrier.Carrier, seen map[types.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	if typeutil.IsContextType(t) || carrier.IsCarrierType(t, carriers) {
		return true
	}
	elem := func(e types.Type) bool { return typeMayHoldContext(e, carriers, seen) }
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return u.Kind() == types.UnsafePointer
	case *types.Pointer:
		return elem(u.Elem())
	case *types.Slice:
		return elem(u.Elem())
	case *types.Array:
		return elem(u.Elem())
	case *types.Chan:
		return elem(u.Elem())
	case *types.Map:
		return elem(u.Key()) || elem(u.Elem())
	case *types.Struct:
		for f := range u.Fields() {
			if elem(f.Type()) {
				return true
			}
		}
		return false
	}
	return true
}

func contextInvolved(v ssa.Value, carriers []carrier.Carrier, seen map[ssa.Value]bool) bool {
	if v == nil || seen[v] {
		return false
	}
	seen[v] = true
	if typeutil.IsContextType(v.Type()) || carrier.IsCarrierType(v.Type(), carriers) {
		return true
	}
	instr, ok := v.(ssa.Instruction)
	if !ok {
		return false
	}
	for _, op := range instr.Operands(nil) {
		if op != nil && contextInvolved(*op, carriers, seen) {
			return true
		}
	}
	return false
}
