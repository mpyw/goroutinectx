package run

import (
	"errors"
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/mpyw/goroutinectx/internal"
	"github.com/mpyw/goroutinectx/internal/checkers"
	"github.com/mpyw/goroutinectx/internal/checkers/spawnerlabel"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/directive"
	"github.com/mpyw/goroutinectx/internal/directive/carrier"
	"github.com/mpyw/goroutinectx/internal/directive/ignore"
	"github.com/mpyw/goroutinectx/internal/directive/spawner"
	"github.com/mpyw/goroutinectx/internal/registry"
	"github.com/mpyw/goroutinectx/internal/ssa"
)

// Config is what the analyzer's flags, or the golangci-lint plugin's
// settings, resolve to. The strings keep the flags' syntax.
type Config struct {
	// GoroutineDeriver is -goroutine-deriver: comma-separated OR groups of
	// functions joined by +.
	GoroutineDeriver string
	// ExternalSpawner is -external-spawner, comma-separated.
	ExternalSpawner string
	// ContextCarriers is -context-carriers, comma-separated.
	ContextCarriers string

	// Each checker is on when its field is true.
	Goroutine    bool
	Waitgroup    bool
	Errgroup     bool
	Conc         bool
	Spawner      bool
	Spawnerlabel bool
	Gotask       bool
}

// DefaultConfig is the configuration the flags give when none is set.
func DefaultConfig() Config {
	return Config{
		Goroutine: true,
		Waitgroup: true,
		Errgroup:  true,
		Conc:      true,
		Spawner:   true,
		Gotask:    true,
	}
}

// ErrNoInspector is returned when the inspect analyzer's result is missing.
var ErrNoInspector = errors.New("inspector analyzer result not found")

// Run analyzes one package with cfg. The analyzer that calls it must require
// inspect.Analyzer and ssa.ProgramAnalyzer.
func Run(pass *analysis.Pass, cfg Config) error {
	insp, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return ErrNoInspector
	}

	// Build set of files to skip
	skipFiles := buildSkipFiles(pass)

	// Parse configuration
	carriers := carrier.Parse(cfg.ContextCarriers)

	// Build ignore maps for each file (excluding skipped files)
	ignoreMaps := buildIgnoreMaps(pass, skipFiles)

	// Build spawner map from //goroutinectx:spawner directives and -external-spawner flag
	spawners := spawner.Build(pass, cfg.ExternalSpawner)

	// Build enabled checkers map
	enabled := buildEnabledCheckers(cfg, spawners)

	// Build SSA program
	ssaProg := ssa.BuildProgram(pass)

	// Build derivers matcher
	var derivers *deriver.Matcher
	if cfg.GoroutineDeriver != "" {
		derivers = deriver.NewMatcher(cfg.GoroutineDeriver)
	}

	// Build checkers
	goStmtCheckers, callCheckers := buildCheckers(cfg, derivers, spawners)

	// Create and run runner
	runner := internal.NewRunner(
		goStmtCheckers,
		callCheckers,
		ssaProg,
		carriers,
		ignoreMaps,
		skipFiles,
	)
	runner.Run(pass, insp)

	// Run spawnerlabel checker if enabled
	if cfg.Spawnerlabel {
		reg := registry.New()

		// Register APIs for spawnerlabel detection
		internal.RegisterErrgroupAPIs(reg)
		internal.RegisterWaitgroupAPIs(reg)
		internal.RegisterConcAPIs(reg)
		internal.RegisterGotaskAPIs(reg)

		spawnerlabelChecker := spawnerlabel.New(spawners, reg, ssaProg)
		spawnerlabelChecker.Check(pass, ignoreMaps, skipFiles)
	}

	// Report unused ignore directives
	reportUnusedIgnores(pass, ignoreMaps, enabled)

	// Report directives written in a form other than //goroutinectx:name
	reportMalformedDirectives(pass, skipFiles)

	return nil
}

