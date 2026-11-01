// Package goroutinectx provides a go/analysis based analyzer for detecting
// missing context propagation in Go code.
package goroutinectx

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/checkername"
	"github.com/mpyw/goroutinectx/internal/checkers"
	"github.com/mpyw/goroutinectx/internal/run"
)

// Analyzer is the main analyzer for goroutinectx. It takes its settings from
// flags, as the goroutinectx command and go vet give them. A program that
// holds its own settings builds an analyzer with New.
var Analyzer = analyzerWithFlags()

// ErrNoInspector is returned when the inspect analyzer's result is missing.
var ErrNoInspector = run.ErrNoInspector

// ErrNoSSA is returned when the buildssa analyzer's result is missing.
var ErrNoSSA = run.ErrNoSSA

// ErrNoSpawners is returned when the goroutinectxspawner analyzer's result is
// missing.
var ErrNoSpawners = run.ErrNoSpawners

// Config holds the settings of an analyzer that New builds. Each field is
// the flag of the same name. The zero Config keeps every checker at its
// default.
type Config struct {
	// GoroutineDeriver requires goroutines to call this function to derive
	// their context, such as github.com/org/pkg.Func or
	// github.com/org/pkg.Type.Method.
	GoroutineDeriver string
	// ExternalSpawner names external functions that spawn goroutines, such
	// as github.com/org/pkg.Func or github.com/org/pkg.Type.Method.
	ExternalSpawner []string
	// ContextCarriers names types to treat as context carriers, such as
	// github.com/labstack/echo/v4.Context.
	ContextCarriers []string
	// ReportWaiters reports a goroutine that only waits and signals too, as
	// one that runs wg.Wait() and then close(done). By default it is not
	// reported.
	ReportWaiters bool
	// Checkers turns a checker on or off by its name, such as "goroutine". A
	// checker left out keeps its default.
	Checkers map[string]bool
}

// New builds an analyzer with the settings of cfg, and no flags. Each call
// builds one of its own, so two of them share nothing. It fails on a checker
// it does not know, and on a function or a type that names no import path.
func New(cfg Config) (*analysis.Analyzer, error) {
	c := run.DefaultConfig()
	c.GoroutineDeriver = cfg.GoroutineDeriver
	c.ExternalSpawner = cfg.ExternalSpawner
	c.ContextCarriers = cfg.ContextCarriers
	c.ReportWaiters = cfg.ReportWaiters
	// In the order of the names, so that of several unknown checkers the
	// error names the same one each time.
	for _, name := range slices.Sorted(maps.Keys(cfg.Checkers)) {
		if !checkers.Settable(checkername.Name(name)) {
			return nil, fmt.Errorf("unknown checker %q", name)
		}
		c.Checkers[checkername.Name(name)] = cfg.Checkers[name]
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return analyzerBuild(&c), nil
}

// analyzerWithFlags builds the analyzer with its flags. The flags' values
// start from the defaults and live in the closure of Run, so nothing is
// shared but the analyzer itself.
func analyzerWithFlags() *analysis.Analyzer {
	c := new(run.DefaultConfig())
	a := analyzerBuild(c)
	a.Flags.Var(analyzerDeriver{&c.GoroutineDeriver}, "goroutine-deriver",
		"require goroutines to call this function to derive context (e.g., github.com/org/pkg.Func or github.com/org/pkg.Type.Method)")
	a.Flags.Var(analyzerList{&c.ExternalSpawner, func(l []string) error { return run.Config{ExternalSpawner: l}.Validate() }}, "external-spawner",
		"comma-separated list of external spawner functions (e.g., github.com/org/pkg.Func or github.com/org/pkg.Type.Method)")
	a.Flags.Var(analyzerList{&c.ContextCarriers, func(l []string) error { return run.Config{ContextCarriers: l}.Validate() }}, "context-carriers",
		"comma-separated list of types to treat as context carriers (e.g., github.com/labstack/echo/v4.Context)")
	a.Flags.BoolVar(&c.ReportWaiters, "report-waiters", false,
		"also report goroutines that only wait (e.g., wg.Wait()) and signal (e.g., close(done))")

	for _, d := range checkers.Defs {
		if d.HasFlag() {
			a.Flags.Var(analyzerChecker{c.Checkers, d.Name}, string(d.Name), d.FlagDoc)
		}
	}
	return a
}

// analyzerBuild builds an analyzer that runs with the settings c holds when
// a pass starts.
func analyzerBuild(c *run.Config) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "goroutinectx",
		Doc:  "checks that context.Context is properly propagated to downstream calls",
		// A copy, so that a change to one analyzer's Requires reaches no
		// other analyzer.
		Requires: slices.Clone(run.Requires),
		Run: func(pass *analysis.Pass) (any, error) {
			return nil, run.Run(pass, *c)
		},
	}
}

// analyzerList is a comma-separated flag held as a list. Setting it replaces
// the list, so setting it to "" empties it. validate, when set, rejects a
// list.
type analyzerList struct {
	list     *[]string
	validate func([]string) error
}

func (l analyzerList) String() string {
	if l.list == nil {
		return ""
	}
	return strings.Join(*l.list, ",")
}

func (l analyzerList) Set(s string) error {
	var list []string
	for part := range strings.SplitSeq(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			list = append(list, part)
		}
	}
	if l.validate != nil {
		if err := l.validate(list); err != nil {
			return err
		}
	}
	*l.list = list
	return nil
}

// analyzerDeriver is -goroutine-deriver. Setting it rejects a function
// specification that names no import path.
type analyzerDeriver struct{ value *string }

func (d analyzerDeriver) String() string {
	if d.value == nil {
		return ""
	}
	return *d.value
}

func (d analyzerDeriver) Set(s string) error {
	if err := (run.Config{GoroutineDeriver: s}).Validate(); err != nil {
		return err
	}
	*d.value = s
	return nil
}

// analyzerChecker is the flag that turns one checker on or off. It is a
// boolean flag, as flag.BoolVar would give, that sets the checker's entry in
// checkers.
type analyzerChecker struct {
	checkers map[checkername.Name]bool
	name     checkername.Name
}

func (c analyzerChecker) String() string {
	return strconv.FormatBool(c.checkers[c.name])
}

func (c analyzerChecker) Set(s string) error {
	on, err := strconv.ParseBool(s)
	if err != nil {
		// The error flag.BoolVar gives.
		return errors.New("parse error")
	}
	c.checkers[c.name] = on
	return nil
}

// IsBoolFlag lets the flag be set with no value, and lists it as a boolean
// flag.
func (analyzerChecker) IsBoolFlag() bool { return true }
