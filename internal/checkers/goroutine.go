package checkers

import (
	"go/ast"
	"slices"

	"golang.org/x/tools/go/analysis"
	gossa "golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/carrier"
	"github.com/mpyw/goroutinectx/internal/checker"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/probe"
	"github.com/mpyw/goroutinectx/internal/ssa"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Goroutine checks that go statements propagate context.
type Goroutine struct{}

// Name returns the checker name for ignore directive matching.
func (*Goroutine) Name() checker.Name {
	return checker.Goroutine
}

// CheckGoStmt checks a go statement for context propagation.
func (c *Goroutine) CheckGoStmt(cctx *probe.Context, stmt *ast.GoStmt) []analysis.Diagnostic {
	if c.propagates(cctx, stmt) {
		return nil
	}
	return []analysis.Diagnostic{{
		Pos:     stmt.Pos(),
		Message: "goroutine does not propagate context \"" + cctx.CtxNames[0] + "\"",
	}}
}

func (c *Goroutine) propagates(cctx *probe.Context, stmt *ast.GoStmt) bool {
	r := spawnedContextResolver(cctx, nil)
	if g := cctx.SSAProg.GoAt(stmt); g != nil && !g.Call.IsInvoke() {
		// A named function is not looked into, but one given only a fresh
		// context does not get ctx. A method is left alone, as its receiver
		// may hold a context.
		if fn, ok := g.Call.Value.(*gossa.Function); ok && fn.Parent() == nil && fn.Synthetic == "" && fn.Signature.Recv() == nil &&
			ssa.GivesOnlyFreshContext(&g.Call, cctx.Carriers) {
			return false
		}
		// A context parameter counts only when the go statement gives it a
		// context other than a fresh context.Background() or TODO().
		args := g.Call.Args
		r.Closure = func(fn *gossa.Function) bool {
			if ssa.ClosureCapturesContext(fn, cctx.Carriers) {
				return true
			}
			return slices.ContainsFunc(fn.Params, func(p *gossa.Parameter) bool {
				if !typeutil.IsContextType(p.Type()) && !carrier.IsCarrierType(p.Type(), cctx.Carriers) {
					return false
				}
				i := slices.Index(fn.Params, p)
				return i >= len(args) || !ssa.IsFreshContext(args[i])
			})
		}
	}
	return spawnedFuncPasses(cctx, stmt.Call.Fun, goroutineSpawnedValue(cctx, stmt), r)
}

// goroutineSpawnedValue gives the SSA value of the function stmt runs, or nil.
func goroutineSpawnedValue(cctx *probe.Context, stmt *ast.GoStmt) gossa.Value {
	g := cctx.SSAProg.GoAt(stmt)
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
func (c *GoroutineDerive) CheckGoStmt(cctx *probe.Context, stmt *ast.GoStmt) []analysis.Diagnostic {
	msg := c.failure(cctx, stmt)
	if msg == "" {
		return nil
	}
	return []analysis.Diagnostic{{Pos: stmt.Pos(), Message: msg}}
}

// failure gives the message to report for stmt, or "" when it derives.
func (c *GoroutineDerive) failure(cctx *probe.Context, stmt *ast.GoStmt) string {
	if c.derivers.IsEmpty() {
		return ""
	}

	v := goroutineSpawnedValue(cctx, stmt)
	if spawnedFuncPasses(cctx, stmt.Call.Fun, v, c.resolver(cctx.Carriers, true)) {
		return ""
	}
	if spawnedFuncPasses(cctx, stmt.Call.Fun, v, c.resolver(cctx.Carriers, false)) {
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
// in defer too.
func (c *GoroutineDerive) resolver(carriers []carrier.Carrier, atStart bool) *ssa.FuncResolver {
	return &ssa.FuncResolver{
		Closure: func(fn *gossa.Function) bool {
			if ssa.HasContextParam(fn, carriers) {
				return true
			}
			result := ssa.ClosureCallsDeriver(fn, c.derivers)
			return result.FoundAtStart || (!atStart && result.FoundOnlyInDefer)
		},
		Factory: func(fn *gossa.Function) bool { return c.factoryDerives(fn, carriers) },
	}
}

// factoryDerives passes a factory that takes a context or one of carriers,
// or that makes a closure taking one or calling the deriver.
func (c *GoroutineDerive) factoryDerives(fn *gossa.Function, carriers []carrier.Carrier) bool {
	return ssa.HasContextParam(fn, carriers) || slices.ContainsFunc(fn.AnonFuncs, func(anon *gossa.Function) bool {
		return ssa.HasContextParam(anon, carriers) || ssa.ClosureCallsDeriver(anon, c.derivers).FoundAtStart
	})
}
