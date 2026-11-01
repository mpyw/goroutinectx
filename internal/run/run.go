package run

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/mpyw/goroutinectx/internal/carrier"
	"github.com/mpyw/goroutinectx/internal/checkername"
	"github.com/mpyw/goroutinectx/internal/checkers"
	"github.com/mpyw/goroutinectx/internal/ctxkind"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/directive"
	"github.com/mpyw/goroutinectx/internal/directive/ignore"
	"github.com/mpyw/goroutinectx/internal/directive/spawner"
	"github.com/mpyw/goroutinectx/internal/env"
	"github.com/mpyw/goroutinectx/internal/flow"
	"github.com/mpyw/goroutinectx/internal/funcspec"
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
	// ReportWaiters is -report-waiters: report a goroutine that only waits
	// and signals too, as [flow.Flow.IsWaiter] tells one (#230).
	ReportWaiters bool `json:"report-waiters"`

	// Checkers turns each checker that has a flag on or off by its name: on
	// when its value is true. In JSON each is a key of its own, beside the
	// keys above.
	Checkers map[checkername.Name]bool `json:"-"`
}

// DefaultConfig is the configuration the flags give when none is set.
func DefaultConfig() Config {
	cfg := Config{Checkers: make(map[checkername.Name]bool)}
	for _, d := range checkers.Defs {
		if d.HasFlag() {
			cfg.Checkers[d.Name] = d.Default
		}
	}
	return cfg
}

// UnmarshalJSON decodes settings onto cfg: an object keyed by the flags'
// names, which is an error with any other key. It decodes them as onto a
// struct with a field for each flag, so the settings it takes and the errors
// it gives are those of such a struct named Config.
func (cfg *Config) UnmarshalJSON(data []byte) error {
	configType := reflect.TypeFor[Config]()
	var fields []reflect.StructField
	for i := range configType.NumField() {
		if f := configType.Field(i); f.Tag.Get("json") != "-" {
			fields = append(fields, reflect.StructField{Name: f.Name, Type: f.Type, Tag: f.Tag})
		}
	}
	nflags := len(fields)
	var names []checkername.Name
	for _, d := range checkers.Defs {
		if d.HasFlag() {
			fields = append(fields, reflect.StructField{
				Name: fmt.Sprintf("Checker%d", len(names)),
				Type: reflect.TypeFor[bool](),
				Tag:  reflect.StructTag(fmt.Sprintf("json:%q", d.Name)),
			})
			names = append(names, d.Name)
		}
	}
	flat := reflect.New(reflect.StructOf(fields)).Elem()
	src := reflect.ValueOf(cfg).Elem()
	for i := range nflags {
		flat.Field(i).Set(src.FieldByName(fields[i].Name))
	}
	for i, name := range names {
		flat.Field(nflags + i).SetBool(cfg.Checkers[name])
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(flat.Addr().Interface()); err != nil {
		// Name the struct, which has no name, as the errors of Config did.
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			if typeErr.Field != "" {
				typeErr.Struct = configType.Name()
			}
			if typeErr.Type == flat.Type() {
				typeErr.Type = configType
			}
		}
		return err
	}

	for i := range nflags {
		src.FieldByName(fields[i].Name).Set(flat.Field(i))
	}
	enabled := make(map[checkername.Name]bool, len(names))
	for i, name := range names {
		enabled[name] = flat.Field(nflags + i).Bool()
	}
	cfg.Checkers = enabled
	return nil
}

// Validate reports the first flag value Run would reject: a function
// specification of -goroutine-deriver or -external-spawner, or a type of
// -context-carriers, that names no import path.
func (cfg Config) Validate() error {
	_, err := cfg.parse()
	return err
}

// parsedConfig holds the flag values of a Config that are parsed.
type parsedConfig struct {
	derivers *deriver.Matcher
	external []funcspec.Spec
	carriers []carrier.Carrier
}

// parse parses the derivers, the external spawners and the carriers.
func (cfg Config) parse() (parsedConfig, error) {
	var p parsedConfig
	var err error
	if p.external, err = spawner.ParseExternal(cfg.ExternalSpawner); err != nil {
		return parsedConfig{}, err
	}
	if p.carriers, err = carrier.Parse(cfg.ContextCarriers); err != nil {
		return parsedConfig{}, err
	}
	if cfg.GoroutineDeriver != "" {
		if p.derivers, err = deriver.NewMatcher(cfg.GoroutineDeriver); err != nil {
			return parsedConfig{}, err
		}
	}
	return p, nil
}

