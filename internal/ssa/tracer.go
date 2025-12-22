package ssa

import (
	"go/token"
	"go/types"
	"maps"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/directives/carrier"
	"github.com/mpyw/goroutinectx/internal/directives/deriver"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Tracer provides SSA-based value tracing for goroutinectx.
type Tracer struct{}

// NewTracer creates a new SSA tracer.
func NewTracer() *Tracer {
	return &Tracer{}
}

// =============================================================================
// Closure Context Checking
// =============================================================================

// ClosureUsesContext checks if a closure captures and uses any of the given context values.
// This handles FreeVars (captured variables) and also checks for context passed as arguments.
func (t *Tracer) ClosureUsesContext(closure *ssa.Function, ctxVars []*types.Var) bool {
	if closure == nil || len(ctxVars) == 0 {
		return false
	}

	// Check free variables (captured from enclosing scope)
	for _, fv := range closure.FreeVars {
		for _, ctx := range ctxVars {
			// Compare by name and type - FreeVar wraps the original Var
			if fv.Name() == ctx.Name() && isContextType(fv.Type()) {
				return true
			}
		}
	}

	return false
}

// ClosureCapturesContext checks if a closure captures any context.Context variable
// or a configured carrier type.
// This is simpler than ClosureUsesContext - it just checks FreeVars types.
// It includes nested closures due to SSA's FreeVars propagation.
func (t *Tracer) ClosureCapturesContext(closure *ssa.Function, carriers []carrier.Carrier) bool {
	if closure == nil {
		return false
	}

	// Check free variables (captured from enclosing scope)
	for _, fv := range closure.FreeVars {
		if typeutil.IsContextOrCarrierType(fv.Type(), carriers) {
			return true
		}
	}

	return false
}

// FindClosure attempts to find a closure from a value, tracing through assignments.
func (t *Tracer) FindClosure(v ssa.Value) *ssa.Function {
	return t.findClosureWithVisited(v, make(map[ssa.Value]bool))
}

func (t *Tracer) findClosureWithVisited(v ssa.Value, visited map[ssa.Value]bool) *ssa.Function {
	if visited[v] {
		return nil
	}
	visited[v] = true

	switch val := v.(type) {
	case *ssa.MakeClosure:
		if fn, ok := val.Fn.(*ssa.Function); ok {
			return fn
		}
	case *ssa.Function:
		// Anonymous function or named function
		return val
	case *ssa.Phi:
		// For Phi, check all edges - return first closure found
		for _, edge := range val.Edges {
			if fn := t.findClosureWithVisited(edge, visited); fn != nil {
				return fn
			}
		}
	case *ssa.Call:
		// Factory function - trace return value
		return t.traceCallReturn(val, visited)
	case *ssa.UnOp:
		if val.Op == token.MUL {
			// Dereference - check stored value
			if stored := t.findStoredValue(val.X); stored != nil {
				return t.findClosureWithVisited(stored, visited)
			}
		}
		return t.findClosureWithVisited(val.X, visited)
	case *ssa.TypeAssert:
		return t.findClosureWithVisited(val.X, visited)
	case *ssa.ChangeInterface:
		return t.findClosureWithVisited(val.X, visited)
	case *ssa.MakeInterface:
		return t.findClosureWithVisited(val.X, visited)
	case *ssa.Extract:
		return t.findClosureWithVisited(val.Tuple, visited)
	case *ssa.FieldAddr:
		if stored := t.findStoredValue(val); stored != nil {
			return t.findClosureWithVisited(stored, visited)
		}
	case *ssa.IndexAddr:
		if stored := t.findStoredValue(val); stored != nil {
			return t.findClosureWithVisited(stored, visited)
		}
	case *ssa.Lookup:
		// Map lookup - try to find the stored value
		if stored := t.findStoredValueInMap(val); stored != nil {
			return t.findClosureWithVisited(stored, visited)
		}
	}

	return nil
}

// traceCallReturn traces return statements in a called function to find closures.
func (t *Tracer) traceCallReturn(call *ssa.Call, visited map[ssa.Value]bool) *ssa.Function {
	callee := call.Call.StaticCallee()
	if callee == nil {
		return nil
	}

	// Check all return statements
	for _, block := range callee.Blocks {
		for _, instr := range block.Instrs {
			ret, ok := instr.(*ssa.Return)
			if !ok || len(ret.Results) == 0 {
				continue
			}

			if fn := t.findClosureWithVisited(ret.Results[0], visited); fn != nil {
				return fn
			}
		}
	}

	return nil
}

// =============================================================================
// Call Argument Tracing
// =============================================================================

// CallUsesContext checks if any argument to a call uses a context variable.
// This handles higher-order functions like makeWorker(ctx).
func (t *Tracer) CallUsesContext(call *ssa.Call, ctxVars []*types.Var) bool {
	for _, arg := range call.Call.Args {
		for _, ctx := range ctxVars {
			if t.valueReferencesVar(arg, ctx) {
				return true
			}
		}
	}
	return false
}

// valueReferencesVar checks if a value references the given variable.
func (t *Tracer) valueReferencesVar(v ssa.Value, target *types.Var) bool {
	switch val := v.(type) {
	case *ssa.Parameter:
		return val.Object() == target
	case *ssa.FreeVar:
		return val.Name() == target.Name() && types.Identical(val.Type(), target.Type())
	case *ssa.UnOp:
		return t.valueReferencesVar(val.X, target)
	case *ssa.MakeInterface:
		return t.valueReferencesVar(val.X, target)
	}
	return false
}

// =============================================================================
// Store Tracking (from zerologlintctx)
// =============================================================================

// findStoredValue finds the value that was stored at the given address.
func (t *Tracer) findStoredValue(addr ssa.Value) ssa.Value {
	var fn *ssa.Function
	switch v := addr.(type) {
	case *ssa.FieldAddr:
		fn = v.Parent()
	case *ssa.IndexAddr:
		fn = v.Parent()
	case *ssa.Alloc:
		fn = v.Parent()
	default:
		if instr, ok := addr.(ssa.Instruction); ok {
			fn = instr.Parent()
		}
	}
	if fn == nil {
		return nil
	}

	// Look for Store instructions that write to a matching address
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			store, ok := instr.(*ssa.Store)
			if !ok {
				continue
			}
			if t.addressesMatch(store.Addr, addr) {
				return store.Val
			}
		}
	}
	return nil
}

