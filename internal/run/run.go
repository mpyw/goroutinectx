package run

import (
	"errors"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/mpyw/goroutinectx/internal/carrier"
	"github.com/mpyw/goroutinectx/internal/checker"
	"github.com/mpyw/goroutinectx/internal/checkers"
	"github.com/mpyw/goroutinectx/internal/checkers/spawnerlabel"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/directive"
	"github.com/mpyw/goroutinectx/internal/directive/ignore"
	"github.com/mpyw/goroutinectx/internal/directive/spawner"
	"github.com/mpyw/goroutinectx/internal/probe"
	"github.com/mpyw/goroutinectx/internal/scope"
	"github.com/mpyw/goroutinectx/internal/ssa"
)

// Config is what the analyzer's flags, or the golangci-lint plugin's
// settings, resolve to. The json keys are the flags' names, and the plugin
// decodes its settings onto [DefaultConfig] with them.
type Config struct {
	// GoroutineDeriver is -goroutine-deriver: comma-separated OR groups of
	// functions joined by +.
	GoroutineDeriver string `json:"goroutine-deriver"`
	// ExternalSpawner is -external-spawner. The flag is comma-separated.
	ExternalSpawner []string `json:"external-spawner"`
	// ContextCarriers is -context-carriers. The flag is comma-separated.
	ContextCarriers []string `json:"context-carriers"`

	// Each checker is on when its field is true.
	Goroutine    bool `json:"goroutine"`
	Waitgroup    bool `json:"waitgroup"`
	Errgroup     bool `json:"errgroup"`
	Conc         bool `json:"conc"`
	Spawner      bool `json:"spawner"`
	Spawnerlabel bool `json:"spawnerlabel"`
	Gotask       bool `json:"gotask"`
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

	// Build ignore maps for each file (excluding skipped files)
	ignoreMaps := buildIgnoreMaps(pass, skipFiles)

	// Build spawner map from //goroutinectx:spawner directives and -external-spawner flag
	spawners := spawner.Build(pass, cfg.ExternalSpawner)

	// Build SSA program
	ssaProg := ssa.BuildProgram(pass)

	// Build derivers matcher
	var derivers *deriver.Matcher
	if cfg.GoroutineDeriver != "" {
		derivers = deriver.NewMatcher(cfg.GoroutineDeriver)
	}

	r := &runner{
		pass:       pass,
		ssaProg:    ssaProg,
		carriers:   carrier.Parse(cfg.ContextCarriers),
		ignoreMaps: ignoreMaps,
		skipFiles:  skipFiles,
		enabled:    make(ignore.EnabledCheckers),
	}
	r.goStmtCheckers, r.callCheckers = buildCheckers(cfg, derivers, spawners)
	for _, c := range r.goStmtCheckers {
		r.enabled[c.Name()] = true
	}
	for _, c := range r.callCheckers {
		r.enabled[c.Name()] = true
	}

	r.checkNodes(insp)

	// Run spawnerlabel checker if enabled
	if cfg.Spawnerlabel {
		r.enabled[checker.Spawnerlabel] = true
		r.report(checker.Spawnerlabel, spawnerlabel.New(spawners, ssaProg).Check(pass, skipFiles))
	}

	// Report unused ignore directives
	reportUnusedIgnores(pass, ignoreMaps, r.enabled)

	// Report directives written in a form other than //goroutinectx:name
	reportMalformedDirectives(pass, skipFiles)

	return nil
}

// goStmtChecker checks go statements (go func()...).
type goStmtChecker interface {
	// Name returns the checker name, which its reports are ignored by.
	Name() checker.Name
	// CheckGoStmt gives the reports for one go statement.
	CheckGoStmt(cctx *probe.Context, stmt *ast.GoStmt) []analysis.Diagnostic
}

