package spawnerlabel

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/directives/ignore"
	"github.com/mpyw/goroutinectx/internal/directives/spawner"
)

const checkerName = ignore.Spawnerlabel

// Checker validates that functions are properly labeled with //goroutinectx:spawner.
type Checker struct {
	spawners *spawner.Map
}

// New creates a new spawnerlabel checker.
func New(spawners *spawner.Map) *Checker {
	return &Checker{spawners: spawners}
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

		if info := isSpawnCall(pass, call, c.spawners); info != nil {
			result = info
			return false
		}

		return true
	})

	return result
}
