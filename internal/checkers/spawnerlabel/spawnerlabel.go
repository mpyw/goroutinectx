package spawnerlabel

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/directive/spawner"
	"github.com/mpyw/goroutinectx/internal/registry"
	internalssa "github.com/mpyw/goroutinectx/internal/ssa"
)

// Checker validates that functions are properly labeled with //goroutinectx:spawner.
type Checker struct {
	spawners *spawner.Map
	registry *registry.Registry
	ssaProg  *internalssa.Program
}

// New creates a new spawnerlabel checker.
func New(spawners *spawner.Map, reg *registry.Registry, ssaProg *internalssa.Program) *Checker {
	return &Checker{spawners: spawners, registry: reg, ssaProg: ssaProg}
}

// spawnCallInfo contains information about a detected spawn call.
type spawnCallInfo struct {
	methodName string // e.g., "errgroup.Group.Go", "sync.WaitGroup.Go", "gotask.DoAll"
}

// hasFuncParams checks if a function has func-typed parameters.
func hasFuncParams(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return false
	}

	params := sig.Params()
	for param := range params.Variables() {
		paramType := param.Type()

		// Handle variadic parameters: ...func() is represented as []func()
		if slice, ok := paramType.(*types.Slice); ok {
			paramType = slice.Elem()
		}

		if _, isFunc := paramType.Underlying().(*types.Signature); isFunc {
			return true
		}
	}

	return false
}

// Check runs the spawnerlabel analysis on the given pass, and gives its
// reports.
func (c *Checker) Check(pass *analysis.Pass, skipFiles map[string]bool) []analysis.Diagnostic {
	var diags []analysis.Diagnostic
	for _, file := range pass.Files {
		filename := pass.Fset.PositionFor(file.Pos(), false).Filename
		if skipFiles[filename] {
			continue
		}

		for _, decl := range file.Decls {
			fnDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			// Skip functions without body (interface methods, external)
			if fnDecl.Body == nil {
				continue
			}

			diags = append(diags, c.checkFunction(pass, fnDecl)...)
		}
	}
	return diags
}

// checkFunction checks a single function declaration.
func (c *Checker) checkFunction(pass *analysis.Pass, fnDecl *ast.FuncDecl) []analysis.Diagnostic {
	fn := c.getFuncObject(pass, fnDecl)
	if fn == nil {
		return nil
	}

	isMarked := c.spawners.IsSpawner(fn)
	spawnInfo := c.findSpawnCall(fnDecl)

	// Check for missing label
	if !isMarked && spawnInfo != nil {
		return []analysis.Diagnostic{{
			Pos: fnDecl.Name.Pos(),
			Message: fmt.Sprintf("function %q should have //goroutinectx:spawner directive (calls %s with func argument)",
				fnDecl.Name.Name, spawnInfo.methodName),
		}}
	}

	// Check for unnecessary label
	if isMarked && spawnInfo == nil && !hasFuncParams(fn) {
		return []analysis.Diagnostic{{
			Pos:     fnDecl.Name.Pos(),
			Message: fmt.Sprintf("function %q has unnecessary //goroutinectx:spawner directive", fnDecl.Name.Name),
		}}
	}

	return nil
}

// getFuncObject gets the *types.Func for a function declaration.
func (c *Checker) getFuncObject(pass *analysis.Pass, fnDecl *ast.FuncDecl) *types.Func {
	fn, _ := pass.TypesInfo.ObjectOf(fnDecl.Name).(*types.Func)
	return fn
}

// findSpawnCall searches for spawn calls using SSA analysis.
// It checks nested function literals, IIFEs, and higher-order function returns.
func (c *Checker) findSpawnCall(fnDecl *ast.FuncDecl) *spawnCallInfo {
	ssaFn := c.ssaProg.FindFuncDecl(fnDecl)
	if ssaFn == nil {
		return nil
	}

	var info *spawnCallInfo
	w := &internalssa.CallWalker{Closures: true}
	var visit func(internalssa.WalkedCall) bool
	visit = func(site internalssa.WalkedCall) bool {
		info = c.spawnCallInfoOf(site.Call)
		if info != nil {
			return false
		}
		// A call to a function that returns a spawning closure spawns too.
		callee := site.Call.StaticCallee()
		if callee == nil || w.Walked(callee) {
			return true
		}
		for _, closure := range returnedClosures(callee) {
			if !w.Walk(closure, visit) {
				return false
			}
		}
		return true
	}
	w.Walk(ssaFn, visit)

	return info
}

// spawnCallInfoOf reports the API a call spawns through, or nil.
func (c *Checker) spawnCallInfoOf(call *ssa.CallCommon) *spawnCallInfo {
	// Get the called function
	calledFn := internalssa.ExtractCalledFunc(call)
	if calledFn == nil {
		return nil
	}

	// Check against registry
	if match := c.registry.MatchFunc(calledFn); match != nil {
		// For spawnerlabel, we need func arguments
		if internalssa.CallHasFuncArgs(call, match.CallbackArgIdx) {
			return &spawnCallInfo{methodName: match.FullName}
		}
		// TaskSource APIs (e.g., DoAsync) always spawn
		if match.AlwaysSpawns {
			return &spawnCallInfo{methodName: match.FullName}
		}
	}

	// Check if calling a spawner-marked function
	if c.spawners.IsSpawner(calledFn) && internalssa.CallHasFuncArgs(call, 0) {
		return &spawnCallInfo{methodName: calledFn.Name()}
	}

	return nil
}

// returnedClosures gives the closures fn returns.
func returnedClosures(fn *ssa.Function) []*ssa.Function {
	var closures []*ssa.Function
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			ret, ok := instr.(*ssa.Return)
			if !ok {
				continue
			}
			for _, result := range ret.Results {
				if mc, ok := result.(*ssa.MakeClosure); ok {
					if closureFn, ok := mc.Fn.(*ssa.Function); ok {
						closures = append(closures, closureFn)
					}
				}
			}
		}
	}
	return closures
}
