package ssa

import (
	"go/constant"
	"go/types"
	"testing"

	"golang.org/x/tools/go/ssa"
)

// resolvedExhaustedWalk is a walk that has done all the work it may.
func resolvedExhaustedWalk() resolvedWalk {
	w := newResolvedWalk()
	*w.steps = resolvedMaxSteps + 1
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