// callChecker checks function call expressions.
type callChecker interface {
	// Name returns the checker name, which its reports are ignored by.
	Name() checker.Name
	// CheckCall gives the reports for one call, none for a call the checker
	// does not check.
	CheckCall(cctx *probe.Context, call *ast.CallExpr) []analysis.Diagnostic
}

// runner runs the go statement and call checkers over one package.
type runner struct {
	pass           *analysis.Pass
	goStmtCheckers []goStmtChecker
	callCheckers   []callChecker
	ssaProg        *ssa.Program
	carriers       []carrier.Carrier
	ignoreMaps     map[string]ignore.Map
	skipFiles      map[string]bool
	// enabled holds the name of each checker built.
	enabled ignore.EnabledCheckers
}

// checkNodes runs the go statement and call checkers on each node inside a
// function with a context in scope.
func (r *runner) checkNodes(insp *inspector.Inspector) {
	// Build context scopes for functions with context parameters
	funcScopes := scope.Build(r.pass, insp, r.carriers)

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

		filename := r.pass.Fset.PositionFor(n.Pos(), false).Filename
		if r.skipFiles[filename] {
			return true
		}

		s := scope.FindEnclosing(funcScopes, stack)
		if s == nil {
			return true // No context in scope
		}

		cctx := &probe.Context{
			Pass:      r.pass,
			Inspector: insp,
			SSAProg:   r.ssaProg,
			CtxNames:  s.CtxNames,
			Carriers:  r.carriers,
		}

		switch node := n.(type) {
		case *ast.GoStmt:
			for _, c := range r.goStmtCheckers {
				r.report(c.Name(), c.CheckGoStmt(cctx, node))
			}
		case *ast.CallExpr:
			for _, c := range r.callCheckers {
				r.report(c.Name(), c.CheckCall(cctx, node))
			}
		}

		return true
	})
}

// report reports each diagnostic that no ignore for name silences at its
// position. The ignore is asked only for a report it would silence. Asking
// it first marked it used on a line with nothing to report, so an ignore that
// silenced nothing was never reported as unused.
func (r *runner) report(name checker.Name, diags []analysis.Diagnostic) {
	for _, d := range diags {
		if !r.shouldIgnore(d.Pos, name) {
			r.pass.Report(d)
		}
	}
}

// shouldIgnore checks if the position should be ignored for the given checker.
func (r *runner) shouldIgnore(pos token.Pos, name checker.Name) bool {
	position := r.pass.Fset.PositionFor(pos, false)
	ignoreMap, ok := r.ignoreMaps[position.Filename]
	if !ok {
		return false
	}
	return ignoreMap.ShouldIgnore(position.Line, name)
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
func buildCheckers(cfg Config, derivers *deriver.Matcher, spawners *spawner.Map) ([]goStmtChecker, []callChecker) {
	var goStmtCheckers []goStmtChecker
	var callCheckers []callChecker

	// Goroutine checkers
	if cfg.Goroutine {
		goStmtCheckers = append(goStmtCheckers, &checkers.Goroutine{})
	}

	if derivers != nil {
		goStmtCheckers = append(goStmtCheckers, checkers.NewGoroutineDerive(derivers))
	}

	// Call checkers
	if cfg.Errgroup {
		callCheckers = append(callCheckers, checkers.NewSpawnCallbackChecker(checker.Errgroup, derivers))
	}

	if cfg.Waitgroup {
		callCheckers = append(callCheckers, checkers.NewSpawnCallbackChecker(checker.Waitgroup, derivers))
	}

	if cfg.Conc {
		callCheckers = append(callCheckers, checkers.NewSpawnCallbackChecker(checker.Conc, derivers))
	}

	if cfg.Spawner && spawners.Len() > 0 {
		callCheckers = append(callCheckers, checkers.NewSpawnerChecker(spawners, derivers))
	}

	if cfg.Gotask && derivers != nil {
		callCheckers = append(callCheckers, checkers.NewGotaskChecker(derivers))
	}

	return goStmtCheckers, callCheckers
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
