// Package goroutinectx provides a go/analysis based analyzer for detecting
// missing context propagation in Go code.
package goroutinectx

import (
	"fmt"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/run"
)

// Analyzer is the main analyzer for goroutinectx. It takes its settings from
// flags, as the goroutinectx command and go vet give them. A program that
// holds its own settings builds an analyzer with New.
var Analyzer = analyzerWithFlags()

// ErrNoInspector is returned when the inspect analyzer's result is missing.
var ErrNoInspector = run.ErrNoInspector

// Config holds the settings of an analyzer that New builds. Each field is
// the flag of the same name. The zero Config keeps every checker at its
// default.
type Config struct {
	// GoroutineDeriver requires goroutines to call this function to derive
	// their context, such as pkg.Func or pkg.Type.Method.
	GoroutineDeriver string
	// ExternalSpawner names external functions that spawn goroutines.
	ExternalSpawner []string
	// ContextCarriers names types to treat as context carriers, such as
	// github.com/labstack/echo/v4.Context.
	ContextCarriers []string
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
	checkers := analyzerCheckers(&c)
	for name, on := range cfg.Checkers {
		field, ok := checkers[name]
		if !ok {
			return nil, fmt.Errorf("unknown checker %q", name)
		}
		*field = on
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
		"require goroutines to call this function to derive context (e.g., pkg.Func or pkg.Type.Method)")
	a.Flags.Var(analyzerList{&c.ExternalSpawner, func(l []string) error { return run.Config{ExternalSpawner: l}.Validate() }}, "external-spawner",
		"comma-separated list of external spawner functions (e.g., pkg.Func or pkg.Type.Method)")
	a.Flags.Var(analyzerList{&c.ContextCarriers, func(l []string) error { return run.Config{ContextCarriers: l}.Validate() }}, "context-carriers",
		"comma-separated list of types to treat as context carriers (e.g., github.com/labstack/echo/v4.Context)")

	// Checker flags
	a.Flags.BoolVar(&c.Goroutine, "goroutine", c.Goroutine, "enable goroutine checker")
	a.Flags.BoolVar(&c.Waitgroup, "waitgroup", c.Waitgroup, "enable waitgroup checker")
	a.Flags.BoolVar(&c.Errgroup, "errgroup", c.Errgroup, "enable errgroup checker")
	a.Flags.BoolVar(&c.Conc, "conc", c.Conc, "enable conc (sourcegraph/conc) checker")
	a.Flags.BoolVar(&c.Spawner, "spawner", c.Spawner, "enable spawner checker")
	a.Flags.BoolVar(&c.Spawnerlabel, "spawnerlabel", c.Spawnerlabel, "enable spawnerlabel checker")
	a.Flags.BoolVar(&c.Gotask, "gotask", c.Gotask, "enable gotask checker (requires -goroutine-deriver)")
	return a
}

// analyzerCheckers maps each checker's name to its field of c.
func analyzerCheckers(c *run.Config) map[string]*bool {
	return map[string]*bool{
		"goroutine":    &c.Goroutine,
		"waitgroup":    &c.Waitgroup,
		"errgroup":     &c.Errgroup,
		"conc":         &c.Conc,
		"spawner":      &c.Spawner,
		"spawnerlabel": &c.Spawnerlabel,
		"gotask":       &c.Gotask,
	}
}

// analyzerBuild builds an analyzer that runs with the settings c holds when
// a pass starts.
func analyzerBuild(c *run.Config) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     "goroutinectx",
		Doc:      "checks that context.Context is properly propagated to downstream calls",
		Requires: run.Requires,
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
