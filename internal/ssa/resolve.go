package ssa

import (
	"go/ast"
	"go/token"
	"slices"

	"golang.org/x/tools/go/ssa"
)

// FuncResolver judges the functions a func-typed SSA value can hold. It
// follows the value back through Phi nodes, loads of local variables, type
// changes and calls of factories to each function literal, and asks Closure
// about it. A value it cannot follow passes, so that only what is resolved is
// reported.
type FuncResolver struct {
	// Closure judges one function literal.
	Closure func(fn *ssa.Function) bool
	// Call, when set, judges the result of a call before the factory rules
	// do. It returns handled false to leave the call to them.
	Call func(call *ssa.CallCommon) (passes, handled bool)
	// FactoryCall, when set, passes a call of a factory without looking at
	// what it returns, as when a context is passed to it.
	FactoryCall func(call *ssa.CallCommon) bool
	// Factory, when set, passes a factory function without looking at what
	// it returns, as when it takes a context.
	Factory func(fn *ssa.Function) bool
}

// Passes reports whether every function v can hold passes.
func (r *FuncResolver) Passes(v ssa.Value) bool {
	return r.passes(v, make(map[ssa.Value]bool))
}

// ResultPasses reports whether calling v gives a function that passes, as
// for a callback that makes tasks.
func (r *FuncResolver) ResultPasses(v ssa.Value) bool {
	return r.resultPasses(v, 1, make(map[ssa.Value]bool))
}

func (r *FuncResolver) passes(v ssa.Value, seen map[ssa.Value]bool) bool {
	if v == nil || seen[v] {
		return true
	}
	seen[v] = true

	switch v := v.(type) {
	case *ssa.MakeClosure:
		fn, _ := v.Fn.(*ssa.Function)
		return r.closurePasses(fn)
	case *ssa.Function:
		return r.closurePasses(v)
	case *ssa.Phi:
		return !slices.ContainsFunc(v.Edges, func(e ssa.Value) bool { return !r.passes(e, seen) })
	case *ssa.ChangeType:
		return r.passes(v.X, seen)
	case *ssa.UnOp:
		if v.Op != token.MUL {
			return true
		}
		stored, ok := resolvedStores(v.X, v)
		return !ok || !slices.ContainsFunc(stored, func(s ssa.Value) bool { return !r.passes(s, seen) })
	case *ssa.Call:
		return r.callPasses(v.Common(), seen)
	}
	return true
}

// closurePasses judges fn when it is a function literal.
func (r *FuncResolver) closurePasses(fn *ssa.Function) bool {
	if fn == nil {
		return true
	}
	if _, ok := fn.Syntax().(*ast.FuncLit); !ok {
		return true
	}
	return r.Closure(fn)
}

// callPasses judges the result of call, the function a factory returns.
func (r *FuncResolver) callPasses(call *ssa.CallCommon, seen map[ssa.Value]bool) bool {
	if r.Call != nil {
		if passes, handled := r.Call(call); handled {
			return passes
		}
	}
	if call.IsInvoke() {
		return true
	}
	if r.FactoryCall != nil && r.FactoryCall(call) {
		return true
	}
	return r.resultPasses(call.Value, 1, seen)
}

// resultPasses reports whether calling v depth times gives a function that
// passes. Of a factory, any value it returns passing is enough.
func (r *FuncResolver) resultPasses(v ssa.Value, depth int, seen map[ssa.Value]bool) bool {
	if depth == 0 {
		return r.passes(v, seen)
	}
	switch x := v.(type) {
	case *ssa.MakeClosure:
		fn, _ := x.Fn.(*ssa.Function)
		return r.factoryFuncPasses(fn, depth, seen)
	case *ssa.Function:
		return r.factoryFuncPasses(x, depth, seen)
	case *ssa.Call:
		if r.FactoryCall != nil && r.FactoryCall(x.Common()) {
			return true
		}
		if x.Common().IsInvoke() {
			return true
		}
		return r.resultPasses(x.Common().Value, depth+1, seen)
	case *ssa.Phi:
		return !slices.ContainsFunc(x.Edges, func(e ssa.Value) bool { return !r.resultPasses(e, depth, seen) })
	case *ssa.ChangeType:
		return r.resultPasses(x.X, depth, seen)
	case *ssa.UnOp:
		if x.Op != token.MUL {
			return true
		}
		stored, ok := resolvedStores(x.X, x)
		return !ok || !slices.ContainsFunc(stored, func(s ssa.Value) bool { return !r.resultPasses(s, depth, seen) })
	}
	return true
}

