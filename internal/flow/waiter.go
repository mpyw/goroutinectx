package flow

import (
	"go/constant"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/funcspec"
)

// waiterMethods are the waits a waiter may make. None of them takes a
// context, so a goroutine that makes one has no use for ctx: what it waits
// for ends by itself, as the goroutines of a WaitGroup or the process of a
// Cmd do, and those are checked on their own.
var waiterMethods = []funcspec.Spec{
	{PkgPath: "sync", TypeName: "WaitGroup", FuncName: "Wait"},
	{PkgPath: "golang.org/x/sync/errgroup", TypeName: "Group", FuncName: "Wait"},
	{PkgPath: "os/exec", TypeName: "Cmd", FuncName: "Wait"},
	{PkgPath: "os", TypeName: "Process", FuncName: "Wait"},
	{PkgPath: "github.com/sourcegraph/conc", TypeName: "WaitGroup", FuncName: "Wait"},
	{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "Pool", FuncName: "Wait"},
	{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ErrorPool", FuncName: "Wait"},
	{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ContextPool", FuncName: "Wait"},
	{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultPool", FuncName: "Wait"},
	{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultErrorPool", FuncName: "Wait"},
	{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultContextPool", FuncName: "Wait"},
	{PkgPath: "github.com/sourcegraph/conc/stream", TypeName: "Stream", FuncName: "Wait"},
}

// IsWaiter reports whether fn only waits and signals (#230). It makes at
// least one wait of [waiterMethods], and otherwise only signals what it
// waited for: it closes a channel, sends on a channel that cannot block it,
// stores to a variable, or returns. It has no loop. A send cannot block when
// it is a case of a select with a default, or when it is the one send of fn
// outside a select and the channel has room for it: see [waiterBuffered].
// A function literal fn calls or defers in place is part of fn, as
// [Runs] goes into it, so its body must have the same shape, and its waits
// and sends count as those of fn (#282). So is the one closure a variable fn
// captures can hold, when fn calls it, as go func() { run() }() calls run
// (#321).
//
// A receive from a channel is not a wait here. A goroutine that receives can
// select on ctx.Done() instead, and it leaks when nothing is ever sent. A
// send on an unbuffered channel leaks the same way when the receiver has
// stopped waiting, as one that selects on ctx.Done() does.
func (f *Flow) IsWaiter(fn *ssa.Function) bool {
	defer f.ask()()
	w := &waiterShape{f: f}
	return w.fits(fn) && w.waits > 0
}

// waiterShape is what [Flow.IsWaiter] has found in a function and the
// function literals it runs in place.
type waiterShape struct {
	f *Flow
	// inside are the functions being fitted, up the stack: a closure that
	// calls itself through a variable it captures does not fit.
	inside map[*ssa.Function]bool
	// waits counts the waits of [waiterMethods].
	waits int
	// sends counts the sends outside a select.
	sends int
}

// fits reports whether fn only waits and signals, as [Flow.IsWaiter] tells,
// counting its waits and sends, and those of the function literals it runs
// in place, into w.
func (w *waiterShape) fits(fn *ssa.Function) bool {
	if fn == nil || fn.Blocks == nil || w.inside[fn] || waiterHasLoop(fn) {
		return false
	}
	if w.inside == nil {
		w.inside = make(map[*ssa.Function]bool)
	}
	w.inside[fn] = true
	defer delete(w.inside, fn)
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			switch x := instr.(type) {
			case *ssa.Call:
				switch {
				case waiterIsWait(x.Common()):
					w.waits++
				case waiterIsClose(x.Common()):
				case !w.fitsOneOf(waiterCallees(w.f, x.Common())):
					return false
				}
			case *ssa.Defer:
				if !waiterIsClose(x.Common()) && !w.fitsOneOf(waiterCallees(w.f, x.Common())) {
					return false
				}
			case *ssa.MakeClosure:
				if !waiterRunInPlace(x) {
					return false
				}
			case *ssa.Send:
				// A second send may find the slot the first one filled.
				if w.sends++; w.sends > 1 || !waiterBuffered(w.f, x.Chan) {
					return false
				}
			case *ssa.Select:
				if x.Blocking || slices.ContainsFunc(x.States, func(s *ssa.SelectState) bool { return s.Dir != types.SendOnly }) {
					return false
				}
			case *ssa.UnOp:
				if x.Op == token.ARROW {
					return false
				}
			case *ssa.Store, *ssa.Return, *ssa.RunDefers, *ssa.Jump, *ssa.If, *ssa.Phi, *ssa.DebugRef,
				*ssa.BinOp, *ssa.FieldAddr, *ssa.Field, *ssa.Alloc, *ssa.Extract,
				*ssa.MakeInterface, *ssa.ChangeType, *ssa.ChangeInterface, *ssa.Convert:
			default:
				return false
			}
		}
	}
	return true
}

// waiterCallees gives the function call runs in place, as [runCallee]
// gives it, or, when call calls the value loaded from a variable the
// function captures, each closure the variable can hold (#321). Whether
// such a closure runs once is left to [waiterRunSite], where it counts.
// None is given when one of the values cannot be followed to a closure.
func waiterCallees(f *Flow, call *ssa.CallCommon) []*ssa.Function {
	if fn := runCallee(call); fn != nil {
		return []*ssa.Function{fn}
	}
	load, ok := call.Value.(*ssa.UnOp)
	if !ok || load.Op != token.MUL {
		return nil
	}
	if _, isFree := load.X.(*ssa.FreeVar); !isFree {
		return nil
	}
	stored, complete := f.CapturedSources(load.X)
	if !complete {
		return nil
	}
	var fns []*ssa.Function
	for _, st := range stored {
		sources, complete := f.Sources(st.Value)
		for _, src := range sources {
			fn := src.Func()
			if !complete || src.Kind != SourceLiteral || fn == nil {
				return nil
			}
			if !slices.Contains(fns, fn) {
				fns = append(fns, fn)
			}
		}
	}
	return fns
}

// fitsOneOf reports whether each of fns fits, as [waiterShape.fits] tells,
// when a call runs one of them. The fewest waits of one count, and the
// most sends.
func (w *waiterShape) fitsOneOf(fns []*ssa.Function) bool {
	if len(fns) == 1 {
		return w.fits(fns[0])
	}
	waits, sends := 0, 0
	for i, fn := range fns {
		one := &waiterShape{f: w.f, inside: w.inside}
		if !one.fits(fn) {
			return false
		}
		if i == 0 || one.waits < waits {
			waits = one.waits
		}
		sends = max(sends, one.sends)
	}
	w.waits += waits
	w.sends += sends
	return len(fns) > 0 && w.sends <= 1
}

// waiterRunInPlace reports whether the closure mk makes is only called or
// deferred in place, so that [waiterShape.fits] goes into it.
func waiterRunInPlace(mk *ssa.MakeClosure) bool {
	refs := *mk.Referrers()
	if len(refs) != 1 {
		return false
	}
	call, ok := refs[0].(ssa.CallInstruction)
	if _, isGo := call.(*ssa.Go); !ok || isGo {
		return false
	}
	return call.Common().Value == mk && !slices.Contains(call.Common().Args, ssa.Value(mk))
}

// waiterHasLoop reports whether fn has a loop: an edge to a block that
// dominates the block it leaves.
func waiterHasLoop(fn *ssa.Function) bool {
	return slices.ContainsFunc(fn.Blocks, func(b *ssa.BasicBlock) bool {
		return slices.ContainsFunc(b.Succs, func(s *ssa.BasicBlock) bool { return s.Dominates(b) })
	})
}

// waiterIsWait reports whether call is a wait of [waiterMethods].
func waiterIsWait(call *ssa.CallCommon) bool {
	if call.IsInvoke() {
		return false
	}
	fn := ExtractCalledFunc(call)
	return fn != nil && slices.ContainsFunc(waiterMethods, func(s funcspec.Spec) bool { return s.Matches(fn) })
}

// waiterIsClose reports whether call is of the builtin close.
func waiterIsClose(call *ssa.CallCommon) bool {
	b, ok := call.Value.(*ssa.Builtin)
	return ok && b.Name() == "close"
}

// waiterBuffered reports whether every channel ch can be, read from a
// variable the closure captures or made in it, has room for every send on
// it (#242). Each is made with a constant buffer of at least one, and used
// only where [waiterChanSends] follows it, with no more sends than its
// buffer holds, and no path leaves the variable it is read from unset, as
// [resolvedMayBeUnset] tells. So once the caller has stopped receiving,
// after one receive or on ctx.Done(), no send can block.
func waiterBuffered(f *Flow, ch ssa.Value) bool {
	read := ch
	if load, ok := ch.(*ssa.UnOp); ok && load.Op == token.MUL {
		if _, isFree := load.X.(*ssa.FreeVar); isFree {
			read = load.X
		}
	}
	stored, complete := f.CapturedSources(read)
	// A variable unset on some path holds a nil channel, on which a send
	// blocks for ever, as var ch chan T; if c { ch = make(chan T, 1) } has
	// (#364).
	if !complete || len(stored) == 0 || resolvedMayBeUnset(f, ch) {
		return false
	}
	return !slices.ContainsFunc(stored, func(s Source) bool {
		sources, complete := f.Sources(s.Value)
		return !complete || len(sources) == 0 || slices.ContainsFunc(sources, func(src Source) bool {
			mc, ok := src.Value.(*ssa.MakeChan)
			if !ok {
				return true
			}
			size, ok := mc.Size.(*ssa.Const)
			if !ok || size.Value == nil {
				return true
			}
			n, exact := constant.Int64Val(constant.ToInt(size.Value))
			if !exact || n < 1 {
				return true
			}
			return int64(waiterCountedSends(f, mc, int(min(n, waiterSendsLimit)))) > n
		})
	})
}

// waiterSendsLimit bounds the buffer a send count is asked against. A
// count stops once it passes the buffer, so a larger one changes nothing
// but the work.
const waiterSendsLimit = 1 << 20

// waiterCountedSends counts the sends on the channel mc makes, as
// [waiterChanSends] does, up to more than limit, the size of its buffer,
// and remembers the count for the pass: every sender on the channel asks
// it (#366). It is more than limit, too, when they cannot be counted.
func waiterCountedSends(f *Flow, mc *ssa.MakeChan, limit int) int {
	if n, done := f.chanSends[mc]; done {
		return n
	}
	sends, ok := waiterChanSends(mc, false, limit)
	if !ok {
		sends = limit + 1
	}
	if f.chanSends == nil {
		f.chanSends = make(map[*ssa.MakeChan]int)
	}
	f.chanSends[mc] = sends
	return sends
}

// waiterChanSends counts the sends on the channel v holds, or, when addr is
// true, the channel in the variable v points to, in the function of v and
// in each closure it is given to. A send in a select counts too, as it fills
// the buffer all the same. ok is false when the channel may be used where
// the sends cannot be counted: given to a call, stored anywhere but to its
// own variable, or sent from or given to a closure that may run more than
// once, as one in a loop is. Closures joined by a Phi and run where it is
// run are alternatives: only one of them runs, so the most sends of one of
// them count, not their sum (#298). So are closures run at sites that
// exclude each other, as [waiterMostSends] counts them (#350). The count
// stops once it is more than limit, as no more sends than that can fit.
func waiterChanSends(v ssa.Value, addr bool, limit int) (sends int, ok bool) {
	// alternatives holds the most sends of the closures run at each site.
	alternatives := make(map[ssa.Instruction]int)
	for _, instr := range *v.Referrers() {
		if mk, isClosure := instr.(*ssa.MakeClosure); isClosure {
			n, site, ok := waiterClosureSends(mk, v, addr, limit)
			if !ok {
				return 0, false
			}
			alternatives[site] = max(alternatives[site], n)
			continue
		}
		n, ok := waiterChanUseSends(instr, v, addr, limit)
		if !ok {
			return 0, false
		}
		// Every count below is a part of the whole, so one more than limit
		// makes the whole more.
		if sends += n; sends > limit {
			return sends, true
		}
	}
	return sends + waiterMostSends(alternatives), true
}

// waiterMostSends gives the most sends the closures run at the sites of
// alternatives can make in one run of the functions holding the sites. Each
// site runs at most once, being outside a loop, and only when its block is
// reached. So the sites that run in one function lie on one path, each
// reaching the next, as [waiterReaches] tells, and the most sends of such a
// chain count, as if c { go f() } else { go g() } runs f or g (#350). The
// counts of the functions are added up. Which block reaches which is found
// once for all the sites (#366).
func waiterMostSends(alternatives map[ssa.Instruction]int) int {
	r := waiterReach{}
	// most holds the most sends of a chain from each site.
	most := make(map[ssa.Instruction]int, len(alternatives))
	var chain func(site ssa.Instruction) int
	chain = func(site ssa.Instruction) int {
		if n, done := most[site]; done {
			return n
		}
		next := 0
		for other := range alternatives {
			if other != site && r.reaches(site, other) {
				next = max(next, chain(other))
			}
		}
		most[site] = alternatives[site] + next
		return most[site]
	}
	// first holds the most sends of a chain in each function.
	first := make(map[*ssa.Function]int)
	for site := range alternatives {
		fn := site.Parent()
		first[fn] = max(first[fn], chain(site))
	}
	sends := 0
	for _, n := range first {
		sends += n
	}
	return sends
}

// waiterReaches reports whether b may run after a in one run of their
// function: b comes later in the block of a, or its block can be reached
// from that of a. Neither is in a loop, so at most one of them reaches the
// other.
func waiterReaches(a, b ssa.Instruction) bool {
	return (&waiterReach{}).reaches(a, b)
}

// waiterReach tells which instructions reach which, as [waiterReaches]
// does, remembering the blocks each block reaches and where each
// instruction is in its block, for many questions about one function.
type waiterReach struct {
	// blocks holds the blocks each block reaches, itself only when it is in
	// a loop.
	blocks map[*ssa.BasicBlock]map[*ssa.BasicBlock]bool
	// index holds where each instruction asked of is in its block.
	index map[ssa.Instruction]int
}

// reaches reports whether b may run after a, as [waiterReaches] tells.
func (r *waiterReach) reaches(a, b ssa.Instruction) bool {
	if a.Parent() != b.Parent() {
		return false
	}
	if a.Block() == b.Block() {
		return r.indexOf(a) < r.indexOf(b)
	}
	return r.reached(a.Block())[b.Block()]
}

// indexOf gives where instr is in its block.
func (r *waiterReach) indexOf(instr ssa.Instruction) int {
	if i, done := r.index[instr]; done {
		return i
	}
	if r.index == nil {
		r.index = make(map[ssa.Instruction]int)
	}
	for i, in := range instr.Block().Instrs {
		r.index[in] = i
	}
	return r.index[instr]
}

// reached gives the blocks from reaches.
func (r *waiterReach) reached(from *ssa.BasicBlock) map[*ssa.BasicBlock]bool {
	if seen, done := r.blocks[from]; done {
		return seen
	}
	seen := map[*ssa.BasicBlock]bool{}
	stack := slices.Clone(from.Succs)
	for len(stack) > 0 {
		next := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if !seen[next] {
			seen[next] = true
			stack = append(stack, next.Succs...)
		}
	}
	if r.blocks == nil {
		r.blocks = make(map[*ssa.BasicBlock]map[*ssa.BasicBlock]bool)
	}
	r.blocks[from] = seen
	return seen
}

// waiterExclusive reports whether at most one of sites runs in one run of
// the functions holding them: each two are the same site, or neither
// reaches the other, as [waiterReaches] tells (#350). Two sites in different
// functions are compared where those functions run: a site in a closure is
// lifted to the site that runs the closure, as [waiterLift] gives it, until
// both are in one function, as if c { func() { go f() }() } else
// { func() { go g() }() } runs f or g (#354). Sites that cannot be lifted to
// one function may both run. sited is as [waiterRunSiteFrom] has it.
func waiterExclusive(sites []ssa.Instruction, sited map[*ssa.MakeClosure]bool) bool {
	for i, a := range sites {
		for _, b := range sites[i+1:] {
			if !waiterApart(a, b, sited) {
				return false
			}
		}
	}
	return true
}

// waiterApart reports whether a and b never both run, as [waiterExclusive]
// tells for two of its sites.
func waiterApart(a, b ssa.Instruction, sited map[*ssa.MakeClosure]bool) bool {
	// lifted holds the sites a is lifted to, by the function holding each.
	lifted := map[*ssa.Function]ssa.Instruction{}
	for site := a; site != nil && lifted[site.Parent()] == nil; site = waiterLift(site, sited) {
		lifted[site.Parent()] = site
	}
	seen := map[*ssa.Function]bool{}
	for site := b; site != nil && !seen[site.Parent()]; site = waiterLift(site, sited) {
		seen[site.Parent()] = true
		if other := lifted[site.Parent()]; other != nil {
			// One site runs one closure, so what runs in two of them are
			// alternatives.
			return other == site || !waiterReaches(other, site) && !waiterReaches(site, other)
		}
	}
	return false
}

// waiterLift gives the site that runs the closure holding site, as
// [waiterRunSiteFrom] gives it, so that site runs at most once in each run
// of the one given. It is nil when the function of site is not a closure
// made outside a loop, or the closure has no such site.
func waiterLift(site ssa.Instruction, sited map[*ssa.MakeClosure]bool) ssa.Instruction {
	mk := waiterMakerOf(site.Parent())
	if mk == nil || waiterInLoop(mk.Block()) {
		return nil
	}
	up, ok := waiterRunSiteFrom(mk, mk, sited)
	if !ok {
		return nil
	}
	return up
}

// waiterMakerOf gives the closure of fn made in the function that encloses
// it, or nil when there is none.
func waiterMakerOf(fn *ssa.Function) *ssa.MakeClosure {
	if fn.Parent() == nil {
		return nil
	}
	for _, b := range fn.Parent().Blocks {
		for _, instr := range b.Instrs {
			if mk, ok := instr.(*ssa.MakeClosure); ok && mk.Fn == fn {
				return mk
			}
		}
	}
	return nil
}

// waiterChanUseSends counts the sends instr makes on the channel of v, as
// [waiterChanSends] does. When addr is true, v points to the variable of the
// channel, and only a store to it, a load from it, and a closure that
// captures it are followed.
func waiterChanUseSends(instr ssa.Instruction, v ssa.Value, addr bool, limit int) (sends int, ok bool) {
	switch x := instr.(type) {
	case *ssa.Store:
		if addr {
			// The variable is set. What it is set to is followed on its own.
			return 0, x.Addr == v
		}
		if _, local := x.Addr.(*ssa.Alloc); local {
			return waiterChanSends(x.Addr, true, limit)
		}
	case *ssa.UnOp:
		if addr {
			return waiterChanSends(x, false, limit)
		}
		return 0, x.Op == token.ARROW
	case *ssa.Send:
		// The channel sent on another one may be sent on anywhere.
		return 1, !addr && x.Chan == v && !waiterInLoop(x.Block())
	case *ssa.Select:
		for _, s := range x.States {
			switch {
			case s.Send == v:
				return 0, false
			case s.Chan == v && s.Dir == types.SendOnly:
				if waiterInLoop(x.Block()) {
					return 0, false
				}
				sends++
			}
		}
		return sends, true
	case *ssa.Call:
		if b, isBuiltin := x.Call.Value.(*ssa.Builtin); isBuiltin && !addr {
			switch b.Name() {
			case "close", "len", "cap":
				return 0, true
			}
		}
	}
	return 0, false
}

// waiterClosureSends counts the sends of the closure mk makes on the channel
// of v, which mk binds, as [waiterChanSends] does, and gives the site that
// runs it, as [waiterRunSite] gives it. The closure must be made outside a
// loop.
func waiterClosureSends(mk *ssa.MakeClosure, v ssa.Value, addr bool, limit int) (sends int, site ssa.Instruction, ok bool) {
	fn, isFunc := mk.Fn.(*ssa.Function)
	i := slices.Index(mk.Bindings, v)
	if !isFunc || i < 0 || i >= len(fn.FreeVars) || waiterInLoop(mk.Block()) {
		return 0, nil, false
	}
	site, ok = waiterRunSite(mk)
	if !ok {
		return 0, nil, false
	}
	sends, ok = waiterChanSends(fn.FreeVars[i], addr, limit)
	return sends, site, ok
}

// waiterRunSite gives the one instruction that uses the closure mk makes,
// outside a loop, which must run it once, as [waiterRunsOnce] tells. A Phi
// that joins the closure with other values, outside a loop, is followed to
// its one use in turn, as h := f; if c { h = g }; go h() runs f or g once
// (#298). So is the one store of it to a local variable, outside a loop, to
// the one load of the variable, as [waiterSpilledLoads] gives it, as
// run := f; go func() { run() }() runs f once (#321). When the variable is
// loaded several times at sites that exclude each other, each load is
// followed to its use in turn, and the variable is the site, as no one
// instruction runs the closure (#345, #356). Several uses that exclude each
// other are taken as [waiterAlternativeUses] takes them (#355). A closure
// that is not used is its own site, as nothing runs it. A closure that may
// run itself, through a variable that a closure it runs captures, has none.
func waiterRunSite(mk *ssa.MakeClosure) (site ssa.Instruction, ok bool) {
	return waiterRunSiteFrom(mk, mk, make(map[*ssa.MakeClosure]bool))
}

// waiterRunSiteFrom gives the site that runs the closure mk makes, as
// [waiterRunSite] does, from v, which holds it. sited holds the closures
// whose sites are being looked for, up the stack.
func waiterRunSiteFrom(mk *ssa.MakeClosure, v ssa.Value, sited map[*ssa.MakeClosure]bool) (site ssa.Instruction, ok bool) {
	if v == ssa.Value(mk) {
		if sited[mk] {
			return nil, false
		}
		sited[mk] = true
		defer delete(sited, mk)
	}
	var refs []ssa.Instruction
	for _, ref := range *v.Referrers() {
		// A Phi that joins v on several edges refers to it once each, and
		// so may stores of v to one variable, as run := f; if c { run = f }
		// makes when a closure captures run.
		if !slices.Contains(refs, ref) && !waiterStoresAgain(refs, ref) {
			refs = append(refs, ref)
		}
	}
	switch len(refs) {
	case 0:
		return mk, v == mk
	case 1:
		return waiterUseSite(mk, v, refs[0], sited)
	default:
		return waiterAlternativeUses(mk, v, refs, sited)
	}
}

// waiterUseSite gives the site that runs the closure mk makes through ref,
// a use of v, which holds it, as [waiterRunSite] tells: ref itself, or the
// site a Phi or a store to a local variable is followed to. sited is as
// [waiterRunSiteFrom] has it.
func waiterUseSite(mk *ssa.MakeClosure, v ssa.Value, ref ssa.Instruction, sited map[*ssa.MakeClosure]bool) (site ssa.Instruction, ok bool) {
	if waiterInLoop(ref.Block()) {
		return nil, false
	}
	switch x := ref.(type) {
	case *ssa.Phi:
		return waiterRunSiteFrom(mk, x, sited)
	case *ssa.Store:
		loads, ok := waiterSpilledLoads(x, v, sited)
		if !ok {
			return nil, false
		}
		if len(loads) == 1 {
			return waiterRunSiteFrom(mk, loads[0], sited)
		}
		sites := make([]ssa.Instruction, 0, len(loads))
		for _, load := range loads {
			site, ok := waiterRunSiteFrom(mk, load, sited)
			if !ok {
				return nil, false
			}
			sites = append(sites, site)
		}
		// Every closure stored to the variable runs at those sites, so the
		// variable is the site of each, and they are alternatives.
		return x.Addr.(*ssa.Alloc), waiterExclusive(sites, sited)
	default:
		return ref, waiterRunsOnce(ref, v)
	}
}

// waiterStoresAgain reports whether ref stores to the variable a store of
// refs stores to, outside a loop, as the first one does.
func waiterStoresAgain(refs []ssa.Instruction, ref ssa.Instruction) bool {
	st, ok := ref.(*ssa.Store)
	return ok && !waiterInLoop(st.Block()) && slices.ContainsFunc(refs, func(r ssa.Instruction) bool {
		first, ok := r.(*ssa.Store)
		return ok && first.Addr == st.Addr && first.Val == st.Val
	})
}

// waiterAlternativeUses gives the site of the closure mk makes when refs,
// the uses of v, which holds it, lead to several sites that exclude each
// other, as [waiterExclusive] tells, each running it once, as
// [waiterUseSite] gives them: if c { go send() } else { go send() } runs
// send once (#355). A use that is a Phi or a store is followed to its site,
// as if c { p := send; if d { p = send }; go p() } else { go send() } runs
// send once too (#361). v is the closure, a Phi or a load, as
// [waiterRunSiteFrom] follows it, and the site is its instruction, which
// comes before each of refs, so that any site that may run with one of them
// may run with it too.
func waiterAlternativeUses(mk *ssa.MakeClosure, v ssa.Value, refs []ssa.Instruction, sited map[*ssa.MakeClosure]bool) (site ssa.Instruction, ok bool) {
	sites := make([]ssa.Instruction, 0, len(refs))
	for _, ref := range refs {
		site, ok := waiterUseSite(mk, v, ref, sited)
		if !ok {
			return nil, false
		}
		sites = append(sites, site)
	}
	if !waiterExclusive(sites, sited) {
		return nil, false
	}
	return v.(ssa.Instruction), true
}

// waiterSpilledLoads gives the loads of the local variable st stores v
// to, as [spilledLoads] follows it into the closures that capture the
// variable. Other values stored to the variable are alternatives, as for a
// Phi: the load runs one of them. Each of those closures must be made
// outside a loop and run once, as [waiterRunSite] tells, so that a load
// runs once when it is outside a loop of its own function. When there are
// several loads, [waiterRunSiteFrom] follows each to its site, and those
// sites must exclude each other, as [waiterExclusive] tells, so that only
// one load runs: as
// run := func() { send() }; if c { run = func() { send() } }; go run()
// runs send once (#345), as if c { go func() { send() }() } else
// { go func() { send() }() } does (#350), and as two loads in one closure
// joined by a Phi do (#356). sited is as [waiterRunSiteFrom] has it.
func waiterSpilledLoads(st *ssa.Store, v ssa.Value, sited map[*ssa.MakeClosure]bool) ([]*ssa.UnOp, bool) {
	loads, binders, ok := spilledLoads(st, true)
	if !ok || st.Val != v || len(loads) == 0 || slices.ContainsFunc(loads, func(load *ssa.UnOp) bool { return waiterInLoop(load.Block()) }) {
		return nil, false
	}
	for _, mk := range binders {
		if waiterInLoop(mk.Block()) {
			return nil, false
		}
		if _, ok := waiterRunSiteFrom(mk, mk, sited); !ok {
			return nil, false
		}
	}
	return loads, true
}

// waiterOnceSpawns are the calls that run the func they are given once, in
// a new goroutine.
var waiterOnceSpawns = []funcspec.Spec{
	{PkgPath: "golang.org/x/sync/errgroup", TypeName: "Group", FuncName: "Go"},
	{PkgPath: "golang.org/x/sync/errgroup", TypeName: "Group", FuncName: "TryGo"},
	{PkgPath: "sync", TypeName: "WaitGroup", FuncName: "Go"},
	{PkgPath: "github.com/sourcegraph/conc", TypeName: "WaitGroup", FuncName: "Go"},
	{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "Pool", FuncName: "Go"},
	{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ErrorPool", FuncName: "Go"},
	{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ContextPool", FuncName: "Go"},
	{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultPool", FuncName: "Go"},
	{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultErrorPool", FuncName: "Go"},
	{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultContextPool", FuncName: "Go"},
	{PkgPath: "github.com/sourcegraph/conc/stream", TypeName: "Stream", FuncName: "Go"},
}

// waiterRunsOnce reports whether instr runs the closure v holds at most
// once: as the function of a go statement or of a direct or deferred call,
// or as the func given to one of [waiterOnceSpawns]. Any other call given
// the closure may run it any number of times (#255), as a channel given to a
// call may be sent on any number of times. So may any other use, as a
// conversion to an interface.
func waiterRunsOnce(instr ssa.Instruction, v ssa.Value) bool {
	var call *ssa.CallCommon
	switch x := instr.(type) {
	case *ssa.Go:
		call = &x.Call
	case *ssa.Call:
		call = &x.Call
	case *ssa.Defer:
		call = &x.Call
	default:
		return false
	}
	if !slices.Contains(call.Args, v) {
		return call.Value == v
	}
	fn := ExtractCalledFunc(call)
	return fn != nil && slices.ContainsFunc(waiterOnceSpawns, func(s funcspec.Spec) bool { return s.Matches(fn) })
}

// waiterInLoop reports whether b may run more than once in its function: it
// can be reached again from itself.
func waiterInLoop(b *ssa.BasicBlock) bool {
	seen := map[*ssa.BasicBlock]bool{}
	stack := slices.Clone(b.Succs)
	for len(stack) > 0 {
		next := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if next == b {
			return true
		}
		if !seen[next] {
			seen[next] = true
			stack = append(stack, next.Succs...)
		}
	}
	return false
}
