package ssa

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
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
	case *ssa.UnOp, *ssa.Lookup:
		stored, isLoad, ok := resolvedLoaded(v, 0)
		return !isLoad || !ok || !slices.ContainsFunc(stored, func(s ssa.Value) bool { return !r.passes(s, seen) })
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

// callPasses judges the result of call, the function a factory returns. A
// call given a function that passes passes too: a decorator, as one that
// recovers panics, runs the function it is given.
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
	if slices.ContainsFunc(call.Args, func(arg ssa.Value) bool {
		_, isFunc := arg.Type().Underlying().(*types.Signature)
		return isFunc && r.passes(arg, seen)
	}) {
		return true
	}
	return r.resultPasses(call.Value, 1, seen)
}

// resultPasses reports whether calling v depth times gives a function that
// passes. Of a factory, any value it returns passing is enough. A result
// more than resolvedMaxDepth calls away passes, so that a factory whose
// type returns itself, as type stateFn func() stateFn, ends.
func (r *FuncResolver) resultPasses(v ssa.Value, depth int, seen map[ssa.Value]bool) bool {
	if depth == 0 {
		return r.passes(v, seen)
	}
	if depth > resolvedMaxDepth {
		return true
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
	case *ssa.UnOp, *ssa.Lookup:
		stored, isLoad, ok := resolvedLoaded(x, 0)
		return !isLoad || !ok || !slices.ContainsFunc(stored, func(s ssa.Value) bool { return !r.resultPasses(s, depth, seen) })
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

// ResolvedReceiver gives the receiver of a static method call, or of a call
// of a method value, which a bound method wrapper holds. It gives nil for
// any other call.
func ResolvedReceiver(common *ssa.CallCommon) ssa.Value {
	if common.IsInvoke() {
		return nil
	}
	if mc, ok := common.Value.(*ssa.MakeClosure); ok {
		if fn, ok := mc.Fn.(*ssa.Function); ok && fn.Synthetic != "" && len(mc.Bindings) == 1 {
			return mc.Bindings[0]
		}
		return nil
	}
	if common.Signature().Recv() == nil || len(common.Args) == 0 {
		return nil
	}
	return common.Args[0]
}

// ResolvedValues gives each value v can be, through Phi nodes, type changes,
// loads of local variables and of their parts, and map lookups. ok is false
// when a load cannot be followed.
func ResolvedValues(v ssa.Value) (values []ssa.Value, ok bool) {
	return resolvedValues(v, 0)
}

func resolvedValues(v ssa.Value, depth int) (values []ssa.Value, ok bool) {
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
		default:
			stored, isLoad, followed := resolvedLoaded(v, depth)
			switch {
			case !isLoad:
				values = append(values, v)
			case !followed:
				ok = false
			default:
				for _, s := range stored {
					visit(s)
				}
			}
		}
	}
	visit(v)
	return values, ok
}

// resolvedLoaded gives the values v can read when it is a load through a
// pointer or a map lookup, and isLoad false for any other value. ok is false
// when the load cannot be followed.
func resolvedLoaded(v ssa.Value, depth int) (values []ssa.Value, isLoad, ok bool) {
	switch x := v.(type) {
	case *ssa.UnOp:
		if x.Op != token.MUL {
			return nil, false, false
		}
		values, ok = resolvedStoresAt(x.X, x, depth)
		return values, true, ok
	case *ssa.Lookup:
		values, ok = resolvedLookups(x, depth)
		return values, true, ok
	}
	return nil, false, false
}