// factoryFuncPasses reports whether fn takes what it needs, or calling one
// of the values it returns depth-1 more times gives a function that passes.
func (r *FuncResolver) factoryFuncPasses(fn *ssa.Function, depth int, seen map[ssa.Value]bool) bool {
	if fn != nil && fn.Blocks == nil {
		fn = fn.Origin()
	}
	if fn == nil || fn.Blocks == nil || fn.Synthetic != "" {
		return true
	}
	if r.Factory != nil && r.Factory(fn) {
		return true
	}
	returned := resolvedReturns(fn)
	return len(returned) == 0 || slices.ContainsFunc(returned, func(v ssa.Value) bool { return r.resultPasses(v, depth-1, seen) })
}

// ResolvedArgs gives the SSA value of each argument of call, in the order of
// its AST arguments. Arguments packed into a variadic slice are given one by
// one. It returns nil when the two do not line up, as for f(g()) with g
// returning several values.
func ResolvedArgs(common *ssa.CallCommon, call *ast.CallExpr) []ssa.Value {
	args := common.Args
	sig := common.Signature()
	if !common.IsInvoke() && sig.Recv() != nil && len(args) > 0 {
		args = args[1:]
	}

	if !sig.Variadic() || call.Ellipsis.IsValid() {
		if len(args) != len(call.Args) {
			return nil
		}
		return args
	}

	fixed := sig.Params().Len() - 1
	if len(args) != fixed+1 || len(call.Args) < fixed {
		return nil
	}
	elems := resolvedSliceElems(args[fixed], len(call.Args)-fixed)
	if elems == nil {
		return nil
	}
	return append(slices.Clone(args[:fixed]), elems...)
}

// ResolvedReceiver gives the receiver of a static method call, or nil.
func ResolvedReceiver(common *ssa.CallCommon) ssa.Value {
	if common.IsInvoke() || common.Signature().Recv() == nil || len(common.Args) == 0 {
		return nil
	}
	return common.Args[0]
}

// ResolvedValues gives each value v can be, through Phi nodes, type changes
// and loads of local variables. ok is false when a load cannot be followed.
func ResolvedValues(v ssa.Value) (values []ssa.Value, ok bool) {
	ok = true
	seen := make(map[ssa.Value]bool)
	var visit func(v ssa.Value)
	visit = func(v ssa.Value) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		switch x := v.(type) {
		case *ssa.Phi:
			for _, e := range x.Edges {
				visit(e)
			}
		case *ssa.ChangeType:
			visit(x.X)
		case *ssa.UnOp:
			if x.Op != token.MUL {
				values = append(values, v)
				return
			}
			stored, followed := resolvedStores(x.X, x)
			if !followed {
				ok = false
				return
			}
			for _, s := range stored {
				visit(s)
			}
		default:
			values = append(values, v)
		}
	}
	visit(v)
	return values, ok
}

// resolvedSliceElems gives the n values a variadic call packed into slice.
func resolvedSliceElems(slice ssa.Value, n int) []ssa.Value {
	if n == 0 {
		return []ssa.Value{}
	}
	sl, ok := slice.(*ssa.Slice)
	if !ok {
		return nil
	}
	array, ok := sl.X.(*ssa.Alloc)
	if !ok {
		return nil
	}
	elems := make([]ssa.Value, n)
	for _, ref := range *array.Referrers() {
		ia, ok := ref.(*ssa.IndexAddr)
		if !ok {
			continue
		}
		idx, ok := ia.Index.(*ssa.Const)
		if !ok {
			continue
		}
		i := int(idx.Int64())
		if i < 0 || i >= n {
			continue
		}
		for _, use := range *ia.Referrers() {
			if st, ok := use.(*ssa.Store); ok && st.Addr == ia {
				elems[i] = st.Val
			}
		}
	}
	if slices.Contains(elems, nil) {
		return nil
	}
	return elems
}

