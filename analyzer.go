// Package goroutinectx provides a go/analysis based analyzer for detecting
// missing context propagation in Go code.
package goroutinectx

import (
	"errors"
	"flag"
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/mpyw/goroutinectx/internal/checkers"
	"github.com/mpyw/goroutinectx/internal/checkers/errgroup"
	"github.com/mpyw/goroutinectx/internal/checkers/goroutine"
	"github.com/mpyw/goroutinectx/internal/checkers/goroutinederive"
	"github.com/mpyw/goroutinectx/internal/checkers/gotask"
	"github.com/mpyw/goroutinectx/internal/checkers/spawner"
	"github.com/mpyw/goroutinectx/internal/checkers/spawnerlabel"
	"github.com/mpyw/goroutinectx/internal/checkers/waitgroup"
	"github.com/mpyw/goroutinectx/internal/context"
	"github.com/mpyw/goroutinectx/internal/directives/carrier"
	"github.com/mpyw/goroutinectx/internal/directives/ignore"
	spawnerdir "github.com/mpyw/goroutinectx/internal/directives/spawner"
)

// Flags for the analyzer.
var (
	goroutineDeriver string
	externalSpawner  string
	contextCarriers  string

	// Checker enable/disable flags (all enabled by default).
	enableGoroutine    bool
	enableWaitgroup    bool
	enableErrgroup     bool
	enableSpawner      bool
	enableSpawnerlabel bool
	enableGotask       bool

	// File filtering flags.
	analyzeTests bool
)

func init() {
	Analyzer.Flags.StringVar(&goroutineDeriver, "goroutine-deriver", "",
		"require goroutines to call this function to derive context (e.g., pkg.Func or pkg.Type.Method)")
	Analyzer.Flags.StringVar(&externalSpawner, "external-spawner", "",
		"comma-separated list of external spawner functions (e.g., pkg.Func or pkg.Type.Method)")
	Analyzer.Flags.StringVar(&contextCarriers, "context-carriers", "",
		"comma-separated list of types to treat as context carriers (e.g., github.com/labstack/echo/v4.Context)")

	// Checker flags (default: all enabled)
	Analyzer.Flags.BoolVar(&enableGoroutine, "goroutine", true, "enable goroutine checker")
	Analyzer.Flags.BoolVar(&enableWaitgroup, "waitgroup", true, "enable waitgroup checker")
	Analyzer.Flags.BoolVar(&enableErrgroup, "errgroup", true, "enable errgroup checker")
	Analyzer.Flags.BoolVar(&enableSpawner, "spawner", true, "enable spawner checker")
	Analyzer.Flags.BoolVar(&enableSpawnerlabel, "spawnerlabel", false, "enable spawnerlabel checker")
	Analyzer.Flags.BoolVar(&enableGotask, "gotask", true, "enable gotask checker (requires -goroutine-deriver)")

	// File filtering flags
	Analyzer.Flags.BoolVar(&analyzeTests, "test", true, "analyze test files (*_test.go)")
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

	// Build set of files to skip
	skipFiles := buildSkipFiles(pass)

	// Parse configuration
	carriers := carrier.Parse(contextCarriers)

	// Build ignore maps for each file (excluding skipped files)
	ignoreMaps := buildIgnoreMaps(pass, skipFiles)

	// Build spawner map from //goroutinectx:spawner directives and -external-spawner flag
	spawners := spawnerdir.Build(pass, externalSpawner)

	// Build enabled checkers map
	enabled := buildEnabledCheckers(spawners)

	// Run AST-based checks (goroutine, errgroup, waitgroup)
	runASTChecks(pass, insp, ignoreMaps, carriers, spawners, skipFiles)

	// Run spawnerlabel checker if enabled
	if enableSpawnerlabel {
		spawnerlabelChecker := spawnerlabel.New(spawners)
		spawnerlabelChecker.Check(pass, ignoreMaps, skipFiles)
	}

	// Report unused ignore directives
	reportUnusedIgnores(pass, ignoreMaps, enabled)

	return nil, nil
}

