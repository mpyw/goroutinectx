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
// the channel is made with a constant buffer, or when the send is a case of
// a select with a default.
//
// A receive from a channel is not a wait here. A goroutine that receives can
// select on ctx.Done() instead, and it leaks when nothing is ever sent. A
// send on an unbuffered channel leaks the same way when the receiver has
// stopped waiting, as one that selects on ctx.Done() does.
func (f *Flow) IsWaiter(fn *ssa.Function) bool {
	defer f.ask()()
	if fn == nil || fn.Blocks == nil || waiterHasLoop(fn) {
		return false
	}
	waits := 0
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			switch x := instr.(type) {
			case *ssa.Call:
				switch {
				case waiterIsWait(x.Common()):
					waits++
				case !waiterIsClose(x.Common()):
					return false
				}
			case *ssa.Defer:
				if !waiterIsClose(x.Common()) {
					return false
				}
			case *ssa.Send:
				if !waiterBuffered(f, x.Chan) {
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
	return waits > 0
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
// variable the closure captures or made in it, is made with a constant
// buffer of at least one.
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
			return !exact || n < 1
		})
	})
}
