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

// DeriverTraceResult represents the result of deriver function detection.
type DeriverTraceResult struct {
	FoundAtStart     bool
	FoundOnlyInDefer bool
}

// ClosureCallsDeriver checks if a closure calls any of the required deriver functions.
func (t *Tracer) ClosureCallsDeriver(closure *ssa.Function, matcher *deriver.Matcher) DeriverTraceResult {
	if closure == nil || matcher == nil || matcher.IsEmpty() {
		return DeriverTraceResult{FoundAtStart: true}
	}

	calls := t.collectDeriverCalls(closure, false, make(map[*ssa.Function]bool))

	// Check if any OR group is satisfied at start
	for _, andGroup := range matcher.OrGroups {
		if t.checkAndGroup(calls, andGroup, false) {
			return DeriverTraceResult{FoundAtStart: true}
		}
	}

	// Check if deriver is only in defer
	for _, andGroup := range matcher.OrGroups {
		if t.checkAndGroup(calls, andGroup, true) {
			return DeriverTraceResult{FoundOnlyInDefer: true}
		}
	}

	return DeriverTraceResult{}
}

type tracedDeriverCall struct {
	fn      *types.Func
	inDefer bool
}

func (t *Tracer) collectDeriverCalls(fn *ssa.Function, inDefer bool, visited map[*ssa.Function]bool) []tracedDeriverCall {
	if fn == nil || visited[fn] {
		return nil
	}
	visited[fn] = true

	var calls []tracedDeriverCall

	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			switch v := instr.(type) {
			case *ssa.Call:
				if calledFn := ExtractCalledFunc(&v.Call); calledFn != nil {
					calls = append(calls, tracedDeriverCall{fn: calledFn, inDefer: inDefer})
				}
				if iifeFn := ExtractIIFECallee(&v.Call); iifeFn != nil {
					calls = append(calls, t.collectDeriverCalls(iifeFn, inDefer, visited)...)
				}

			case *ssa.Defer:
				if calledFn := ExtractCalledFunc(&v.Call); calledFn != nil {
					calls = append(calls, tracedDeriverCall{fn: calledFn, inDefer: true})
				}
				if iifeFn := ExtractIIFECallee(&v.Call); iifeFn != nil {
					calls = append(calls, t.collectDeriverCalls(iifeFn, true, visited)...)
				}
			}
		}
	}

	return calls
}

func (t *Tracer) checkAndGroup(calls []tracedDeriverCall, andGroup []funcspec.Spec, includeDefer bool) bool {
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