// buildSkipFiles creates a set of filenames to skip.
func buildSkipFiles(pass *analysis.Pass) map[string]bool {
	skipFiles := make(map[string]bool)

	for _, file := range pass.Files {
		filename := pass.Fset.PositionFor(file.Pos(), false).Filename

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
		filename := pass.Fset.PositionFor(file.Pos(), false).Filename
		if skipFiles[filename] {
			continue
		}
		m, problems := ignore.Build(pass.Fset, file)
		ignoreMaps[filename] = m
		for _, p := range problems {
			pass.Reportf(p.Pos, "%s", p.Message)
		}
	}

	return ignoreMaps
}

// buildCheckers creates the checker instances.
func buildCheckers(cfg Config, derivers *deriver.Matcher, spawners *spawner.Map) ([]internal.GoStmtChecker, []internal.CallChecker) {
	var goStmtCheckers []internal.GoStmtChecker
	var callCheckers []internal.CallChecker

	// Goroutine checkers
	if cfg.Goroutine {
		goStmtCheckers = append(goStmtCheckers, &checkers.Goroutine{})
	}

	if derivers != nil {
		goStmtCheckers = append(goStmtCheckers, checkers.NewGoroutineDerive(derivers))
	}

	// Call checkers
	if cfg.Errgroup {
		callCheckers = append(callCheckers, checkers.NewErrgroupSpawnChecker(derivers))
	}

	if cfg.Waitgroup {
		callCheckers = append(callCheckers, checkers.NewWaitgroupSpawnChecker(derivers))
	}

	if cfg.Conc {
		callCheckers = append(callCheckers, checkers.NewConcSpawnChecker(derivers))
	}

	if cfg.Spawner && spawners.Len() > 0 {
		callCheckers = append(callCheckers, checkers.NewSpawnerChecker(spawners, derivers))
	}

	if cfg.Gotask && derivers != nil {
		if gotaskChecker := checkers.NewGotaskChecker(derivers); gotaskChecker != nil {
			callCheckers = append(callCheckers, gotaskChecker)
		}
	}

	return goStmtCheckers, callCheckers
}

// buildEnabledCheckers creates a map of which checkers are enabled.
func buildEnabledCheckers(cfg Config, spawners *spawner.Map) ignore.EnabledCheckers {
	enabled := make(ignore.EnabledCheckers)

	if cfg.Goroutine {
		enabled[ignore.Goroutine] = true
	}

	if cfg.GoroutineDeriver != "" {
		enabled[ignore.GoroutineDerive] = true
	}

	if cfg.Waitgroup {
		enabled[ignore.Waitgroup] = true
	}

	if cfg.Errgroup {
		enabled[ignore.Errgroup] = true
	}

	if cfg.Conc {
		enabled[ignore.Conc] = true
	}

	if cfg.Spawner && spawners.Len() > 0 {
		enabled[ignore.Spawner] = true
	}

	if cfg.Spawnerlabel {
		enabled[ignore.Spawnerlabel] = true
	}

	if cfg.GoroutineDeriver != "" && cfg.Gotask {
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

// reportMalformedDirectives reports comments that look like a directive but
// are not in the canonical form. Such a comment is not a directive, and would
// otherwise be dropped with no sign: a spawner would not be checked, and an
// ignore would not suppress anything.
func reportMalformedDirectives(pass *analysis.Pass, skipFiles map[string]bool) {
	for _, file := range pass.Files {
		if skipFiles[pass.Fset.PositionFor(file.Pos(), false).Filename] {
			continue
		}
		for _, cg := range file.Comments {
			for _, c := range cg.List {
				if directive.Malformed(c.Text) {
					pass.Reportf(c.Pos(), "malformed goroutinectx directive: write it as //goroutinectx:name")
				} else if d, ok := directive.Parse(c.Text); ok && !directive.Known(d.Name) {
					pass.Reportf(c.Pos(), "unknown directive goroutinectx:%s", d.Name)
				}
			}
		}
	}
}
