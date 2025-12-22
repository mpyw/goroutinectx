package spawnerlabel

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/directives/ignore"
	"github.com/mpyw/goroutinectx/internal/directives/spawner"
	"github.com/mpyw/goroutinectx/internal/registry"
	internalssa "github.com/mpyw/goroutinectx/internal/ssa"
)

const checkerName = ignore.Spawnerlabel

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

// Check runs the spawnerlabel analysis on the given pass.
func (c *Checker) Check(pass *analysis.Pass, ignoreMaps map[string]ignore.Map, skipFiles map[string]bool) {
	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Pos()).Filename
		if skipFiles[filename] {
			continue
		}
		ignoreMap := ignoreMaps[filename]

		for _, decl := range file.Decls {
			fnDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			// Skip functions without body (interface methods, external)
			if fnDecl.Body == nil {
				continue
			}

			c.checkFunction(pass, fnDecl, ignoreMap)
		}
	}
}

// checkFunction checks a single function declaration.
func (c *Checker) checkFunction(pass *analysis.Pass, fnDecl *ast.FuncDecl, ignoreMap ignore.Map) {
	fn := c.getFuncObject(pass, fnDecl)
	if fn == nil {
		return
	}

	isMarked := c.spawners.IsSpawner(fn)
	spawnInfo := c.findSpawnCall(pass, fnDecl)

	// Check for missing label
	if !isMarked && spawnInfo != nil {
		line := pass.Fset.Position(fnDecl.Pos()).Line
		if !ignoreMap.ShouldIgnore(line, checkerName) {
			pass.Reportf(
				fnDecl.Name.Pos(),
				"function %q should have //goroutinectx:spawner directive (calls %s with func argument)",
				fnDecl.Name.Name,
				spawnInfo.methodName,
			)
		}
	}

	// Check for unnecessary label
	if isMarked && spawnInfo == nil && !hasFuncParams(fn) {
		line := pass.Fset.Position(fnDecl.Pos()).Line
		if !ignoreMap.ShouldIgnore(line, checkerName) {
			pass.Reportf(
				fnDecl.Name.Pos(),
				"function %q has unnecessary //goroutinectx:spawner directive",
				fnDecl.Name.Name,
			)
		}
	}
}

// getFuncObject gets the *types.Func for a function declaration.
func (c *Checker) getFuncObject(pass *analysis.Pass, fnDecl *ast.FuncDecl) *types.Func {
	obj := pass.TypesInfo.ObjectOf(fnDecl.Name)
	if obj == nil {
		return nil
	}

	fn, ok := obj.(*types.Func)
	if !ok {
		return nil
	}

	return fn
}

// findSpawnCall searches for spawn calls using SSA analysis.
// It checks nested function literals, IIFEs, and higher-order function returns.
func (c *Checker) findSpawnCall(_ *analysis.Pass, fnDecl *ast.FuncDecl) *spawnCallInfo {
	if c.ssaProg == nil {
		return nil
	}
	ssaFn := c.ssaProg.FindFuncDecl(fnDecl)
	if ssaFn == nil {
		return nil
	}
	return c.findSpawnCallSSA(ssaFn, make(map[*ssa.Function]bool))
}

// findSpawnCallSSA uses SSA to find spawn calls, including in nested functions and IIFEs.
func (c *Checker) findSpawnCallSSA(fn *ssa.Function, visited map[*ssa.Function]bool) *spawnCallInfo {
	if fn == nil || visited[fn] {
		return nil
	}
	visited[fn] = true

	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if info := c.checkInstrForSpawn(instr, visited); info != nil {
				return info
			}
		}
	}

	return nil
}

// checkInstrForSpawn checks a single SSA instruction for spawn calls.
func (c *Checker) checkInstrForSpawn(instr ssa.Instruction, visited map[*ssa.Function]bool) *spawnCallInfo {
	switch v := instr.(type) {
	case *ssa.Call:
		// Check if this is a spawn call
		if info := c.checkCallForSpawn(&v.Call, visited); info != nil {
			return info
		}

		// Check for IIFE - traverse into immediately invoked functions
		if iifeFn := extractIIFE(&v.Call); iifeFn != nil {
			if info := c.findSpawnCallSSA(iifeFn, visited); info != nil {
				return info
			}
		}

	case *ssa.Defer:
		// Check deferred calls too
		if info := c.checkCallForSpawn(&v.Call, visited); info != nil {
			return info
		}

		// Check for deferred IIFE
		if iifeFn := extractIIFE(&v.Call); iifeFn != nil {
			if info := c.findSpawnCallSSA(iifeFn, visited); info != nil {
				return info
			}
		}

	case *ssa.Go:
		// go statement itself is a spawn, but we check the called function
		if info := c.checkCallForSpawn(&v.Call, visited); info != nil {
			return info
		}

	case *ssa.MakeClosure:
		// Traverse into closures created in this function
		if closureFn, ok := v.Fn.(*ssa.Function); ok {
			if info := c.findSpawnCallSSA(closureFn, visited); info != nil {
				return info
			}
		}
	}

	return nil
}