// buildSkipFiles creates a set of filenames to skip based on flags.
// Generated files are always skipped.
// Test files are skipped when analyzeTests is false.
func buildSkipFiles(pass *analysis.Pass) map[string]bool {
	skipFiles := make(map[string]bool)

	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Pos()).Filename

		// Always skip generated files
		if ast.IsGenerated(file) {
			skipFiles[filename] = true
			continue
		}

		// Skip test files if -test=false
		if !analyzeTests && strings.HasSuffix(filename, "_test.go") {
			skipFiles[filename] = true
		}
	}

	return skipFiles
}

// buildIgnoreMaps creates ignore maps for each file in the pass.
func buildIgnoreMaps(pass *analysis.Pass, skipFiles map[string]bool) map[string]ignore.Map {
	ignoreMaps := make(map[string]ignore.Map)

	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Pos()).Filename
		if skipFiles[filename] {
			continue
		}
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
	spawners *spawnerdir.Map,
	skipFiles map[string]bool,
) {
	// Build context scopes for functions with context parameters
	funcScopes := buildFuncScopes(pass, insp, carriers)

	// Build checkers based on flags
	var (
		callCheckers   []checkers.CallChecker
		goStmtCheckers []checkers.GoStmtChecker
	)

	if enableGoroutine {
		goStmtCheckers = append(goStmtCheckers, goroutine.New())
	}

	if goroutineDeriver != "" {
		goStmtCheckers = append(goStmtCheckers, goroutinederive.New(goroutineDeriver))
	}

	if enableWaitgroup {
		callCheckers = append(callCheckers, waitgroup.New())
	}

	if enableErrgroup {
		callCheckers = append(callCheckers, errgroup.New())
	}

	// Add spawner checker if enabled and any functions are marked
	if enableSpawner && spawners.Len() > 0 {
		callCheckers = append(callCheckers, spawner.New(spawners))
	}

	// gotask checker requires goroutine-deriver to be set
	if goroutineDeriver != "" && enableGotask {
		callCheckers = append(callCheckers, gotask.New(goroutineDeriver))
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

		filename := pass.Fset.Position(n.Pos()).Filename
		if skipFiles[filename] {
			return true
		}

		scope := findEnclosingScope(funcScopes, stack)
		if scope == nil {
			return true // No context in scope
		}

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

// buildEnabledCheckers creates a map of which checkers are enabled.
func buildEnabledCheckers(spawners *spawnerdir.Map) ignore.EnabledCheckers {
	enabled := make(ignore.EnabledCheckers)

	if enableGoroutine {
		enabled[ignore.Goroutine] = true
	}

	if goroutineDeriver != "" {
		enabled[ignore.GoroutineDerive] = true
	}

	if enableWaitgroup {
		enabled[ignore.Waitgroup] = true
	}

	if enableErrgroup {
		enabled[ignore.Errgroup] = true
	}

	if enableSpawner && spawners.Len() > 0 {
		enabled[ignore.Spawner] = true
	}

	if enableSpawnerlabel {
		enabled[ignore.Spawnerlabel] = true
	}

	if goroutineDeriver != "" && enableGotask {
		enabled[ignore.Gotask] = true
	}

	return enabled
}

// reportUnusedIgnores reports any ignore directives that were not used.
func reportUnusedIgnores(pass *analysis.Pass, ignoreMaps map[string]ignore.Map, enabled ignore.EnabledCheckers) {
	for _, ignoreMap := range ignoreMaps {
		for _, unused := range ignoreMap.GetUnusedIgnores(enabled) {
			if len(unused.Checkers) == 0 {
				pass.Reportf(unused.Pos, "unused goroutinectx:ignore directive")
			} else {
				checkerNames := make([]string, len(unused.Checkers))
				for i, c := range unused.Checkers {
					checkerNames[i] = string(c)
				}
				pass.Reportf(unused.Pos, "unused goroutinectx:ignore directive for checker(s): %s", strings.Join(checkerNames, ", "))
			}
		}
	}
}
