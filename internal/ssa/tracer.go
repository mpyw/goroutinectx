package ssa

import (
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/directive/carrier"
	"github.com/mpyw/goroutinectx/internal/funcspec"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Tracer provides SSA-based value tracing.
type Tracer struct{}

// NewTracer creates a new SSA tracer.
func NewTracer() *Tracer {
	return &Tracer{}
}

// ClosureCapturesContext checks if a closure captures any context.Context variable
// or a configured carrier type.
func (t *Tracer) ClosureCapturesContext(closure *ssa.Function, carriers []carrier.Carrier) bool {
	if closure == nil {
		return false
	}

	for _, fv := range closure.FreeVars {
		if typeutil.IsContextType(fv.Type()) || carrier.IsCarrierType(fv.Type(), carriers) {
			return true
		}
	}

	return false
}

// DeriverResult represents the result of deriver function detection.
type DeriverResult struct {
	FoundAtStart     bool
	FoundOnlyInDefer bool
}

// ClosureCallsDeriver checks if a closure calls any of the required deriver functions.
func (t *Tracer) ClosureCallsDeriver(closure *ssa.Function, matcher *deriver.Matcher) DeriverResult {
	if closure == nil || matcher == nil || matcher.IsEmpty() {
		return DeriverResult{FoundAtStart: true}
	}

	calls := t.collectDeriverCalls(closure, false, make(map[*ssa.Function]bool))

	// Check if any OR group is satisfied at start
	for _, andGroup := range matcher.OrGroups {
		if t.checkAndGroup(calls, andGroup, false) {
			return DeriverResult{FoundAtStart: true}
		}
	}

	// Check if deriver is only in defer
	for _, andGroup := range matcher.OrGroups {
		if t.checkAndGroup(calls, andGroup, true) {
			return DeriverResult{FoundOnlyInDefer: true}
		}
	}

	return DeriverResult{}
}

type deriverCall struct {
	fn      *types.Func
	inDefer bool
}

func (t *Tracer) collectDeriverCalls(fn *ssa.Function, inDefer bool, visited map[*ssa.Function]bool) []deriverCall {
	if fn == nil || visited[fn] {
		return nil
	}
	visited[fn] = true

	var calls []deriverCall

	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			switch v := instr.(type) {
			case *ssa.Call:
				if calledFn := ExtractCalledFunc(&v.Call); calledFn != nil {
					calls = append(calls, deriverCall{fn: calledFn, inDefer: inDefer})
				}
				if iifeFn := ExtractIIFE(&v.Call); iifeFn != nil {
					calls = append(calls, t.collectDeriverCalls(iifeFn, inDefer, visited)...)
				}

			case *ssa.Defer:
				if calledFn := ExtractCalledFunc(&v.Call); calledFn != nil {
					calls = append(calls, deriverCall{fn: calledFn, inDefer: true})
				}
				if iifeFn := ExtractIIFE(&v.Call); iifeFn != nil {
					calls = append(calls, t.collectDeriverCalls(iifeFn, true, visited)...)
				}
			}
		}
	}

	return calls
}

func (t *Tracer) checkAndGroup(calls []deriverCall, andGroup []funcspec.Spec, includeDefer bool) bool {
	for _, spec := range andGroup {
		found := false
		for _, call := range calls {
			if !includeDefer && call.inDefer {
				continue
			}
			if call.fn != nil && spec.Matches(call.fn) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// =============================================================================
// Helper Functions
// =============================================================================

// ExtractCalledFunc extracts the types.Func from a CallCommon.
func ExtractCalledFunc(call *ssa.CallCommon) *types.Func {
	if call.IsInvoke() {
		return call.Method
	}

	if fn := call.StaticCallee(); fn != nil {
		if obj, ok := fn.Object().(*types.Func); ok {
			return obj
		}
		if origin := fn.Origin(); origin != nil {
			if obj, ok := origin.Object().(*types.Func); ok {
				return obj
			}
		}
	}

	return nil
}

// ExtractIIFE checks if a CallCommon is an IIFE.
func ExtractIIFE(call *ssa.CallCommon) *ssa.Function {
	if call.IsInvoke() {
		return nil
	}

	if mc, ok := call.Value.(*ssa.MakeClosure); ok {
		if fn, ok := mc.Fn.(*ssa.Function); ok {
			return fn
		}
	}

	if fn, ok := call.Value.(*ssa.Function); ok {
		if fn.Parent() != nil {
			return fn
		}
	}

	return nil
}

// HasFuncArgs checks if the call has func-typed arguments starting from startIdx.
func HasFuncArgs(call *ssa.CallCommon, startIdx int) bool {
	args := call.Args
	if startIdx < 0 || startIdx >= len(args) {
		return false
	}

	for i := startIdx; i < len(args); i++ {
		underlying := args[i].Type().Underlying()
		if _, isFunc := underlying.(*types.Signature); isFunc {
			return true
		}
		if slice, ok := underlying.(*types.Slice); ok {
			if _, isFunc := slice.Elem().Underlying().(*types.Signature); isFunc {
				return true
			}
		}
	}
	return false
}
