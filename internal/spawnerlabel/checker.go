package spawnerlabel

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/directives/ignore"
	"github.com/mpyw/goroutinectx/internal/directives/spawner"
	"github.com/mpyw/goroutinectx/internal/registry"
)

const checkerName = ignore.Spawnerlabel

// Checker validates that functions are properly labeled with //goroutinectx:spawner.
type Checker struct {
	spawners *spawner.Map
	registry *registry.Registry
}

// New creates a new spawnerlabel checker.
func New(spawners *spawner.Map, reg *registry.Registry) *Checker {
	return &Checker{spawners: spawners, registry: reg}
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
	spawnInfo := c.findSpawnCallInBody(pass, fnDecl.Body)

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

// findSpawnCallInBody searches the function body for spawn calls with func arguments.
// Returns info about the first spawn call found, or nil if none found.
func (c *Checker) findSpawnCallInBody(pass *analysis.Pass, body *ast.BlockStmt) *spawnCallInfo {
	var result *spawnCallInfo

	ast.Inspect(body, func(n ast.Node) bool {
		if result != nil {
			return false // Already found one
		}

		// Skip nested function literals - they have their own scope
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check registered APIs (errgroup, waitgroup, gotask, etc.)
		if entry, _ := c.registry.Match(pass, call); entry != nil {
			// For spawnerlabel, verify spawn happens via this call:
			// 1. Method calls with TaskConstructor (e.g., Task.DoAsync) always spawn
			// 2. Function calls need func arguments (e.g., errgroup.Go, DoAllFns)
			isMethodWithTask := entry.API.Kind == registry.KindMethod && entry.API.TaskConstructor != nil
			hasFuncArgs := hasFuncArgFromIndex(pass, call, entry.API.CallbackArgIdx)
			if isMethodWithTask || hasFuncArgs {
				result = &spawnCallInfo{methodName: entry.API.FullName()}
				return false
			}
		}

		// Check spawner-marked functions
		if info := isSpawnerMarkedCall(pass, call, c.spawners); info != nil {
			result = info
			return false
		}

		return true
	})

	return result
}

// hasFuncArgFromIndex checks if the call has any func-typed argument starting from the given index.
func hasFuncArgFromIndex(pass *analysis.Pass, call *ast.CallExpr, startIdx int) bool {
	if startIdx < 0 || startIdx >= len(call.Args) {
		return false
	}

	for i := startIdx; i < len(call.Args); i++ {
		arg := call.Args[i]
		tv, ok := pass.TypesInfo.Types[arg]
		if !ok {
			continue
		}
		if _, isFunc := tv.Type.Underlying().(*types.Signature); isFunc {
			return true
		}
	}
	return false
}
