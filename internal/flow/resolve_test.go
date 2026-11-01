package flow

import (
	"go/constant"
	"go/types"
	"testing"

	"golang.org/x/tools/go/ssa"
)

// resolvedExhaustedWalk is a walk that has done all the work it may.
func resolvedExhaustedWalk() resolvedWalk {
	w := resolvedWalk{f: NewFlow()}
	w.f.steps = resolvedMaxSteps + 1
	return w
}

// TestResolvedWalkStopsWhenExhausted checks that every step of a resolution
// gives up, leaving the value not followed, once the walk has done all the
// work it may.
func TestResolvedWalkStopsWhenExhausted(t *testing.T) {
	t.Parallel()

	key := ssa.NewConst(constant.MakeString("a"), types.Typ[types.String])

	if _, ok := resolvedValues(key, resolvedExhaustedWalk()); ok {
		t.Error("resolvedValues followed a value past the bound")
	}
	if _, ok := resolvedPartStores(key, nil, nil, resolvedExhaustedWalk()); ok {
		t.Error("resolvedPartStores followed a part past the bound")
	}
	if _, ok := resolvedPart(key, []int{0}, resolvedExhaustedWalk()); ok {
		t.Error("resolvedPart followed a part past the bound")
	}
	if _, ok := resolvedLookups(&ssa.Lookup{Index: key}, resolvedExhaustedWalk()); ok {
		t.Error("resolvedLookups followed a lookup past the bound")
	}
	if _, ok := resolvedSliceElemsAt(key, resolvedExhaustedWalk()); ok {
		t.Error("resolvedSliceElemsAt followed a slice past the bound")
	}
}

// TestResolvedAppendNew checks that a value already held is not added again.
func TestResolvedAppendNew(t *testing.T) {
	t.Parallel()

	a := ssa.NewConst(constant.MakeInt64(1), types.Typ[types.Int])
	b := ssa.NewConst(constant.MakeInt64(2), types.Typ[types.Int])
	got := resolvedAppendNew([]ssa.Value{a}, a, b, b)
	if len(got) != 2 || got[0] != a || got[1] != b {
		t.Errorf("resolvedAppendNew = %v, want [a b]", got)
	}
}

// resolvedScanSrc has a variable x of h that the closure h returns reads,
// and a function g outside h.
const resolvedScanSrc = `package p

func h() func() int {
	x := 1
	return func() int { return x }
}

func g() int { return 2 }
`

// TestResolvedScanFromClosureCannotTell checks that the stores a closure
// sees are not scanned for when the load is not in a closure of the
// variable's function, or the closure is made more than once.
func TestResolvedScanFromClosureCannotTell(t *testing.T) {
	t.Parallel()

	pkg := buildClosurePackage(t, resolvedScanSrc)
	h := pkg.Func("h")
	var x *ssa.Alloc
	var made *ssa.MakeClosure
	for _, instr := range h.Blocks[0].Instrs {
		switch v := instr.(type) {
		case *ssa.Alloc:
			x = v
		case *ssa.MakeClosure:
			made = v
		}
	}
	if x == nil || made == nil {
		t.Fatal("no variable or closure in h")
	}
	visit := func(ssa.Instruction) bool { return false }

	outside := pkg.Func("g").Blocks[0].Instrs[0]
	if resolvedScanFromClosure(outside, x, visit) {
		t.Error("resolvedScanFromClosure scanned for a load outside the variable's function")
	}

	// A function literal is made once. Make it twice, to see that a closure
	// made more than once is not scanned for.
	block := made.Block()
	block.Instrs = append(block.Instrs[:len(block.Instrs)-1], made, block.Instrs[len(block.Instrs)-1])
	inside := h.AnonFuncs[0].Blocks[0].Instrs[0]
	if resolvedScanFromClosure(inside, x, visit) {
		t.Error("resolvedScanFromClosure scanned for a closure made twice")
	}
}
