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
// and sends count as those of fn (#282).
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
	// waits counts the waits of [waiterMethods].
	waits int
	// sends counts the sends outside a select.
	sends int
}

// fits reports whether fn only waits and signals, as [Flow.IsWaiter] tells,
// counting its waits and sends, and those of the function literals it runs
// in place, into w.
func (w *waiterShape) fits(fn *ssa.Function) bool {
	if fn == nil || fn.Blocks == nil || waiterHasLoop(fn) {
		return false
	}
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			switch x := instr.(type) {
			case *ssa.Call:
				switch {
				case waiterIsWait(x.Common()):
					w.waits++
				case waiterIsClose(x.Common()):
				case !w.fits(runCallee(x.Common())):
					return false
				}
			case *ssa.Defer:
				if !waiterIsClose(x.Common()) && !w.fits(runCallee(x.Common())) {
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
// buffer holds. So once the caller has stopped receiving, after one receive
// or on ctx.Done(), no send can block.
func waiterBuffered(f *Flow, ch ssa.Value) bool {
	read := ch
	if load, ok := ch.(*ssa.UnOp); ok && load.Op == token.MUL {
		if _, isFree := load.X.(*ssa.FreeVar); isFree {
			read = load.X
		}
	}
	stored, complete := f.CapturedSources(read)
	if !complete || len(stored) == 0 {
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
			sends, ok := waiterChanSends(mc, false)
			return !ok || int64(sends) > n
		})
	})
}

// waiterChanSends counts the sends on the channel v holds, or, when addr is
// true, the channel in the variable v points to, in the function of v and
// in each closure it is given to. A send in a select counts too, as it fills
// the buffer all the same. ok is false when the channel may be used where
// the sends cannot be counted: given to a call, stored anywhere but to its
// own variable, or sent from or given to a closure that may run more than
// once, as one in a loop is. Closures joined by a Phi and run where it is
// run are alternatives: only one of them runs, so the most sends of one of
// them count, not their sum (#298).
func waiterChanSends(v ssa.Value, addr bool) (sends int, ok bool) {
	// alternatives holds the most sends of the closures run at each site.
	alternatives := make(map[ssa.Instruction]int)
	for _, instr := range *v.Referrers() {
		if mk, isClosure := instr.(*ssa.MakeClosure); isClosure {
			n, site, ok := waiterClosureSends(mk, v, addr)
			if !ok {
				return 0, false
			}
			alternatives[site] = max(alternatives[site], n)
			continue
		}
		n, ok := waiterChanUseSends(instr, v, addr)
		if !ok {
			return 0, false
		}
		sends += n
	}
	for _, n := range alternatives {
		sends += n
	}
	return sends, true
}

// waiterChanUseSends counts the sends instr makes on the channel of v, as
// [waiterChanSends] does. When addr is true, v points to the variable of the
// channel, and only a store to it, a load from it, and a closure that
// captures it are followed.
func waiterChanUseSends(instr ssa.Instruction, v ssa.Value, addr bool) (sends int, ok bool) {
	switch x := instr.(type) {
	case *ssa.Store:
		if addr {
			// The variable is set. What it is set to is followed on its own.
			return 0, x.Addr == v
		}
		if _, local := x.Addr.(*ssa.Alloc); local {
			return waiterChanSends(x.Addr, true)
		}
	case *ssa.UnOp:
		if addr {
			return waiterChanSends(x, false)
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
func waiterClosureSends(mk *ssa.MakeClosure, v ssa.Value, addr bool) (sends int, site ssa.Instruction, ok bool) {
	fn, isFunc := mk.Fn.(*ssa.Function)
	i := slices.Index(mk.Bindings, v)
	if !isFunc || i < 0 || i >= len(fn.FreeVars) || waiterInLoop(mk.Block()) {
		return 0, nil, false
	}
	site, ok = waiterRunSite(mk)
	if !ok {
		return 0, nil, false
	}
	sends, ok = waiterChanSends(fn.FreeVars[i], addr)
	return sends, site, ok
}

// waiterRunSite gives the one instruction that uses the closure mk makes,
// outside a loop, which must run it once, as [waiterRunsOnce] tells. A Phi
// that joins the closure with other values, outside a loop, is followed to
// its one use in turn, as h := f; if c { h = g }; go h() runs f or g once
// (#298). A closure that is not used is its own site, as nothing runs it.
func waiterRunSite(mk *ssa.MakeClosure) (site ssa.Instruction, ok bool) {
	var v ssa.Value = mk
	for {
		var refs []ssa.Instruction
		for _, ref := range *v.Referrers() {
			// A Phi that joins v on several edges refers to it once each.
			if !slices.Contains(refs, ref) {
				refs = append(refs, ref)
			}
		}
		switch {
		case len(refs) == 0 && v == mk:
			return mk, true
		case len(refs) != 1 || waiterInLoop(refs[0].Block()):
			return nil, false
		}
		phi, isPhi := refs[0].(*ssa.Phi)
		if !isPhi {
			return refs[0], waiterRunsOnce(refs[0], v)
		}
		v = phi
	}
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
