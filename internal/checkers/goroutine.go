package checkers

import (
	"go/ast"
	"slices"

	"golang.org/x/tools/go/analysis"
	gossa "golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/checker"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/probe"
	"github.com/mpyw/goroutinectx/internal/ssa"
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
	return spawnedFuncPasses(cctx, stmt.Call.Fun, goroutineSpawnedValue(cctx, stmt), spawnedContextResolver(cctx, nil), true)
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
	if spawnedFuncPasses(cctx, stmt.Call.Fun, v, c.resolver(true), false) {
		return ""
	}
	if spawnedFuncPasses(cctx, stmt.Call.Fun, v, c.resolver(false), false) {
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

// resolver judges a closure by whether it takes a context or calls the
// deriver: at its start when atStart is set, or else in defer too.
func (c *GoroutineDerive) resolver(atStart bool) *ssa.FuncResolver {
	return &ssa.FuncResolver{
		Closure: func(fn *gossa.Function) bool {
			if ssa.HasContextParam(fn) {
				return true
			}
			result := ssa.ClosureCallsDeriver(fn, c.derivers)
			return result.FoundAtStart || (!atStart && result.FoundOnlyInDefer)
		},
		Factory: c.factoryDerives,
	}
}

// factoryDerives passes a factory that takes a context, or that makes a
// closure taking a context or calling the deriver.
func (c *GoroutineDerive) factoryDerives(fn *gossa.Function) bool {
	return ssa.HasContextParam(fn) || slices.ContainsFunc(fn.AnonFuncs, func(anon *gossa.Function) bool {
		return ssa.HasContextParam(anon) || ssa.ClosureCallsDeriver(anon, c.derivers).FoundAtStart
	})
}