// resolvedFreeVarValues gives each value the variable a closure captures as
// fv can hold: the value it was made with, or for a variable captured by
// reference, every value stored to it. ok is false when they cannot all be
// followed.
//
//declscope:shared // closure.go tells a captured fresh context from one that carries ctx
func resolvedFreeVarValues(fv *ssa.FreeVar) (values []ssa.Value, ok bool) {
	if _, isPtr := fv.Type().Underlying().(*types.Pointer); !isPtr {
		binding, ok := resolvedBinding(fv)
		if !ok {
			return nil, false
		}
		return []ssa.Value{binding}, true
	}
	return resolvedStores(fv, nil)
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
// the stores there that reach the closure's creation count, and every store
// after it, since the closure may run any time after; with no at, every
// store counts. ok is false when the variable may be written in a
// way not followed: through a closure, or through its address.
func resolvedStores(addr ssa.Value, at ssa.Instruction) (stored []ssa.Value, ok bool) {
	return resolvedStoresAt(addr, at, 0)
}

// resolvedStoresAt is resolvedStores, depth steps into the parts of
// variables. A struct field, or an element at a constant index, is followed
// as a part of the variable it is in.
func resolvedStoresAt(addr ssa.Value, at ssa.Instruction, depth int) (stored []ssa.Value, ok bool) {
	switch a := addr.(type) {
	case *ssa.Alloc:
		if !resolvedAllocOnlyStored(a) {
			return nil, false
		}
		if at != nil && at.Parent() == a.Parent() {
			return resolvedReachingStores(a, at), true
		}
		take := func(instr ssa.Instruction) bool {
			st, ok := instr.(*ssa.Store)
			if ok && st.Addr == a {
				stored = append(stored, st.Val)
			}
			return ok && st.Addr == a
		}
		if !resolvedScanFromClosure(at, a, take) {
			for _, ref := range *a.Referrers() {
				take(ref)
			}
		}
		return stored, true

	case *ssa.FreeVar:
		binding, ok := resolvedBinding(a)
		if !ok {
			return nil, false
		}
		return resolvedStoresAt(binding, at, depth)

	case *ssa.FieldAddr, *ssa.IndexAddr:
		return resolvedPartStores(a, nil, at, depth)
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
	resolvedScanBack(at, a, func(instr ssa.Instruction) bool {
		st, ok := instr.(*ssa.Store)
		if ok && st.Addr == a {
			stored = append(stored, st.Val)
		}
		return ok && st.Addr == a
	})
	return stored
}

// resolvedScanBack visits the instructions before at, on every path to it,
// the latest first. visit returns true to stop the path there. A path also
// stops at origin, the instruction that makes the variable or map scanned
// for: what comes before it belongs to an earlier one, as in a loop.
func resolvedScanBack(at, origin ssa.Instruction, visit func(ssa.Instruction) bool) {
	visited := make(map[*ssa.BasicBlock]bool)
	var scan func(b *ssa.BasicBlock, end int)
	scan = func(b *ssa.BasicBlock, end int) {
		for i := end - 1; i >= 0; i-- {
			if b.Instrs[i] == origin || visit(b.Instrs[i]) {
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
}

// resolvedScanFromClosure visits the instructions of origin's function that
// a load at at, in a closure of that function, can see: those resolvedScanBack
// visits from the closure's creation, and every one after it, which visit
// cannot stop. It reports false when it cannot tell: for no at, or when the
// closure is not made exactly once.
func resolvedScanFromClosure(at, origin ssa.Instruction, visit func(ssa.Instruction) bool) bool {
	if at == nil {
		return false
	}
	outer := origin.Parent()
	fn := at.Parent()
	for fn != nil && fn.Parent() != outer {
		fn = fn.Parent()
	}
	if fn == nil {
		return false
	}
	var made *ssa.MakeClosure
	for _, block := range outer.Blocks {
		for _, instr := range block.Instrs {
			if mc, ok := instr.(*ssa.MakeClosure); ok && mc.Fn == fn {
				if made != nil {
					return false
				}
				made = mc
			}
		}
	}
	if made == nil {
		return false
	}
	resolvedScanBack(made, origin, visit)
	resolvedScanForward(made, origin, func(instr ssa.Instruction) { visit(instr) })
	return true
}

// resolvedScanForward visits the instructions after from, on every path from
// it. A path stops at origin, as in resolvedScanBack.
func resolvedScanForward(from, origin ssa.Instruction, visit func(ssa.Instruction)) {
	visited := make(map[*ssa.BasicBlock]bool)
	var scan func(b *ssa.BasicBlock, start int)
	scan = func(b *ssa.BasicBlock, start int) {
		for _, instr := range b.Instrs[start:] {
			if instr == origin {
				return
			}
			visit(instr)
		}
		for _, succ := range b.Succs {
			if !visited[succ] {
				visited[succ] = true
				scan(succ, 0)
			}
		}
	}
	block := from.Block()
	scan(block, slices.Index(block.Instrs, from)+1)
}

// resolvedMaxDepth bounds how many parts and loads a part is followed
// through, so that a value that refers to itself, as p = p.next does, ends.
const resolvedMaxDepth = 16

// resolvedPartStores gives the values stored at the part of a variable addr
// addresses, extra steps further in, that a load at at can read. For a
// variable of at's function, only the stores that reach at count; for one
// captured from an outer function, every store there does. A store of the
// whole variable, or of a part holding the one read, counts too, and the
// part is taken from the value stored. ok is false when a variable may be
// written in a way not followed, or a part was written after the value that
// holds it.
func resolvedPartStores(addr ssa.Value, extra []int, at ssa.Instruction, depth int) ([]ssa.Value, bool) {
	if depth > resolvedMaxDepth {
		return nil, false
	}
	base, path, ok := resolvedAddrPath(addr)
	if !ok {
		return nil, false
	}
	path = append(path, extra...)
	roots, ok := resolvedRoots(base, depth+1)
	if !ok {
		return nil, false
	}

	var values []ssa.Value
	for _, root := range roots {
		writes, ok := resolvedPartWrites(root)
		if !ok {
			return nil, false
		}
		var found []*ssa.Store
		partial := false
		take := func(st *ssa.Store) bool {
			q, ok := writes[st]
			if !ok {
				return false
			}
			covers, part := resolvedPathCovers(q, path)
			partial = partial || part
			if covers {
				found = append(found, st)
			}
			return covers
		}
		if at != nil && at.Parent() == root.Parent() {
			resolvedScanBack(at, root, func(instr ssa.Instruction) bool {
				st, ok := instr.(*ssa.Store)
				return ok && take(st)
			})
		} else if !resolvedScanFromClosure(at, root, func(instr ssa.Instruction) bool {
			st, ok := instr.(*ssa.Store)
			return ok && take(st)
		}) {
			for st := range writes {
				take(st)
			}
		}
		if partial {
			return nil, false
		}
		for _, st := range found {
			parts, ok := resolvedPart(st.Val, path[len(writes[st]):], depth+1)
			if !ok {
				return nil, false
			}
			values = append(values, parts...)
		}
	}
	return values, true
}

// resolvedPart gives the values the part of v at path can be: v itself for
// an empty path.
func resolvedPart(v ssa.Value, path []int, depth int) ([]ssa.Value, bool) {
	if len(path) == 0 {
		return []ssa.Value{v}, true
	}
	if depth > resolvedMaxDepth {
		return nil, false
	}
	switch x := v.(type) {
	case *ssa.UnOp:
		if x.Op == token.MUL {
			return resolvedPartStores(x.X, path, x, depth+1)
		}
	case *ssa.Phi:
		var values []ssa.Value
		for _, e := range x.Edges {
			parts, ok := resolvedPart(e, path, depth+1)
			if !ok {
				return nil, false
			}
			values = append(values, parts...)
		}
		return values, true
	case *ssa.Field:
		return resolvedPart(x.X, append([]int{x.Field}, path...), depth+1)
	case *ssa.Index:
		if i, ok := resolvedConstIndex(x.Index); ok {
			return resolvedPart(x.X, append([]int{i}, path...), depth+1)
		}
	}
	return nil, false
}

// resolvedAddrPath gives the address addr is a part of, and the path to the
// part: a field index, or a constant index, for each step in. ok is false for
// an index that is not constant.
func resolvedAddrPath(addr ssa.Value) (base ssa.Value, path []int, ok bool) {
	for {
		switch a := addr.(type) {
		case *ssa.FieldAddr:
			path = append(path, a.Field)
			addr = a.X
		case *ssa.IndexAddr:
			i, ok := resolvedConstIndex(a.Index)
			if !ok {
				return nil, nil, false
			}
			path = append(path, i)
			addr = a.X
		default:
			slices.Reverse(path)
			return addr, path, true
		}
	}
}

// resolvedConstIndex gives the value of an index that is a constant.
func resolvedConstIndex(v ssa.Value) (int, bool) {
	c, ok := v.(*ssa.Const)
	if !ok || c.Value == nil || c.Value.Kind() != constant.Int {
		return 0, false
	}
	i, exact := constant.Int64Val(c.Value)
	return int(i), exact
}

// resolvedRoots gives the variables base can address the start of: a
// variable, or the array a slice of it from its start covers. ok is false
// for any other base.
func resolvedRoots(base ssa.Value, depth int) ([]*ssa.Alloc, bool) {
	values, ok := resolvedValues(base, depth)
	if !ok || len(values) == 0 {
		return nil, false
	}
	var roots []*ssa.Alloc
	for _, v := range values {
		switch x := v.(type) {
		case *ssa.Alloc:
			roots = append(roots, x)
		case *ssa.Slice:
			a, ok := x.X.(*ssa.Alloc)
			if !ok || x.Low != nil {
				return nil, false
			}
			roots = append(roots, a)
		case *ssa.FreeVar:
			binding, ok := resolvedBinding(x)
			if !ok {
				return nil, false
			}
			bound, ok := resolvedRoots(binding, depth+1)
			if !ok {
				return nil, false
			}
			roots = append(roots, bound...)
		default:
			return nil, false
		}
	}
	return roots, true
}

// resolvedPartWrites gives each store into root, or into a part of it, with
// the path to what it writes. ok is false when root may be written in a way
// not followed: through an address that is passed on, at an index that is
// not constant, or in a closure.
func resolvedPartWrites(root *ssa.Alloc) (map[*ssa.Store][]int, bool) {
	writes := make(map[*ssa.Store][]int)
	var walk func(v ssa.Value, path []int, inClosure bool) bool
	walk = func(v ssa.Value, path []int, inClosure bool) bool {
		for _, ref := range *v.Referrers() {
			switch r := ref.(type) {
			case *ssa.Store:
				if r.Addr != v || inClosure {
					return false
				}
				writes[r] = path
			case *ssa.UnOp:
				if r.Op != token.MUL {
					return false
				}
			case *ssa.FieldAddr:
				if !walk(r, append(slices.Clone(path), r.Field), inClosure) {
					return false
				}
			case *ssa.IndexAddr:
				i, ok := resolvedConstIndex(r.Index)
				if !ok || !walk(r, append(slices.Clone(path), i), inClosure) {
					return false
				}
			case *ssa.Slice:
				if r.Low != nil || !walk(r, path, inClosure) {
					return false
				}
			case *ssa.MakeClosure:
				fn, ok := r.Fn.(*ssa.Function)
				if !ok {
					return false
				}
				for i, b := range r.Bindings {
					if b == v && !walk(fn.FreeVars[i], path, true) {
						return false
					}
				}
			case *ssa.DebugRef:
			default:
				return false
			}
		}
		return true
	}
	return writes, walk(root, nil, false)
}

// resolvedPathCovers tells how a write to path q bears on a read of path p.
// It covers the read when q is p or holds it, and is partial when it writes
// a part of p.
func resolvedPathCovers(q, p []int) (covers, partial bool) {
	n := min(len(q), len(p))
	if !slices.Equal(q[:n], p[:n]) {
		return false, false
	}
	return len(q) <= len(p), len(q) > len(p)
}

// resolvedLookups gives the values a lookup of a constant key can read from
// a map made in its function: the values the updates of that key that reach
// it store. ok is false for any other lookup, and for a map that is used in
// any way but a lookup or an update at a constant key, as when it is passed
// on or captured.
func resolvedLookups(l *ssa.Lookup, depth int) ([]ssa.Value, bool) {
	key, ok := l.Index.(*ssa.Const)
	if l.CommaOk || !ok || key.Value == nil || depth > resolvedMaxDepth {
		return nil, false
	}
	if _, isMap := l.X.Type().Underlying().(*types.Map); !isMap {
		return nil, false
	}
	maps, ok := resolvedValues(l.X, depth+1)
	if !ok || len(maps) == 0 {
		return nil, false
	}

	var values []ssa.Value
	for _, v := range maps {
		m, ok := v.(*ssa.MakeMap)
		if !ok || m.Parent() != l.Parent() || !resolvedMapOnlyUpdated(m) {
			return nil, false
		}
		take := func(instr ssa.Instruction) bool {
			u, ok := instr.(*ssa.MapUpdate)
			if !ok || u.Map != m {
				return false
			}
			k, _ := u.Key.(*ssa.Const)
			if k == nil || k.Value == nil || !constant.Compare(k.Value, token.EQL, key.Value) {
				return false
			}
			values = append(values, u.Value)
			return true
		}
		resolvedScanBack(l, m, take)
	}
	return values, true
}

// resolvedMapOnlyUpdated reports whether the map m is only looked up, and
// updated at constant keys.
func resolvedMapOnlyUpdated(m *ssa.MakeMap) bool {
	for _, ref := range *m.Referrers() {
		switch r := ref.(type) {
		case *ssa.MapUpdate:
			if _, isConst := r.Key.(*ssa.Const); r.Map != m || !isConst {
				return false
			}
		case *ssa.Lookup:
			if r.X != m {
				return false
			}
		case *ssa.DebugRef:
		default:
			return false
		}
	}
	return true
}
