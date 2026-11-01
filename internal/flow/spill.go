package flow

import (
	"golang.org/x/tools/go/ssa"
)

// spilledLoads follows st, a store of a value to a local variable, as SSA
// spills a variable a closure captures, to the loads of that variable: in
// the function of st, and in each closure that captures it, out to the
// closures those make in turn. binders are the closures that capture it,
// each as it is made. ok is false unless st is the one store to the
// variable, and the variable, and each free variable a closure has for it,
// is only loaded or captured, so that every load reads the value st
// stores, or nothing yet.
func spilledLoads(st *ssa.Store) (loads []*ssa.UnOp, binders []*ssa.MakeClosure, ok bool) {
	a, isAlloc := st.Addr.(*ssa.Alloc)
	if !isAlloc {
		return nil, nil, false
	}
	var follow func(v ssa.Value) bool
	follow = func(v ssa.Value) bool {
		for _, ref := range *v.Referrers() {
			switch x := ref.(type) {
			case *ssa.Store:
				if x != st {
					return false
				}
			case *ssa.UnOp:
				// A pointer is only loaded from.
				loads = append(loads, x)
			case *ssa.MakeClosure:
				fn := x.Fn.(*ssa.Function)
				binders = append(binders, x)
				for i, b := range x.Bindings {
					if b == v && !follow(fn.FreeVars[i]) {
						return false
					}
				}
			default:
				return false
			}
		}
		return true
	}
	if !follow(a) {
		return nil, nil, false
	}
	return loads, binders, true
}

// spilledUses gives v and the values loaded from each local variable v is
// spilled to, as [spilledLoads] follows them.
//
//declscope:shared // closure.go reads a parameter a nested closure captures
func spilledUses(v ssa.Value) []ssa.Value {
	uses := []ssa.Value{v}
	for _, ref := range *v.Referrers() {
		st, isStore := ref.(*ssa.Store)
		if !isStore || st.Val != v {
			continue
		}
		loads, _, ok := spilledLoads(st)
		if !ok {
			continue
		}
		for _, load := range loads {
			uses = append(uses, load)
		}
	}
	return uses
}
