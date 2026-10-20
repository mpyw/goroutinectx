// Package goroutinectx provides a go/analysis based analyzer for detecting
// missing context propagation in Go code.
package goroutinectx

import (
	"flag"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"

	"github.com/mpyw/goroutinectx/internal/run"
	"github.com/mpyw/goroutinectx/internal/ssa"
)

// analyzerConfig holds the flags' values. It starts from the defaults.
var analyzerConfig = run.DefaultConfig()

func init() {
	c := &analyzerConfig
	Analyzer.Flags.StringVar(&c.GoroutineDeriver, "goroutine-deriver", c.GoroutineDeriver,
		"require goroutines to call this function to derive context (e.g., pkg.Func or pkg.Type.Method)")
	Analyzer.Flags.Var(analyzerList{&c.ExternalSpawner}, "external-spawner",
		"comma-separated list of external spawner functions (e.g., pkg.Func or pkg.Type.Method)")
	Analyzer.Flags.Var(analyzerList{&c.ContextCarriers}, "context-carriers",
		"comma-separated list of types to treat as context carriers (e.g., github.com/labstack/echo/v4.Context)")

	// Checker flags
	Analyzer.Flags.BoolVar(&c.Goroutine, "goroutine", c.Goroutine, "enable goroutine checker")
	Analyzer.Flags.BoolVar(&c.Waitgroup, "waitgroup", c.Waitgroup, "enable waitgroup checker")
	Analyzer.Flags.BoolVar(&c.Errgroup, "errgroup", c.Errgroup, "enable errgroup checker")
	Analyzer.Flags.BoolVar(&c.Conc, "conc", c.Conc, "enable conc (sourcegraph/conc) checker")
	Analyzer.Flags.BoolVar(&c.Spawner, "spawner", c.Spawner, "enable spawner checker")
	Analyzer.Flags.BoolVar(&c.Spawnerlabel, "spawnerlabel", c.Spawnerlabel, "enable spawnerlabel checker")
	Analyzer.Flags.BoolVar(&c.Gotask, "gotask", c.Gotask, "enable gotask checker (requires -goroutine-deriver)")
}

// Analyzer is the main analyzer for goroutinectx.
var Analyzer = &analysis.Analyzer{
	Name:      "goroutinectx",
	Doc:       "checks that context.Context is properly propagated to downstream calls",
	Requires:  []*analysis.Analyzer{inspect.Analyzer, ssa.ProgramAnalyzer},
	FactTypes: run.FactTypes,
	Run:       analyzerRun,
	Flags:     flag.FlagSet{},
}

// ErrNoInspector is returned when the inspect analyzer's result is missing.
var ErrNoInspector = run.ErrNoInspector

func analyzerRun(pass *analysis.Pass) (any, error) {
	return nil, run.Run(pass, analyzerConfig)
}

// analyzerList is a comma-separated flag held as a list. Setting it replaces
// the list, so setting it to "" empties it.
type analyzerList struct{ list *[]string }

func (l analyzerList) String() string {
	if l.list == nil {
		return ""
	}
	return strings.Join(*l.list, ",")
}

func (l analyzerList) Set(s string) error {
	*l.list = nil
	for part := range strings.SplitSeq(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*l.list = append(*l.list, part)
		}
	}
	return nil
}
