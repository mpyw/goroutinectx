// Package goroutinectx provides a go/analysis based analyzer for detecting
// missing context propagation in Go code.
package goroutinectx

import (
	"errors"
	"flag"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/mpyw/goroutinectx/internal/checkers"
	"github.com/mpyw/goroutinectx/internal/checkers/errgroup"
	"github.com/mpyw/goroutinectx/internal/checkers/goroutine"
	"github.com/mpyw/goroutinectx/internal/checkers/goroutinederive"
	"github.com/mpyw/goroutinectx/internal/checkers/gotask"
	"github.com/mpyw/goroutinectx/internal/checkers/spawner"
	"github.com/mpyw/goroutinectx/internal/checkers/waitgroup"
	"github.com/mpyw/goroutinectx/internal/context"
	"github.com/mpyw/goroutinectx/internal/directives/carrier"
	"github.com/mpyw/goroutinectx/internal/directives/ignore"
	spawnerdir "github.com/mpyw/goroutinectx/internal/directives/spawner"
)

// Flags for the analyzer.
var (
	goroutineDeriver string
	contextCarriers  string

	// Checker enable/disable flags (all enabled by default).
	enableErrgroup  bool
	enableWaitgroup bool
	enableGoroutine bool
	enableSpawner   bool
	enableGotask    bool
)

func init() {
	Analyzer.Flags.StringVar(&goroutineDeriver, "goroutine-deriver", "",
		"require goroutines to call this function to derive context (e.g., pkg.Func or pkg.Type.Method)")
	Analyzer.Flags.StringVar(&contextCarriers, "context-carriers", "",
		"comma-separated list of types to treat as context carriers (e.g., github.com/labstack/echo/v4.Context)")

	// Checker flags (default: all enabled)
	Analyzer.Flags.BoolVar(&enableErrgroup, "errgroup", true, "enable errgroup checker")
	Analyzer.Flags.BoolVar(&enableWaitgroup, "waitgroup", true, "enable waitgroup checker")
	Analyzer.Flags.BoolVar(&enableGoroutine, "goroutine", true, "enable goroutine checker")
	Analyzer.Flags.BoolVar(&enableSpawner, "spawner", true, "enable spawner checker")
	Analyzer.Flags.BoolVar(&enableGotask, "gotask", true, "enable gotask checker (requires -goroutine-deriver)")
}

// Analyzer is the main analyzer for goroutinectx.
var Analyzer = &analysis.Analyzer{
	Name:     "goroutinectx",
	Doc:      "checks that context.Context is properly propagated to downstream calls",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
	Flags:    flag.FlagSet{},
}

var ErrNoInspector = errors.New("inspector analyzer result not found")

func run(pass *analysis.Pass) (any, error) {
	insp, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, ErrNoInspector
	}

	// Parse configuration
	carriers := carrier.Parse(contextCarriers)

	// Build ignore maps for each file
	ignoreMaps := buildIgnoreMaps(pass)

	// Build spawner map from //goroutinectx:spawner directives
	spawners := spawnerdir.Build(pass)

	// Run AST-based checks (goroutine, errgroup, waitgroup)
	runASTChecks(pass, insp, ignoreMaps, carriers, spawners)

	return nil, nil
}

// buildIgnoreMaps creates ignore maps for each file in the pass.
func buildIgnoreMaps(pass *analysis.Pass) map[string]ignore.Map {
	ignoreMaps := make(map[string]ignore.Map)

	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Pos()).Filename
		ignoreMaps[filename] = ignore.Build(pass.Fset, file)
	}

	return ignoreMaps
}

// runASTChecks runs AST-based checkers on the pass.
func runASTChecks(
	pass *analysis.Pass,
	insp *inspector.Inspector,
	ignoreMaps map[string]ignore.Map,
	carriers []carrier.Carrier,
	spawners spawnerdir.Map,
) {
	// Build context scopes for functions with context parameters
	funcScopes := buildFuncScopes(pass, insp, carriers)

	// Build checkers based on flags
	var (
		callCheckers   []checkers.CallChecker
		goStmtCheckers []checkers.GoStmtChecker
	)

	if enableErrgroup {
		callCheckers = append(callCheckers, errgroup.New())
	}

	if enableWaitgroup {
		callCheckers = append(callCheckers, waitgroup.New())
	}

	// Add spawner checker if enabled and any functions are marked
	if enableSpawner && len(spawners) > 0 {
		callCheckers = append(callCheckers, spawner.New(spawners))
	}

	// When goroutine-deriver is set, it replaces the base goroutine checker.
	// The derive checker is a more specific version that checks for deriver function calls.
	if goroutineDeriver != "" {
		goStmtCheckers = append(goStmtCheckers, goroutinederive.New(goroutineDeriver))
		// gotask checker also requires goroutine-deriver to be set
		if enableGotask {
			callCheckers = append(callCheckers, gotask.New(goroutineDeriver))
		}
	} else if enableGoroutine {
		goStmtCheckers = append(goStmtCheckers, goroutine.New())
	}

	// Node types we're interested in
	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
		(*ast.FuncLit)(nil),
		(*ast.GoStmt)(nil),
		(*ast.CallExpr)(nil),
	}

	// Check nodes within context-aware functions
	insp.WithStack(nodeFilter, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}

		scope := findEnclosingScope(funcScopes, stack)
		if scope == nil {
			return true // No context in scope
		}

		filename := pass.Fset.Position(n.Pos()).Filename
		cctx := &context.CheckContext{
			Pass:      pass,
			Scope:     scope,
			IgnoreMap: ignoreMaps[filename],
			Carriers:  carriers,
		}

		switch node := n.(type) {
		case *ast.GoStmt:
			for _, checker := range goStmtCheckers {
				checker.CheckGoStmt(cctx, node)
			}
		case *ast.CallExpr:
			for _, checker := range callCheckers {
				checker.CheckCall(cctx, node)
			}
		}

		return true
	})
}

// buildFuncScopes identifies functions with context parameters.
func buildFuncScopes(
	pass *analysis.Pass,
	insp *inspector.Inspector,
	carriers []carrier.Carrier,
) map[ast.Node]*context.Scope {
	funcScopes := make(map[ast.Node]*context.Scope)

	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil), (*ast.FuncLit)(nil)}, func(n ast.Node) {
		var fnType *ast.FuncType

		switch fn := n.(type) {
		case *ast.FuncDecl:
			fnType = fn.Type
		case *ast.FuncLit:
			fnType = fn.Type
		}

		if scope := context.FindScope(pass, fnType, carriers); scope != nil {
			funcScopes[n] = scope
		}
	})

	return funcScopes
}

// findEnclosingScope finds the closest enclosing function with a context parameter.
func findEnclosingScope(funcScopes map[ast.Node]*context.Scope, stack []ast.Node) *context.Scope {
	for i := len(stack) - 1; i >= 0; i-- {
		if scope, ok := funcScopes[stack[i]]; ok {
			return scope
		}
	}

	return nil
}