// resolvedReturns gives the first result of each return of fn.
func resolvedReturns(fn *ssa.Function) []ssa.Value {
	var returned []ssa.Value
	for _, block := range fn.Blocks {
		if ret, ok := block.Instrs[len(block.Instrs)-1].(*ssa.Return); ok && len(ret.Results) > 0 {
			returned = append(returned, ret.Results[0])
		}
	}
	return returned
}

// resolvedStores gives the values stored at addr that a load at the
// instruction at can read. For a local variable of at's function, only the
// stores that reach at count. For a variable captured from an outer function,
// every store there counts. ok is false when the variable may be written in a
// way not followed: through a closure, or through its address.
func resolvedStores(addr ssa.Value, at ssa.Instruction) (stored []ssa.Value, ok bool) {
	switch a := addr.(type) {
	case *ssa.Alloc:
		if !resolvedAllocOnlyStored(a) {
			return nil, false
		}
		if at != nil && at.Parent() == a.Parent() {
			return resolvedReachingStores(a, at), true
		}
		for _, ref := range *a.Referrers() {
			if st, ok := ref.(*ssa.Store); ok && st.Addr == a {
				stored = append(stored, st.Val)
			}
		}
		return stored, true

	case *ssa.FreeVar:
		binding, ok := resolvedBinding(a)
		if !ok {
			return nil, false
		}
		return resolvedStores(binding, nil)
	}
	return nil, false
}

// resolvedBinding gives the value the closure owning fv was made with for it.
func resolvedBinding(fv *ssa.FreeVar) (ssa.Value, bool) {
	fn := fv.Parent()
	idx := slices.Index(fn.FreeVars, fv)
	parent := fn.Parent()
	if idx < 0 || parent == nil {
		return nil, false
	}
	var binding ssa.Value
	for _, block := range parent.Blocks {
		for _, instr := range block.Instrs {
			mc, ok := instr.(*ssa.MakeClosure)
			if !ok || mc.Fn != fn {
				continue
			}
			if binding != nil && binding != mc.Bindings[idx] {
				return nil, false
			}
			binding = mc.Bindings[idx]
		}
	}
	return binding, binding != nil
}

// resolvedAllocOnlyStored reports whether a local variable is only loaded
// and stored, and every closure that captures it only reads it.
func resolvedAllocOnlyStored(a *ssa.Alloc) bool {
	for _, ref := range *a.Referrers() {
		switch r := ref.(type) {
		case *ssa.Store:
			if r.Addr != a {
				return false
			}
		case *ssa.UnOp:
			if r.Op != token.MUL {
				return false
			}
		case *ssa.MakeClosure:
			if !resolvedCaptureOnlyRead(r, a) {
				return false
			}
		case *ssa.DebugRef:
		default:
			return false
		}
	}
	return true
}

// resolvedCaptureOnlyRead reports whether the closure mc makes only reads
// the variable a it captures.
func resolvedCaptureOnlyRead(mc *ssa.MakeClosure, a ssa.Value) bool {
	fn, ok := mc.Fn.(*ssa.Function)
	if !ok {
		return false
	}
	for i, b := range mc.Bindings {
		if b != a {
			continue
		}
		fv := fn.FreeVars[i]
		for _, ref := range *fv.Referrers() {
			switch r := ref.(type) {
			case *ssa.UnOp:
				if r.Op != token.MUL {
					return false
				}
			case *ssa.MakeClosure:
				if !resolvedCaptureOnlyRead(r, fv) {
					return false
				}
			case *ssa.DebugRef:
			default:
				return false
			}
		}
	}
	return true
}

// resolvedReachingStores gives the values of the stores to a that reach at:
// the last store on each path to it.
func resolvedReachingStores(a *ssa.Alloc, at ssa.Instruction) []ssa.Value {
	var stored []ssa.Value
	visited := make(map[*ssa.BasicBlock]bool)
	var scan func(b *ssa.BasicBlock, end int)
	scan = func(b *ssa.BasicBlock, end int) {
		for i := end - 1; i >= 0; i-- {
			if st, ok := b.Instrs[i].(*ssa.Store); ok && st.Addr == a {
				stored = append(stored, st.Val)
				return
			}
		}
		for _, pred := range b.Preds {
			if !visited[pred] {
				visited[pred] = true
				scan(pred, len(pred.Instrs))
			}
		}
	}
	block := at.Block()
	scan(block, slices.Index(block.Instrs, at))
	return stored
}
