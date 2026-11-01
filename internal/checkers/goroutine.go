package checkers

import (
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/checkername"
	"github.com/mpyw/goroutinectx/internal/ctxkind"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/env"
	"github.com/mpyw/goroutinectx/internal/flow"
)

// Goroutine checks that go statements propagate context.
type Goroutine struct{}

// Name returns the checker name for ignore directive matching.
func (*Goroutine) Name() checkername.Name {
	return checkername.Goroutine
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
	p := newSpawnedContextPolicy(e, nil)
	if g := e.SSA.GoAt(stmt); g != nil && !g.Call.IsInvoke() {
		if spawnedGivesNamedFuncOnlyFreshContext(e, &g.Call) {
			return false
		}
		p = newSpawnedGoContextPolicy(e, &g.Call)
	}
	return spawnedFuncVerdict(e, stmt.Call.Fun, goroutineSpawnedValue(e, stmt), p).Passes()
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

// newGoroutineDerive creates a new GoroutineDerive checker.
//
//declscope:shared // Defs in checkers.go builds the checker with it
func newGoroutineDerive(derivers *deriver.Matcher) *GoroutineDerive {
	return &GoroutineDerive{derivers: derivers}
}

// Name returns the checker name for ignore directive matching.
func (*GoroutineDerive) Name() checkername.Name {
	return checkername.GoroutineDerive
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

	var given []*types.Func
	if g := e.SSA.GoAt(stmt); g != nil {
		given = goroutineArgCalls(&g.Call)
	}
	p := &goroutineDerivePolicy{kinds: e.Kinds, derivers: c.derivers, given: given}
	switch spawnedFuncVerdict(e, stmt.Call.Fun, goroutineSpawnedValue(e, stmt), p) {
	case flow.Fail:
		return c.message()
	case flow.FailDeferOnly:
		return c.deferMessage()
	}
	return ""
}

func (c *GoroutineDerive) message() string {
	return "goroutine should call " + c.derivers.Original + " to derive context"
}

func (c *GoroutineDerive) deferMessage() string {
	return "goroutine calls " + c.derivers.Original + " in defer, but it should be called at goroutine start"
}

// goroutineDerivePolicy judges a closure by whether it takes a context or
// a carrier ([ctxkind.Param]), or calls the deriver at its start. One that
// calls it only in defer fails with [flow.FailDeferOnly]. given are the
// functions the go statement calls in its arguments, which count as called
// at the start. A factory passes when it takes a context or a carrier, or
// makes a closure taking one or calling the deriver.
type goroutineDerivePolicy struct {
	kinds    *ctxkind.Classifier
	derivers *deriver.Matcher
	given    []*types.Func
}

// Literal judges a closure as [goroutineDerivePolicy] tells.
func (p *goroutineDerivePolicy) Literal(fn *ssa.Function, _ flow.Site) flow.Verdict {
	if p.kinds.HasParam(fn, ctxkind.Param) {
		return flow.Pass
	}
	switch result := flow.ClosureCallsDeriver(fn, p.derivers, p.given...); {
	case result.FoundAtStart:
		return flow.Pass
	case result.FoundOnlyInDefer:
		return flow.FailDeferOnly
	}
	return flow.Fail
}

// Call leaves every call to the built-in rules.
func (*goroutineDerivePolicy) Call(*ssa.CallCommon) flow.Verdict {
	return flow.Unknown
}

// FactoryCall follows every factory a call calls.
func (*goroutineDerivePolicy) FactoryCall(*ssa.CallCommon) bool {
	return false
}

// Factory passes a factory as [goroutineDerivePolicy] tells.
func (p *goroutineDerivePolicy) Factory(fn *ssa.Function) bool {
	return p.kinds.HasParam(fn, ctxkind.Param) || slices.ContainsFunc(fn.AnonFuncs, func(anon *ssa.Function) bool {
		return p.kinds.HasParam(anon, ctxkind.Param) || flow.ClosureCallsDeriver(anon, p.derivers).FoundAtStart
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