// Requires are the analyzers Run reads the results of. The analyzer that
// calls it must require them. None of them is built for dependencies but
// spawner.Analyzer, which exports facts and reads only comments, so SSA is
// built only for the packages analyzed.
var Requires = []*analysis.Analyzer{inspect.Analyzer, flow.ProgramAnalyzer, spawner.Analyzer}

// ErrNoInspector is returned when the inspect analyzer's result is missing.
var ErrNoInspector = errors.New("inspector analyzer result not found")

// ErrNoSpawners is returned when spawner.Analyzer's result is missing.
var ErrNoSpawners = errors.New("spawner analyzer result not found")

// ErrNoSSA is returned when the buildssa analyzer's result is missing.
var ErrNoSSA = errors.New("buildssa analyzer result not found")

// Run analyzes one package with cfg. The analyzer that calls it must require
// [Requires].
func Run(pass *analysis.Pass, cfg Config) error {
	insp, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return ErrNoInspector
	}

	parsed, err := cfg.parse()
	if err != nil {
		return err
	}

	// Read the directives of each file that is not generated
	skipFiles, ignoreMaps := readDirectives(pass)

	// Build spawner map from //goroutinectx:spawner directives and -external-spawner flag
	marked, ok := pass.ResultOf[spawner.Analyzer].(*spawner.Marked)
	if !ok {
		return ErrNoSpawners
	}
	spawners := spawner.Build(marked, parsed.external)

	// Build SSA program
	ssaProg := flow.BuildProgram(pass)
	if ssaProg == nil {
		return ErrNoSSA
	}

	r := &runner{
		env:        env.New(pass, ssaProg, parsed.carriers, parsed.derivers, spawners, cfg.ReportWaiters),
		ignoreMaps: ignoreMaps,
		skipFiles:  skipFiles,
		enabled:    make(ignore.EnabledCheckers),
	}
	packageCheckers := r.buildCheckers(cfg)

	r.checkNodes(insp)

	// Run the package checkers, such as spawnerlabel
	for _, c := range packageCheckers {
		r.report(c.Name(), c.Check(pass, skipFiles), token.NoPos)
	}

	// Report unused ignore directives
	reportUnusedIgnores(pass, ignoreMaps, r.enabled)

	return nil
}

// goStmtChecker checks go statements (go func()...).
type goStmtChecker interface {
	// Name returns the checker name, which its reports are ignored by.
	Name() checkername.Name
	// CheckGoStmt gives the reports for one go statement. ctxName is the
	// first context parameter of the nearest enclosing function that has
	// one, for the message.
	CheckGoStmt(e *env.Env, ctxName string, stmt *ast.GoStmt) []analysis.Diagnostic
}

// callChecker checks function call expressions.
type callChecker interface {
	// Name returns the checker name, which its reports are ignored by.
	Name() checkername.Name
	// CheckCall gives the reports for one call, none for a call the checker
	// does not check. ctxName is as for CheckGoStmt.
	CheckCall(e *env.Env, ctxName string, call *ast.CallExpr) []analysis.Diagnostic
}

// packageChecker checks a whole package once, after the other checkers.
type packageChecker interface {
	// Name returns the checker name, which its reports are ignored by.
	Name() checkername.Name
	// Check gives the reports for the package, skipping skipFiles.
	Check(pass *analysis.Pass, skipFiles map[string]bool) []analysis.Diagnostic
}

// runner runs the go statement and call checkers over one package.
type runner struct {
	env            *env.Env
	goStmtCheckers []goStmtChecker
	callCheckers   []callChecker
	ignoreMaps     map[string]ignore.Map
	skipFiles      map[string]bool
	// enabled holds the name of each checker built.
	enabled ignore.EnabledCheckers
}

// checkNodes runs the go statement and call checkers on each node inside a
// function with a context in scope.
func (r *runner) checkNodes(insp *inspector.Inspector) {
	pass := r.env.Pass
	ctxNames := runFuncContexts(r.env, insp)

	// Node types we're interested in. A file is visited to skip a generated
	// one whole.
	nodeFilter := []ast.Node{
		(*ast.File)(nil),
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

		if file, ok := n.(*ast.File); ok {
			return !r.skipFiles[pass.Fset.PositionFor(file.Pos(), false).Filename]
		}

		ctxName, ok := runEnclosingContext(ctxNames, stack)
		if !ok {
			return true // No context in scope
		}

		switch node := n.(type) {
		case *ast.GoStmt:
			for _, c := range r.goStmtCheckers {
				r.report(c.Name(), c.CheckGoStmt(r.env, ctxName, node), token.NoPos)
			}
		case *ast.CallExpr:
			for _, c := range r.callCheckers {
				r.report(c.Name(), c.CheckCall(r.env, ctxName, node), node.Pos())
			}
		}

		return true
	})
}

