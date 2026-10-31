package checkers

import (
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/carrier"
	"github.com/mpyw/goroutinectx/internal/checker"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/env"
	"github.com/mpyw/goroutinectx/internal/flow"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Goroutine checks that go statements propagate context.
type Goroutine struct{}

// Name returns the checker name for ignore directive matching.
func (*Goroutine) Name() checker.Name {
	return checker.Goroutine
}

// CheckGoStmt checks a go statement for context propagation.
func (c *Goroutine) CheckGoStmt(e *env.Env, ctxName string, stmt *ast.GoStmt) []analysis.Diagnostic {
	if c.propagates(e, stmt) {
		return nil
	}
	return []analysis.Diagnostic{{
		Pos:     stmt.Pos(),
		Message: "goroutine does not propagate context \"" + ctxName + "\"",
	}}
}

func (c *Goroutine) propagates(e *env.Env, stmt *ast.GoStmt) bool {
	r := spawnedContextResolver(e, nil)
	if g := e.SSA.GoAt(stmt); g != nil && !g.Call.IsInvoke() {
		if spawnedGivesNamedFuncOnlyFreshContext(e, &g.Call) {
			return false
		}
		// A context parameter counts only when the go statement gives it a
		// context other than a fresh context.Background() or TODO(), or
		// gives another value that may hold a context.
		args := g.Call.Args
		onlyFresh := flow.GivesOnlyFreshContext(&g.Call, e.Carriers)
		r.Closure = func(fn *ssa.Function) bool {
			if flow.ClosureCapturesContext(fn, e.Carriers, e.Module) {
				return true
			}
			return slices.ContainsFunc(fn.Params, func(p *ssa.Parameter) bool {
				if !typeutil.IsContextType(p.Type()) && !carrier.IsCarrierType(p.Type(), e.Carriers) {
					return false
				}
				i := slices.Index(fn.Params, p)
				return i >= len(args) || !onlyFresh || !flow.IsFreshContext(args[i])
			})
		}
	}
	return spawnedFuncPasses(e, stmt.Call.Fun, goroutineSpawnedValue(e, stmt), r)
}

// goroutineSpawnedValue gives the SSA value of the function stmt runs, or nil.
func goroutineSpawnedValue(e *env.Env, stmt *ast.GoStmt) ssa.Value {
	g := e.SSA.GoAt(stmt)
	if g == nil || g.Call.IsInvoke() {
		return nil
	}
	return g.Call.Value
}

// GoroutineDerive checks that go statements call a deriver function.
type GoroutineDerive struct {
	derivers *deriver.Matcher
}

// NewGoroutineDerive creates a new GoroutineDerive checker.
func NewGoroutineDerive(derivers *deriver.Matcher) *GoroutineDerive {
	return &GoroutineDerive{derivers: derivers}
}

// Name returns the checker name for ignore directive matching.
func (*GoroutineDerive) Name() checker.Name {
	return checker.GoroutineDerive
}

// CheckGoStmt checks a go statement for deriver function calls.
func (c *GoroutineDerive) CheckGoStmt(e *env.Env, _ string, stmt *ast.GoStmt) []analysis.Diagnostic {
	msg := c.failure(e, stmt)
	if msg == "" {
		return nil
	}
	return []analysis.Diagnostic{{Pos: stmt.Pos(), Message: msg}}
}

// failure gives the message to report for stmt, or "" when it derives.
func (c *GoroutineDerive) failure(e *env.Env, stmt *ast.GoStmt) string {
	if c.derivers.IsEmpty() {
		return ""
	}

	v := goroutineSpawnedValue(e, stmt)
	var given []*types.Func
	if g := e.SSA.GoAt(stmt); g != nil {
		given = goroutineArgCalls(&g.Call)
	}
	if spawnedFuncPasses(e, stmt.Call.Fun, v, c.resolver(e.Carriers, given, true)) {
		return ""
	}
	if spawnedFuncPasses(e, stmt.Call.Fun, v, c.resolver(e.Carriers, given, false)) {
		return c.deferMessage()
	}
	return c.message()
}

func (c *GoroutineDerive) message() string {
	return "goroutine should call " + c.derivers.Original + " to derive context"
}

func (c *GoroutineDerive) deferMessage() string {
	return "goroutine calls " + c.derivers.Original + " in defer, but it should be called at goroutine start"
}

// resolver judges a closure by whether it takes a context or one of
// carriers, or calls the deriver: at its start when atStart is set, or else
// in defer too. given are the functions the go statement calls in its
// arguments, which count as called at the start.
func (c *GoroutineDerive) resolver(carriers []carrier.Carrier, given []*types.Func, atStart bool) *flow.FuncResolver {
	return &flow.FuncResolver{
		Closure: func(fn *ssa.Function) bool {
			if flow.HasContextParam(fn, carriers) {
				return true
			}
			result := flow.ClosureCallsDeriver(fn, c.derivers, given...)
			return result.FoundAtStart || (!atStart && result.FoundOnlyInDefer)
		},
		Factory: func(fn *ssa.Function) bool { return c.factoryDerives(fn, carriers) },
	}
}

// factoryDerives passes a factory that takes a context or one of carriers,
// or that makes a closure taking one or calling the deriver.
func (c *GoroutineDerive) factoryDerives(fn *ssa.Function, carriers []carrier.Carrier) bool {
	return flow.HasContextParam(fn, carriers) || slices.ContainsFunc(fn.AnonFuncs, func(anon *ssa.Function) bool {
		return flow.HasContextParam(anon, carriers) || flow.ClosureCallsDeriver(anon, c.derivers).FoundAtStart
	})
}

// goroutineArgCalls gives the functions called to make the arguments of call, as
// txn.NewGoroutine() in go worker(txn.NewGoroutine()), however deep in them.
func goroutineArgCalls(call *ssa.CallCommon) []*types.Func {
	var fns []*types.Func
	seen := make(map[ssa.Value]bool)
	var visit func(v ssa.Value)
	visit = func(v ssa.Value) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		if c, ok := v.(*ssa.Call); ok {
			if fn := flow.ExtractCalledFunc(c.Common()); fn != nil {
				fns = append(fns, fn)
			}
		}
		instr, ok := v.(ssa.Instruction)
		if !ok {
			return
		}
		for _, op := range instr.Operands(nil) {
			if op != nil {
				visit(*op)
			}
		}
	}
	for _, arg := range call.Args {
		visit(arg)
	}
	return fns
}