// findStoredValueInMap attempts to find a value stored in a map literal.
func (t *Tracer) findStoredValueInMap(lookup *ssa.Lookup) ssa.Value {
	// This is a best-effort approach for map literals
	// Full map tracking would require more sophisticated analysis
	return nil
}

// addressesMatch checks if two addresses refer to the same memory location.
func (t *Tracer) addressesMatch(a, b ssa.Value) bool {
	if a == b {
		return true
	}

	// Check for equivalent FieldAddr
	fa1, ok1 := a.(*ssa.FieldAddr)
	fa2, ok2 := b.(*ssa.FieldAddr)
	if ok1 && ok2 {
		return fa1.X == fa2.X && fa1.Field == fa2.Field
	}

	// Check for equivalent IndexAddr
	ia1, ok1 := a.(*ssa.IndexAddr)
	ia2, ok2 := b.(*ssa.IndexAddr)
	if ok1 && ok2 && ia1.X == ia2.X {
		c1, cok1 := ia1.Index.(*ssa.Const)
		c2, cok2 := ia2.Index.(*ssa.Const)
		if cok1 && cok2 {
			return c1.Value == c2.Value
		}
	}

	return false
}

// =============================================================================
// Phi Node Handling
// =============================================================================

// AllPhiEdgesSatisfy checks if all non-nil, non-cyclic edges of a Phi satisfy the predicate.
func (t *Tracer) AllPhiEdgesSatisfy(phi *ssa.Phi, pred func(ssa.Value) bool) bool {
	if len(phi.Edges) == 0 {
		return false
	}

	visited := make(map[ssa.Value]bool)
	hasValidEdge := false

	for _, edge := range phi.Edges {
		// Skip cyclic edges
		if t.edgeLeadsTo(edge, phi, maps.Clone(visited)) {
			continue
		}
		// Skip nil constants
		if t.isNilConst(edge) {
			continue
		}

		hasValidEdge = true
		if !pred(edge) {
			return false
		}
	}

	return hasValidEdge
}