// runFuncContexts maps each function declaration and literal that has a
// context parameter, as [ctxkind.Classifier.ParamNames] finds one with
// [ctxkind.Param], to the name of the first. A blank parameter and a
// receiver are left out.
func runFuncContexts(e *env.Env, insp *inspector.Inspector) map[ast.Node]string {
	ctxNames := make(map[ast.Node]string)
	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil), (*ast.FuncLit)(nil)}, func(n ast.Node) {
		var fnType *ast.FuncType
		switch fn := n.(type) {
		case *ast.FuncDecl:
			fnType = fn.Type
		case *ast.FuncLit:
			fnType = fn.Type
		}
		if names := e.Kinds.ParamNames(e.Pass.TypesInfo, fnType, ctxkind.Param); len(names) > 0 {
			ctxNames[n] = names[0]
		}
	})
	return ctxNames
}

// runEnclosingContext gives the context name of the innermost function of
// stack that has one. A function literal without a context parameter of its
// own is checked under the context of the function around it.
func runEnclosingContext(ctxNames map[ast.Node]string, stack []ast.Node) (string, bool) {
	for _, n := range slices.Backward(stack) {
		if name, ok := ctxNames[n]; ok {
			return name, true
		}
	}
	return "", false
}

// report reports each diagnostic that no ignore for name silences, at its
// own line or, for a report inside a call, at the line the call starts:
// call is the call's position, or token.NoPos. The ignore is asked only for
// a report it would silence. Asking it first marked it used on a line with
// nothing to report, so an ignore that silenced nothing was never reported
// as unused. Only the ignore that silences the report is marked used: the
// one at its own line, or else the one at the call.
func (r *runner) report(name checkername.Name, diags []analysis.Diagnostic, call token.Pos) {
	for _, d := range diags {
		e := r.ignoreAt(d.Pos, name)
		if e == nil && call.IsValid() {
			e = r.ignoreAt(call, name)
		}
		if e != nil {
			e.MarkUsed(name)
			continue
		}
		r.env.Pass.Report(d)
	}
}

// ignoreAt gives the ignore that silences the reports of the checker name
// at pos, or nil.
func (r *runner) ignoreAt(pos token.Pos, name checkername.Name) *ignore.Entry {
	position := r.env.Pass.Fset.PositionFor(pos, false)
	return r.ignoreMaps[position.Filename].Covering(position.Line, name)
}

// readDirectives scans each file once with directive.Scan, skipping a
// generated file whole, and reports the comments addressed to goroutinectx
// that do nothing. It gives the names of the files skipped, and the ignore
// map of each file read.
func readDirectives(pass *analysis.Pass) (map[string]bool, map[string]ignore.Map) {
	skipFiles := make(map[string]bool)
	ignoreMaps := make(map[string]ignore.Map)

	for _, file := range pass.Files {
		filename := pass.Fset.PositionFor(file.Pos(), false).Filename
		if ast.IsGenerated(file) {
			skipFiles[filename] = true
			continue
		}
		found := directive.Scan(file)
		ignoreMaps[filename] = ignore.Build(pass.Fset, file, found.Ignores)
		for _, p := range found.Problems {
			pass.Reportf(p.Pos, "%s", p.Message)
		}
	}

	return skipFiles, ignoreMaps
}

// buildCheckers builds each checker of [checkers.Defs] that cfg turns on,
// and that has the deriver it needs, from r's Env, and marks it enabled. It
// gives the go statement and call checkers to r, and returns the package
// checkers.
func (r *runner) buildCheckers(cfg Config) []packageChecker {
	var packageCheckers []packageChecker
	for _, d := range checkers.Defs {
		if (d.HasFlag() && !cfg.Checkers[d.Name]) || (d.NeedsDeriver && r.env.Derivers == nil) {
			continue
		}
		switch c := d.Build(r.env).(type) {
		case goStmtChecker:
			r.goStmtCheckers = append(r.goStmtCheckers, c)
		case callChecker:
			r.callCheckers = append(r.callCheckers, c)
		case packageChecker:
			packageCheckers = append(packageCheckers, c)
		default:
			panic(fmt.Sprintf("checker %s is not a go statement, call or package checker", d.Name))
		}
		r.enabled[d.Name] = true
	}
	return packageCheckers
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
