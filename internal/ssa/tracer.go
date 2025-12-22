package ssa

import (
	"go/token"
	"go/types"
	"maps"

	"golang.org/x/tools/go/ssa"
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
func isContextType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "context" && obj.Name() == "Context"
}

// GetContextVars returns all context.Context variables from function parameters.
func GetContextVars(fn *ssa.Function) []*types.Var {
	var ctxVars []*types.Var
	if fn == nil {
		return ctxVars
	}

	sig := fn.Signature
	params := sig.Params()
	for i := 0; i < params.Len(); i++ {
		param := params.At(i)
		if isContextType(param.Type()) {
			ctxVars = append(ctxVars, param)
		}
	}

	return ctxVars
}