func (t *Tracer) edgeLeadsTo(edge ssa.Value, target *ssa.Phi, seen map[ssa.Value]bool) bool {
	if edge == target {
		return true
	}
	if seen[edge] {
		return false
	}
	seen[edge] = true

	switch val := edge.(type) {
	case *ssa.Phi:
		for _, e := range val.Edges {
			if t.edgeLeadsTo(e, target, seen) {
				return true
			}
		}
	case *ssa.Call:
		if len(val.Call.Args) > 0 {
			return t.edgeLeadsTo(val.Call.Args[0], target, seen)
		}
	}

	return false
}

func (t *Tracer) isNilConst(v ssa.Value) bool {
	c, ok := v.(*ssa.Const)
	return ok && c.Value == nil
}

// =============================================================================
// Context Type Detection
// =============================================================================

// isContextType checks if a type is context.Context.
// Handles pointer types (*context.Context) which are common in SSA FreeVars.
func isContextType(t types.Type) bool {
	// Unwrap pointer types
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "context" && obj.Name() == "Context"
}

// GetContextVars returns all context.Context variables available in a function.
// This includes both parameters and captured variables (FreeVars) from enclosing scopes.
func GetContextVars(fn *ssa.Function) []*types.Var {
	var ctxVars []*types.Var
	if fn == nil {
		return ctxVars
	}

	// Check parameters
	sig := fn.Signature
	params := sig.Params()
	for i := 0; i < params.Len(); i++ {
		param := params.At(i)
		if isContextType(param.Type()) {
			ctxVars = append(ctxVars, param)
		}
	}

	// Check captured variables (FreeVars) from enclosing scopes
	for _, fv := range fn.FreeVars {
		if isContextType(fv.Type()) {
			// FreeVar doesn't have an underlying types.Var directly accessible,
			// but we can create a synthetic Var for matching purposes.
			// The package is derived from the parent function.
			var pkg *types.Package
			if fn.Pkg != nil {
				pkg = fn.Pkg.Pkg
			}
			ctxVars = append(ctxVars, types.NewVar(fv.Pos(), pkg, fv.Name(), fv.Type()))
		}
	}

	return ctxVars
}

// =============================================================================
// Deriver Function Detection
// =============================================================================

// DeriverResult represents the result of deriver function detection.
type DeriverResult struct {
	// FoundAtStart indicates the deriver is called at goroutine start (not in defer)
	FoundAtStart bool
	// FoundOnlyInDefer indicates the deriver is called, but only in defer statements
	FoundOnlyInDefer bool
}

// CheckDeriverCalls checks if a closure calls any of the required deriver functions.
// It traverses into immediately-invoked function expressions (IIFE) but tracks
// whether calls are made in defer statements.
func (t *Tracer) CheckDeriverCalls(closure *ssa.Function, matcher *deriver.Matcher) DeriverResult {
	if closure == nil || matcher == nil || matcher.IsEmpty() {
		return DeriverResult{FoundAtStart: true} // No check needed
	}

	// Collect all function calls with their defer status
	calls := t.collectDeriverCalls(closure, false, make(map[*ssa.Function]bool))

	// Check if any OR group is satisfied
	for _, andGroup := range matcher.OrGroups {
		startResult := t.checkAndGroup(calls, andGroup, false)
		if startResult {
			return DeriverResult{FoundAtStart: true}
		}
	}

	// Check if deriver is only in defer
	for _, andGroup := range matcher.OrGroups {
		deferResult := t.checkAndGroup(calls, andGroup, true)
		if deferResult {
			return DeriverResult{FoundOnlyInDefer: true}
		}
	}

	return DeriverResult{}
}

// deriverCall represents a function call with its context (defer or not).
type deriverCall struct {
	fn      *types.Func
	inDefer bool
}

