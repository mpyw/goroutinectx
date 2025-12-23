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

	"github.com/mpyw/goroutinectx/internal"
	"github.com/mpyw/goroutinectx/internal/directives/carrier"
	"github.com/mpyw/goroutinectx/internal/directives/deriver"
	"github.com/mpyw/goroutinectx/internal/directives/ignore"
	"github.com/mpyw/goroutinectx/internal/directives/spawner"
	"github.com/mpyw/goroutinectx/internal/registry"
	"github.com/mpyw/goroutinectx/internal/spawnerlabel"
	internalssa "github.com/mpyw/goroutinectx/internal/ssa"
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
	enableConc         bool
	enableSpawner      bool
	enableSpawnerlabel bool
	enableGotask       bool
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
	Analyzer.Flags.BoolVar(&enableConc, "conc", true, "enable conc (sourcegraph/conc) checker")
	Analyzer.Flags.BoolVar(&enableSpawner, "spawner", true, "enable spawner checker")
	Analyzer.Flags.BoolVar(&enableSpawnerlabel, "spawnerlabel", false, "enable spawnerlabel checker")
	Analyzer.Flags.BoolVar(&enableGotask, "gotask", true, "enable gotask checker (requires -goroutine-deriver)")
}

// Analyzer is the main analyzer for goroutinectx.
var Analyzer = &analysis.Analyzer{
	Name:     "goroutinectx",
	Doc:      "checks that context.Context is properly propagated to downstream calls",
	Requires: []*analysis.Analyzer{inspect.Analyzer, internalssa.BuildSSAAnalyzer},
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
	spawners := spawner.Build(pass, externalSpawner)

	// Build enabled checkers map
	enabled := buildEnabledCheckers(spawners)

	// Build registry
	reg := buildRegistry()

	// Build SSA program
	ssaProg := internalssa.Build(pass)

	// Spawner map (nil if disabled)
	var spawnerMap *spawner.Map
	if enableSpawner {
		spawnerMap = spawners
	}

	// Create and run unified checker
	runner := internal.NewRunner(
		reg,
		spawnerMap,
		ssaProg,
		carriers,
		ignoreMaps,
		skipFiles,
	)
	runner.Run(pass, insp)

	// Run spawnerlabel checker if enabled
	if enableSpawnerlabel {
		spawnerlabelChecker := spawnerlabel.New(spawners, reg, ssaProg)
		spawnerlabelChecker.Check(pass, ignoreMaps, skipFiles)
	}

	// Report unused ignore directives
	reportUnusedIgnores(pass, ignoreMaps, enabled)

	return nil, nil
}

// buildSkipFiles creates a set of filenames to skip.
// Generated files are always skipped.
// Test files can be skipped via the driver's built-in -test flag.
func buildSkipFiles(pass *analysis.Pass) map[string]bool {
	skipFiles := make(map[string]bool)

	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Pos()).Filename

		// Always skip generated files
		if ast.IsGenerated(file) {
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

// buildRegistry creates and populates the API registry.
func buildRegistry() *registry.Registry {
	reg := registry.New()

	// Create derivers matcher once if configured
	var derivers *deriver.Matcher
	if goroutineDeriver != "" {
		derivers = deriver.NewMatcher(goroutineDeriver)
	}

	// Register patterns (pass derivers for deriver checking)
	if enableGoroutine {
		internal.RegisterGoroutinePatterns(reg, derivers)
	}
	if enableErrgroup {
		internal.RegisterErrgroupAPIs(reg, derivers)
	}
	if enableWaitgroup {
		internal.RegisterWaitgroupAPIs(reg, derivers)
	}
	if enableConc {
		internal.RegisterConcAPIs(reg, derivers)
	}
	if enableGotask {
		internal.RegisterGotaskAPIs(reg, derivers)
	}

	return reg
}

// buildEnabledCheckers creates a map of which checkers are enabled.
func buildEnabledCheckers(spawners *spawner.Map) ignore.EnabledCheckers {
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

	if enableErrgroup || enableConc {
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
