// Package spawner handles //goroutinectx:spawner directives.
package spawner

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Map tracks functions marked with //goroutinectx:spawner.
// These functions are expected to spawn goroutines with their func arguments.
type Map map[*types.Func]struct{}

// IsSpawner checks if a function is marked as a spawner.
func (m Map) IsSpawner(fn *types.Func) bool {
	_, ok := m[fn]

	return ok
}

// Build scans files for functions marked with the directive.
func Build(pass *analysis.Pass) Map {
	m := make(Map)

	for _, file := range pass.Files {
		buildSpawnersForFile(pass, file, m)
	}

	return m
}

// buildSpawnersForFile scans a single file for spawner directives.
func buildSpawnersForFile(pass *analysis.Pass, file *ast.File, m Map) {
	// Build a map of line -> comment for quick lookup
	lineComments := make(map[int]string)

	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if isSpawnerComment(c.Text) {
				line := pass.Fset.Position(c.Pos()).Line
				lineComments[line] = c.Text
			}
		}
	}

	// Find function declarations that have the directive on the previous line
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		funcLine := pass.Fset.Position(funcDecl.Pos()).Line

		// Check if directive is on previous line
		if _, hasDirective := lineComments[funcLine-1]; !hasDirective {
			continue
		}

		// Get the types.Func for this declaration
		obj := pass.TypesInfo.ObjectOf(funcDecl.Name)
		if obj == nil {
			continue
		}

		fn, ok := obj.(*types.Func)
		if !ok {
			continue
		}

		m[fn] = struct{}{}
	}
}

// isSpawnerComment checks if a comment is a spawner directive.
func isSpawnerComment(text string) bool {
	text = strings.TrimPrefix(text, "//")
	text = strings.TrimSpace(text)

	return strings.HasPrefix(text, "goroutinectx:spawner")
}

// GetFuncFromCall extracts the *types.Func from a call expression if possible.
// Returns nil if the callee cannot be determined statically.
func GetFuncFromCall(pass *analysis.Pass, call *ast.CallExpr) *types.Func {
	var ident *ast.Ident

	switch fun := call.Fun.(type) {
	case *ast.Ident:
		ident = fun
	case *ast.SelectorExpr:
		ident = fun.Sel
	default:
		return nil
	}

	obj := pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return nil
	}

	fn, ok := obj.(*types.Func)
	if !ok {
		return nil
	}

	return fn
}

// FindFuncArgs finds all arguments in a call that are func types.
// Returns the indices and the arguments themselves.
func FindFuncArgs(pass *analysis.Pass, call *ast.CallExpr) []ast.Expr {
	var funcArgs []ast.Expr

	for _, arg := range call.Args {
		tv, ok := pass.TypesInfo.Types[arg]
		if !ok {
			continue
		}

		// Check if argument is a function type
		if _, isFunc := tv.Type.Underlying().(*types.Signature); isFunc {
			funcArgs = append(funcArgs, arg)
		}
	}

	return funcArgs
}