// collectDeriverCalls collects all function calls in a closure, including IIFE.
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
				// Regular function call
				if calledFn := t.extractCalledFunc(v); calledFn != nil {
					calls = append(calls, deriverCall{fn: calledFn, inDefer: inDefer})
				}
				// Check for IIFE: call where the callee is a MakeClosure
				if iifeFn := t.extractIIFE(v); iifeFn != nil {
					// Traverse into the IIFE with the same defer status
					calls = append(calls, t.collectDeriverCalls(iifeFn, inDefer, visited)...)
				}

			case *ssa.Defer:
				// Deferred function call - mark as inDefer
				if calledFn := t.extractCalledFuncFromCallCommon(&v.Call); calledFn != nil {
					calls = append(calls, deriverCall{fn: calledFn, inDefer: true})
				}
				// Check for deferred IIFE
				if iifeFn := t.extractIIFEFromCallCommon(&v.Call); iifeFn != nil {
					// Traverse into the deferred IIFE with inDefer=true
					calls = append(calls, t.collectDeriverCalls(iifeFn, true, visited)...)
				}
			}
		}
	}

	return calls
}

// extractCalledFunc extracts the types.Func from a Call instruction.
func (t *Tracer) extractCalledFunc(call *ssa.Call) *types.Func {
	return t.extractCalledFuncFromCallCommon(&call.Call)
}

// extractCalledFuncFromCallCommon extracts the types.Func from a CallCommon.
func (t *Tracer) extractCalledFuncFromCallCommon(call *ssa.CallCommon) *types.Func {
	if call.IsInvoke() {
		// Interface method call
		return call.Method
	}

	// Static call
	if fn := call.StaticCallee(); fn != nil {
		if obj, ok := fn.Object().(*types.Func); ok {
			return obj
		}
	}

	return nil
}

// extractIIFE checks if a Call instruction is an IIFE (immediately invoked function expression).
// Returns the called function if it's an IIFE, nil otherwise.
func (t *Tracer) extractIIFE(call *ssa.Call) *ssa.Function {
	return t.extractIIFEFromCallCommon(&call.Call)
}

// extractIIFEFromCallCommon checks if a CallCommon is an IIFE.
func (t *Tracer) extractIIFEFromCallCommon(call *ssa.CallCommon) *ssa.Function {
	if call.IsInvoke() {
		return nil
	}

	// Check if the callee is a MakeClosure
	if mc, ok := call.Value.(*ssa.MakeClosure); ok {
		if fn, ok := mc.Fn.(*ssa.Function); ok {
			return fn
		}
	}

	// Check if the callee is a direct function reference
	if fn, ok := call.Value.(*ssa.Function); ok {
		// Only count as IIFE if it's an anonymous function (has no name in package scope)
		if fn.Parent() != nil {
			return fn
		}
	}

	return nil
}

// checkAndGroup checks if all specs in an AND group are satisfied.
// If includeDefer is false, only non-defer calls are considered.
// If includeDefer is true, all calls (including defer) are considered.
func (t *Tracer) checkAndGroup(calls []deriverCall, andGroup []deriver.FuncSpec, includeDefer bool) bool {
	for _, spec := range andGroup {
		found := false
		for _, call := range calls {
			if !includeDefer && call.inDefer {
				continue
			}
			if t.matchesSpec(call.fn, spec) {
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

// matchesSpec checks if a types.Func matches the given deriver function spec.
func (t *Tracer) matchesSpec(fn *types.Func, spec deriver.FuncSpec) bool {
	if fn == nil {
		return false
	}

	if fn.Name() != spec.FuncName {
		return false
	}

	if fn.Pkg() == nil || fn.Pkg().Path() != spec.PkgPath {
		return false
	}

	if spec.TypeName != "" {
		return t.matchesMethod(fn, spec.TypeName)
	}

	return true
}

// matchesMethod checks if a types.Func is a method on the expected type.
func (t *Tracer) matchesMethod(fn *types.Func, typeName string) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return false
	}

	recv := sig.Recv()
	if recv == nil {
		return false
	}

	recvType := recv.Type()
	if ptr, ok := recvType.(*types.Pointer); ok {
		recvType = ptr.Elem()
	}

	named, ok := recvType.(*types.Named)
	if !ok {
		return false
	}

	return named.Obj().Name() == typeName
}
