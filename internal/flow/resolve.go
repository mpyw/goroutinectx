package flow

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/typeutil"
)

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

// resolvedSources gives each value v can be, in a new walk of f's question.
//
//declscope:shared // flow.go gives them as the sources of v
func resolvedSources(f *Flow, v ssa.Value) ([]ssa.Value, bool) {
	return resolvedValues(v, resolvedWalk{f: f})
}

// resolvedValues gives each value v can be, through Phi nodes, type
// changes, loads of local variables and of their parts, and map lookups. ok
// is false when a load cannot be followed. What it finds is remembered for
// the pass, unless a bound cut it short.
func resolvedValues(v ssa.Value, w resolvedWalk) (values []ssa.Value, ok bool) {
	if r, found := w.f.sources[v]; found {
		return r.values, r.ok
	}
	cuts := w.f.cuts
	defer func() {
		if w.f.cuts == cuts {
			w.f.sources[v] = resolvedValuesResult{values: values, ok: ok}
		}
	}()
	ok = true
	seen := make(map[ssa.Value]bool)
	var visit func(v ssa.Value)
	visit = func(v ssa.Value) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		if w.f.steps++; w.exhausted() {
			ok = false
			return
		}
		switch x := v.(type) {
		case *ssa.Phi:
			for _, e := range x.Edges {
				visit(e)
			}
		case *ssa.ChangeType:
			visit(x.X)
		default:
			stored, isLoad, followed := resolvedLoaded(v, w)
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
func resolvedLoaded(v ssa.Value, w resolvedWalk) (values []ssa.Value, isLoad, ok bool) {
	switch x := v.(type) {
	case *ssa.UnOp:
		if x.Op != token.MUL {
			return nil, false, false
		}
		values, ok = resolvedStoresAt(x.X, x, w)
		return values, true, ok
	case *ssa.Lookup:
		values, ok = resolvedLookups(x, w)
		return values, true, ok
	}
	return nil, false, false
}

// resolvedCaptured gives the values v reads when it reads a variable a
// closure captures: for a free variable, the value the closure was made
// with, or for a variable captured by reference, every value stored to it;
// for a field, or an element at a constant index, of a value, that part of
// each value the value can be. Any other v is given as it is. ok is false
// when they cannot all be followed.
//
//declscope:shared // flow.go gives their sources as those of a captured variable
func resolvedCaptured(f *Flow, v ssa.Value) (values []ssa.Value, ok bool) {
	w := resolvedWalk{f: f}
	switch x := v.(type) {
	case *ssa.FreeVar:
		if _, isPtr := x.Type().Underlying().(*types.Pointer); isPtr {
			return resolvedStoresAt(x, nil, w)
		}
		binding, ok := resolvedBinding(x)
		if !ok {
			return nil, false
		}
		return []ssa.Value{binding}, true
	case *ssa.Field:
		return resolvedPart(x.X, []int{x.Field}, w)
	case *ssa.Index:
		i, ok := resolvedConstIndex(x.Index)
		if !ok {
			return nil, false
		}
		return resolvedPart(x.X, []int{i}, w)
	}
	return []ssa.Value{v}, true
}

// ResolvedSliceElems gives the elements of a slice made from a whole local
// array, as a slice literal or the slice of a variadic call is, or of each
// slice a function called to make it returns, as [Flow.Returns] gives them.
// ok is false for any other slice.
func (f *Flow) ResolvedSliceElems(v ssa.Value) (elems []ssa.Value, ok bool) {
	defer f.ask()()
	return resolvedSliceElemsAt(v, resolvedWalk{f: f})
}

func resolvedSliceElemsAt(v ssa.Value, w resolvedWalk) ([]ssa.Value, bool) {
	if w.exhausted() {
		return nil, false
	}
	switch x := v.(type) {
	case *ssa.Slice:
		a, ok := x.X.(*ssa.Alloc)
		if !ok || x.Low != nil || x.High != nil {
			return nil, false
		}
		array, ok := typeutil.UnwrapPointer(a.Type()).Underlying().(*types.Array)
		if !ok {
			return nil, false
		}
		elems := resolvedSliceElems(x, int(array.Len()))
		return elems, elems != nil
	case *ssa.Call:
		fn := x.Common().StaticCallee()
		if fn == nil || fn.Blocks == nil {
			return nil, false
		}
		var elems []ssa.Value
		for _, ret := range w.f.Returns(fn) {
			e, ok := resolvedSliceElemsAt(ret.Value, w.deeper())
			if !ok {
				return nil, false
			}
			elems = append(elems, e...)
		}
		return elems, len(elems) > 0
	}
	return nil, false
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

// resolvedStoresAt gives the values stored at addr that a load at the
// instruction at can read. For a local variable of at's function, only the
// stores that reach at count. For a variable captured from an outer function,
// the stores there that reach the closure's creation count, and every store
// after it, since the closure may run any time after; with no at, every
// store counts. ok is false when the variable may be written in a
// way not followed: through a closure, or through its address. A struct
// field, or an element at a constant index, is followed as a part of the
// variable it is in, w.depth steps in, and a variable whose address is
// loaded from another, as a whole part of it.
func resolvedStoresAt(addr ssa.Value, at ssa.Instruction, w resolvedWalk) (stored []ssa.Value, ok bool) {
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
				stored = resolvedAppendNew(stored, st.Val)
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
		return resolvedStoresAt(binding, at, w)

	case *ssa.FieldAddr, *ssa.IndexAddr:
		return resolvedPartStores(a, nil, at, w)

	case *ssa.UnOp:
		// An address loaded from a variable, as p of p := &v that a
		// closure captures holds (#273).
		if a.Op == token.MUL {
			return resolvedPartStores(a, nil, at, w)
		}
	}
	return nil, false
}

// resolvedBinding gives the value the closure owning fv was made with for it.
//
//declscope:shared // flow.go and closure.go follow a variable captured by reference to itself
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
			stored = resolvedAppendNew(stored, st.Val)
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
// cannot stop. When the closure runs at once (#272), as a function literal
// called in place or the body of a range-over-func loop does, only those
// resolvedScanBack visits from the call that runs it count. So it is for a
// closure in it, when each closure between it and at runs before the one it
// is in returns: at once, or deferred. It reports false when it cannot
// tell: for no at, or when the closure is not made exactly once.
func resolvedScanFromClosure(at, origin ssa.Instruction, visit func(ssa.Instruction) bool) bool {
	if at == nil {
		return false
	}
	outer := origin.Parent()
	fn := at.Parent()
	inRun := true
	for fn.Parent() != outer {
		if fn.Parent() == nil {
			return false
		}
		inRun = inRun && resolvedRunOf(resolvedMadeOnce(fn)) != nil
		fn = fn.Parent()
	}
	made := resolvedMadeOnce(fn)
	if made == nil {
		return false
	}
	if run, atOnce := resolvedRunOf(made).(*ssa.Call); inRun && atOnce {
		resolvedScanBack(run, origin, visit)
		return true
	}
	resolvedScanBack(made, origin, visit)
	resolvedScanForward(made, origin, func(instr ssa.Instruction) { visit(instr) })
	return true
}

// resolvedMadeOnce gives the one instruction of fn's parent that makes the
// closure fn, or nil when there is not exactly one.
func resolvedMadeOnce(fn *ssa.Function) *ssa.MakeClosure {
	var made *ssa.MakeClosure
	for _, block := range fn.Parent().Blocks {
		for _, instr := range block.Instrs {
			if mc, ok := instr.(*ssa.MakeClosure); ok && mc.Fn == fn {
				if made != nil {
					return nil
				}
				made = mc
			}
		}
	}
	return made
}

// resolvedRunOf gives the call or defer that runs the closure mc makes, and
// is its only use, when the closure runs before the function that makes it
// returns: a call or defer of it in place, or the iterator call of a
// range-over-func loop whose body it is. It gives nil for any other closure,
// or for no mc.
func resolvedRunOf(mc *ssa.MakeClosure) ssa.CallInstruction {
	if mc == nil || len(*mc.Referrers()) != 1 {
		return nil
	}
	var run ssa.CallInstruction
	switch r := (*mc.Referrers())[0].(type) {
	case *ssa.Call:
		run = r
	case *ssa.Defer:
		run = r
	default:
		return nil
	}
	if slices.Contains(run.Common().Args, ssa.Value(mc)) {
		if _, isCall := run.(*ssa.Call); !isCall || Classify(mc.Fn.(*ssa.Function)) != ClassifiedRangeBody {
			return nil
		}
	}
	return run
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

// resolvedMaxSteps bounds how many values, parts, loads and lookups one
// question to a [Flow] follows in all, with the questions it asks in turn.
// Depth alone does not bound the work: a variable copied from another on
// each of many branches, a few levels deep, fans out at each level.
const resolvedMaxSteps = 4096

// resolvedWalk is one path of a question: how deep it is, and the Flow
// that counts the work of the question and remembers what it found.
type resolvedWalk struct {
	f     *Flow
	depth int
}

// resolvedPartKey is a store, and the path into the value it stores.
//
//declscope:shared // flow.go's Flow remembers the parts by it
type resolvedPartKey struct {
	//declscope:private
	store *ssa.Store
	//declscope:private
	path string
}

// resolvedValuesResult is what a walk gave for a value or a part: the values
// it can be, and whether they could all be followed.
//
//declscope:shared // flow.go's Flow remembers them
type resolvedValuesResult struct {
	//declscope:private
	values []ssa.Value
	//declscope:private
	ok bool
}

// deeper is w one step further in, counting the step toward the bound.
func (w resolvedWalk) deeper() resolvedWalk {
	w.f.steps++
	w.depth++
	return w
}

// exhausted reports whether w is too deep, or the question has done too
// much work, to go on. A resolution that stops is not followed, so what it
// judges passes. What a stopped walk finds is not remembered.
func (w resolvedWalk) exhausted() bool {
	if w.depth > resolvedMaxDepth || w.f.steps > resolvedMaxSteps {
		w.f.cuts++
		return true
	}
	return false
}

// resolvedPartStores gives the values stored at the part of a variable addr
// addresses, extra steps further in, that a load at at can read. For a
// variable of at's function, only the stores that reach at count; for one
// captured from an outer function, every store there does. A store of the
// whole variable, or of a part holding the one read, counts too, and the
// part is taken from the value stored. ok is false when a variable may be
// written in a way not followed, or a part was written after the value that
// holds it.
func resolvedPartStores(addr ssa.Value, extra []int, at ssa.Instruction, w resolvedWalk) ([]ssa.Value, bool) {
	if w.exhausted() {
		return nil, false
	}
	base, path, ok := resolvedAddrPath(addr)
	if !ok {
		return nil, false
	}
	path = append(path, extra...)
	roots, ok := resolvedRoots(base, w.deeper())
	if !ok {
		return nil, false
	}

	var values []ssa.Value
	for _, root := range roots {
		writes, stores, ok := resolvedPartWrites(root, nil)
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
			for _, st := range stores {
				take(st)
			}
		}
		if partial {
			return nil, false
		}
		for _, st := range found {
			parts, ok := resolvedStoredPart(st, path[len(writes[st]):], w)
			if !ok {
				return nil, false
			}
			values = resolvedAppendNew(values, parts...)
		}
	}
	return values, true
}

// resolvedAppendNew appends to values each of more that it does not hold
// yet. Many paths often lead to the same value, and keeping each copy makes
// the lists grow with the number of paths, not of values.
func resolvedAppendNew(values []ssa.Value, more ...ssa.Value) []ssa.Value {
	for _, v := range more {
		if !slices.Contains(values, v) {
			values = append(values, v)
		}
	}
	return values
}

// resolvedStoredPart is resolvedPart of the value st stores, remembered for
// the rest of the pass, unless a bound cut it short: many loads may reach
// the same store.
func resolvedStoredPart(st *ssa.Store, path []int, w resolvedWalk) ([]ssa.Value, bool) {
	key := resolvedPartKey{store: st, path: fmt.Sprint(path)}
	if r, ok := w.f.parts[key]; ok {
		return r.values, r.ok
	}
	cuts := w.f.cuts
	values, ok := resolvedPart(st.Val, path, w.deeper())
	if w.f.cuts == cuts {
		w.f.parts[key] = resolvedValuesResult{values: values, ok: ok}
	}
	return values, ok
}

// resolvedPart gives the values the part of v at path can be: v itself for
// an empty path.
func resolvedPart(v ssa.Value, path []int, w resolvedWalk) ([]ssa.Value, bool) {
	if len(path) == 0 {
		return []ssa.Value{v}, true
	}
	if w.exhausted() {
		return nil, false
	}
	switch x := v.(type) {
	case *ssa.UnOp:
		if x.Op == token.MUL {
			return resolvedPartStores(x.X, path, x, w.deeper())
		}
	case *ssa.Phi:
		var values []ssa.Value
		for _, e := range x.Edges {
			parts, ok := resolvedPart(e, path, w.deeper())
			if !ok {
				return nil, false
			}
			values = resolvedAppendNew(values, parts...)
		}
		return values, true
	case *ssa.Field:
		return resolvedPart(x.X, append([]int{x.Field}, path...), w.deeper())
	case *ssa.Index:
		if i, ok := resolvedConstIndex(x.Index); ok {
			return resolvedPart(x.X, append([]int{i}, path...), w.deeper())
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
func resolvedRoots(base ssa.Value, w resolvedWalk) ([]*ssa.Alloc, bool) {
	values, ok := resolvedValues(base, w)
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
			bound, ok := resolvedRoots(binding, w.deeper())
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
// the path to what it writes. An address of root kept in a local variable
// that holds nothing else, as h := &T{} that a closure captures keeps it, is
// followed through the loads of that variable. ok is false when root may be
// written in a way not followed: through an address that is passed on, at
// an index that is not constant, or in a closure. given, when not nil,
// tells a call that may be given v, an address of root or of its part, or
// a slice of it, without that counting as a write not followed, as the go
// statement that gives it to the closure judged, or a call in a closure
// given it (#351), or one that has no context to write there (#357).
// stores gives the stores of writes in the order they are found, so that a
// walk over them takes the same order each time.
func resolvedPartWrites(root *ssa.Alloc, given func(call ssa.CallInstruction, v ssa.Value) bool) (writes map[*ssa.Store][]int, stores []*ssa.Store, ok bool) {
	writes = make(map[*ssa.Store][]int)
	kept := make(map[ssa.Value]ssa.Value)
	// givenOnly reports whether call is given v without that counting as
	// a write.
	givenOnly := func(call ssa.CallInstruction, v ssa.Value) bool { return given != nil && given(call, v) }
	var walk func(v ssa.Value, path []int, inClosure bool) bool
	// keep walks the loads of v, a variable that addr, the address of root
	// or of its part at path, is stored in. A variable that holds any other
	// value is not followed.
	var keep func(v, addr ssa.Value, path []int, inClosure bool) bool
	keep = func(v, addr ssa.Value, path []int, inClosure bool) bool {
		if held, ok := kept[v]; ok {
			return held == addr
		}
		kept[v] = addr
		for _, ref := range *v.Referrers() {
			switch r := ref.(type) {
			case *ssa.Store:
				if r.Addr != v || r.Val != addr {
					return false
				}
			case *ssa.UnOp:
				if r.Op != token.MUL || !walk(r, path, inClosure) {
					return false
				}
			case *ssa.MakeClosure:
				fn, ok := r.Fn.(*ssa.Function)
				if !ok {
					return false
				}
				for i, b := range r.Bindings {
					if b == v && !keep(fn.FreeVars[i], addr, path, true) {
						return false
					}
				}
			case *ssa.DebugRef:
			case ssa.CallInstruction:
				if !givenOnly(r, v) {
					return false
				}
			default:
				return false
			}
		}
		return true
	}
	walk = func(v ssa.Value, path []int, inClosure bool) bool {
		for _, ref := range *v.Referrers() {
			switch r := ref.(type) {
			case *ssa.Store:
				if r.Addr != v {
					a, ok := r.Addr.(*ssa.Alloc)
					if !ok || r.Val != v || !keep(a, v, path, inClosure) {
						return false
					}
					continue
				}
				if inClosure {
					return false
				}
				writes[r] = path
				stores = append(stores, r)
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
			case ssa.CallInstruction:
				if !givenOnly(r, v) {
					return false
				}
			default:
				return false
			}
		}
		return true
	}
	ok = walk(root, nil, false)
	return writes, stores, ok
}

// ResolvedHeldValues gives every value stored into what v holds, or into
// a part of it, however deep: the local variable v addresses, the one v is
// a load of, or the local array a slice v is of covers. held is false for
// any other v. given is as [resolvedPartWrites] has it: it tells a call
// given the variable that does not count as a write, or is nil. ok is
// false when the variable may be written in a way not followed, as
// [resolvedPartWrites] tells.
func (f *Flow) ResolvedHeldValues(v ssa.Value, given func(call ssa.CallInstruction, v ssa.Value) bool) (values []ssa.Value, held, ok bool) {
	defer f.ask()()
	var root ssa.Value
	switch x := v.(type) {
	case *ssa.Alloc:
		root = x
	case *ssa.UnOp:
		if x.Op == token.MUL {
			root = x.X
		}
	case *ssa.Slice:
		root = x.X
	}
	a, isAlloc := root.(*ssa.Alloc)
	if !isAlloc {
		return nil, false, false
	}
	_, stores, ok := resolvedPartWrites(a, given)
	if !ok {
		return nil, true, false
	}
	for _, st := range stores {
		values = resolvedAppendNew(values, st.Val)
	}
	return values, true, true
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
// it store. A map made in a function around it, that a closure captures
// (#302), is read the same, with the updates a load of a captured variable
// sees ([resolvedScanFromClosure]): those that reach the closure's run, for
// one that runs at once, or else every one that reaches its creation or
// comes after it. ok is false for any other lookup, and for a map that is
// used in any way but a lookup or an update at a constant key, as when it
// is passed on, or a closure captures it and does more than look it up.
func resolvedLookups(l *ssa.Lookup, w resolvedWalk) ([]ssa.Value, bool) {
	key, ok := l.Index.(*ssa.Const)
	if l.CommaOk || !ok || key.Value == nil || w.exhausted() {
		return nil, false
	}
	if _, isMap := l.X.Type().Underlying().(*types.Map); !isMap {
		return nil, false
	}
	maps, ok := resolvedValues(l.X, w.deeper())
	if !ok || len(maps) == 0 {
		return nil, false
	}

	var values []ssa.Value
	for _, v := range maps {
		m, _ := v.(*ssa.MakeMap)
		take := func(instr ssa.Instruction) bool {
			u, ok := instr.(*ssa.MapUpdate)
			if !ok || u.Map != m {
				return false
			}
			k, _ := u.Key.(*ssa.Const)
			if k == nil || k.Value == nil || !constant.Compare(k.Value, token.EQL, key.Value) {
				return false
			}
			values = resolvedAppendNew(values, u.Value)
			return true
		}
		if m == nil || !resolvedMapOnlyUpdated(m) || !resolvedLookupScan(l, m, take) {
			return nil, false
		}
	}
	return values, true
}

// resolvedLookupScan visits the updates of m that the lookup l can see:
// those that reach it in m's function, or, in a closure of a function around
// it, those [resolvedScanFromClosure] visits. It reports false when it cannot
// tell.
func resolvedLookupScan(l *ssa.Lookup, m *ssa.MakeMap, take func(ssa.Instruction) bool) bool {
	if m.Parent() == l.Parent() {
		resolvedScanBack(l, m, take)
		return true
	}
	return resolvedScanFromClosure(l, m, take)
}

// resolvedMapOnlyUpdated reports whether the map m is only looked up, and
// updated at constant keys. It may be stored to a variable that closures
// capture, as long as the variable is only looked up through
// ([resolvedMapVarOnlyLooked]).
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
		case *ssa.Store:
			a, isAlloc := r.Addr.(*ssa.Alloc)
			if !isAlloc || r.Val != m || !resolvedMapVarOnlyLooked(a) {
				return false
			}
		case *ssa.DebugRef:
		default:
			return false
		}
	}
	return true
}

// resolvedMapVarOnlyLooked reports whether v, a variable a map is stored to
// or a free variable that captures one, is stored to only once, as the map
// is, and every value loaded from it is only looked up, in its function or
// in a closure that captures it, out through each closure in turn (#302).
// The updates of the map are then all made to the map itself.
func resolvedMapVarOnlyLooked(v ssa.Value) bool {
	// The one store of the map, to the variable itself: a closure stores
	// to none.
	stores := 0
	_, isAlloc := v.(*ssa.Alloc)
	for _, ref := range *v.Referrers() {
		switch r := ref.(type) {
		case *ssa.Store:
			stores++
			if !isAlloc || r.Addr != v || stores > 1 {
				return false
			}
		case *ssa.UnOp:
			if r.Op != token.MUL || !resolvedMapOnlyLooked(r) {
				return false
			}
		case *ssa.MakeClosure:
			fn := r.Fn.(*ssa.Function)
			for i, b := range r.Bindings {
				if b == v && !resolvedMapVarOnlyLooked(fn.FreeVars[i]) {
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

// resolvedMapOnlyLooked reports whether the map m is only looked up.
func resolvedMapOnlyLooked(m ssa.Value) bool {
	return !slices.ContainsFunc(*m.Referrers(), func(ref ssa.Instruction) bool {
		l, ok := ref.(*ssa.Lookup)
		return !ok || l.X != m
	})
}