// checkCallForSpawn checks if a call is a spawn call.
func (c *Checker) checkCallForSpawn(call *ssa.CallCommon, visited map[*ssa.Function]bool) *spawnCallInfo {
	// Get the called function
	calledFn := extractCalledFunc(call)
	if calledFn == nil {
		return nil
	}

	// Check against registry
	if entry := c.registry.MatchFunc(calledFn); entry != nil {
		// For spawnerlabel, we need func arguments
		if hasFuncArgsInCall(call, entry.API.CallbackArgIdx) {
			return &spawnCallInfo{methodName: entry.API.FullName()}
		}
		// Method with TaskArgConfig always spawns
		if entry.API.Kind == registry.KindMethod && entry.API.TaskArgConfig != nil {
			return &spawnCallInfo{methodName: entry.API.FullName()}
		}
	}

	// Check if calling a spawner-marked function
	if c.spawners.IsSpawner(calledFn) && hasFuncArgsInCall(call, 0) {
		return &spawnCallInfo{methodName: calledFn.Name()}
	}

	// Check higher-order function returns: if calling a function that returns a spawning func
	if staticCallee := call.StaticCallee(); staticCallee != nil {
		if info := c.checkReturnedFuncForSpawn(staticCallee, visited); info != nil {
			return info
		}
	}

	return nil
}

// checkReturnedFuncForSpawn checks if a function returns another function that contains spawn calls.
func (c *Checker) checkReturnedFuncForSpawn(fn *ssa.Function, visited map[*ssa.Function]bool) *spawnCallInfo {
	if fn == nil || visited[fn] {
		return nil
	}

	// Look for Return instructions that return function values
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			ret, ok := instr.(*ssa.Return)
			if !ok {
				continue
			}

			for _, result := range ret.Results {
				// Check if the result is a closure
				if mc, ok := result.(*ssa.MakeClosure); ok {
					if closureFn, ok := mc.Fn.(*ssa.Function); ok {
						if info := c.findSpawnCallSSA(closureFn, visited); info != nil {
							return info
						}
					}
				}
			}
		}
	}

	return nil
}

// extractCalledFunc extracts the types.Func from a CallCommon.
func extractCalledFunc(call *ssa.CallCommon) *types.Func {
	if call.IsInvoke() {
		// Interface method call
		return call.Method
	}

	// Static call
	if fn := call.StaticCallee(); fn != nil {
		// Try to get the Object directly
		if obj, ok := fn.Object().(*types.Func); ok {
			return obj
		}

		// For generic function instantiations, Object() returns nil.
		// Use Origin() to get the generic function before instantiation.
		if origin := fn.Origin(); origin != nil {
			if obj, ok := origin.Object().(*types.Func); ok {
				return obj
			}
		}
	}

	return nil
}

// extractIIFE checks if a CallCommon is an IIFE.
func extractIIFE(call *ssa.CallCommon) *ssa.Function {
	if call.IsInvoke() {
		return nil
	}

	// Check if the callee is a MakeClosure
	if mc, ok := call.Value.(*ssa.MakeClosure); ok {
		if fn, ok := mc.Fn.(*ssa.Function); ok {
			return fn
		}
	}

	// Check if the callee is a direct function reference (anonymous function)
	if fn, ok := call.Value.(*ssa.Function); ok {
		if fn.Parent() != nil {
			return fn
		}
	}

	return nil
}

// hasFuncArgsInCall checks if the call has func-typed arguments starting from startIdx.
func hasFuncArgsInCall(call *ssa.CallCommon, startIdx int) bool {
	args := call.Args
	if startIdx < 0 || startIdx >= len(args) {
		return false
	}

	for i := startIdx; i < len(args); i++ {
		underlying := args[i].Type().Underlying()
		// Direct function argument
		if _, isFunc := underlying.(*types.Signature); isFunc {
			return true
		}
		// Variadic slice of functions (e.g., ...func(ctx) error)
		if slice, ok := underlying.(*types.Slice); ok {
			if _, isFunc := slice.Elem().Underlying().(*types.Signature); isFunc {
				return true
			}
		}
	}
	return false
}
