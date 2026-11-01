package goroutinectx_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/mpyw/goroutinectx"
)

// The deriver and carrier settings that the fixtures are written for.
const (
	apmDeriver      = "github.com/my-example-app/telemetry/apm.NewGoroutineContext"
	newrelicDeriver = "github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+" +
		"github.com/newrelic/go-agent/v3/newrelic.NewContext"
	echoCarrier = "github.com/labstack/echo/v4.Context"
	// reqCarrier is a carrier that is a context.Context too.
	reqCarrier = "github.com/example/reqctx.Context"
)

// TestAnalyzer runs each fixture package with the settings it is written
// for. Each case builds its own analyzer with New, so the cases share no
// settings and run in parallel.
func TestAnalyzer(t *testing.T) {
	t.Parallel()
	label := map[string]bool{"spawnerlabel": true}
	tests := []struct {
		name string
		cfg  goroutinectx.Config
		// dir is the testdata directory, analysistest.TestData() when empty.
		dir  string
		pkgs []string
	}{
		// The cases that every checker shares, side by side: the goroutine,
		// errgroup, waitgroup and spawner variants of each.
		{name: "patterns", pkgs: []string{"patterns"}},
		// The goroutines that only wait and signal of patterns, reported
		// (#230).
		{name: "reportwaiters", cfg: goroutinectx.Config{ReportWaiters: true}, pkgs: []string{"reportwaiters"}},
		{name: "patternsderive", cfg: goroutinectx.Config{GoroutineDeriver: apmDeriver}, pkgs: []string{"patternsderive"}},
		// In module mode, a method of a captured value counts when its
		// context field is unexported but of the analyzed module, and not
		// when it is of another module (#191).
		{name: "receivermodule", dir: "receivermodule", pkgs: []string{"./use"}},
		{name: "goroutine", pkgs: []string{"goroutine"}},
		{name: "errgroup", pkgs: []string{"errgroup"}},
		// Closures given through variables, resolved through SSA (#82, #83),
		// and context parameters given a fresh context (#84).
		{name: "resolve", pkgs: []string{"resolve"}},
		{name: "resolvederive", cfg: goroutinectx.Config{GoroutineDeriver: apmDeriver}, pkgs: []string{"resolvederive"}},
		{name: "waitgroup", pkgs: []string{"waitgroup"}},
		// How //goroutinectx:ignore is read (#65).
		{name: "ignoreforms", pkgs: []string{"ignoreforms"}},
		{name: "conc", pkgs: []string{"conc"}},
		{name: "goroutinederive", cfg: goroutinectx.Config{GoroutineDeriver: apmDeriver}, pkgs: []string{"goroutinederive"}},
		// AND: all must be called (Transaction.NewGoroutine + NewContext).
		{name: "goroutinederiveand", cfg: goroutinectx.Config{GoroutineDeriver: newrelicDeriver}, pkgs: []string{"goroutinederiveand"}},
		// Mixed: (Transaction.NewGoroutine AND NewContext) OR
		// apm.NewGoroutineContext.
		{name: "goroutinederivemixed", cfg: goroutinectx.Config{GoroutineDeriver: newrelicDeriver + "," + apmDeriver}, pkgs: []string{"goroutinederivemixed"}},
		{name: "carrier", cfg: goroutinectx.Config{ContextCarriers: []string{echoCarrier, reqCarrier}}, pkgs: []string{"carrier"}},
		{name: "carrierderive", cfg: goroutinectx.Config{ContextCarriers: []string{echoCarrier}, GoroutineDeriver: apmDeriver}, pkgs: []string{"carrierderive"}},
		{name: "spawner", pkgs: []string{"spawner"}},
		// Spawner directives across packages (#89) and on any doc comment
		// line (#85).
		{name: "spawnerfact", pkgs: []string{"spawnerfact"}},
		// -spawnerlabel with the directive on any doc comment line (#85).
		{name: "spawnerlabeldoc", cfg: goroutinectx.Config{Checkers: label}, pkgs: []string{"spawnerlabeldoc"}},
		// Calls of generic functions and of methods of generic types (#86),
		// and generic methods (Go 1.27).
		{name: "generics", pkgs: []string{"generics"}},
		// generics for gotask, with a deriver.
		{name: "genericsderive", cfg: goroutinectx.Config{GoroutineDeriver: apmDeriver}, pkgs: []string{"genericsderive"}},
		// -spawnerlabel with generic spawners (#86).
		{name: "genericslabel", cfg: goroutinectx.Config{Checkers: label}, pkgs: []string{"genericslabel"}},
		{name: "directive", pkgs: []string{"directive"}},
		{name: "externalspawner", cfg: goroutinectx.Config{ExternalSpawner: []string{
			"github.com/example/workerpool.Pool.Submit",
			"github.com/example/workerpool.Run",
		}}, pkgs: []string{"externalspawner"}},
		{name: "spawnerlabel", cfg: goroutinectx.Config{Checkers: label}, pkgs: []string{"spawnerlabel"}},
		{name: "gotask", cfg: goroutinectx.Config{GoroutineDeriver: apmDeriver}, pkgs: []string{"gotask"}},
		// Generated files are skipped.
		{name: "filefilter", pkgs: []string{"filefilter"}},
		// //line directives do not move ignore and spawner directives.
		{name: "linedirective", pkgs: []string{"linedirective"}},
		// Generated files with //line directives are still skipped.
		{name: "linedirectivegen", pkgs: []string{"linedirectivegen"}},
		{name: "linedirectivelabel", cfg: goroutinectx.Config{Checkers: label}, pkgs: []string{"linedirectivelabel"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := analysistest.TestData()
			if tt.dir != "" {
				dir = filepath.Join(dir, tt.dir)
			}
			a, err := goroutinectx.New(tt.cfg)
			if err != nil {
				t.Fatal(err)
			}
			analysistest.Run(t, dir, a, tt.pkgs...)
		})
	}
}

// TestFlags checks that the flags of the analyzer reach its run: the
// carrierderive fixture passes only with both -context-carriers and
// -goroutine-deriver set.
func TestFlags(t *testing.T) {
	t.Parallel()
	a := goroutinectx.AnalyzerWithFlags()
	for name, value := range map[string]string{
		"context-carriers":  echoCarrier,
		"goroutine-deriver": apmDeriver,
		"external-spawner":  "github.com/example/workerpool.Run",
		"spawnerlabel":      "false",
	} {
		if err := a.Flags.Set(name, value); err != nil {
			t.Fatal(err)
		}
	}
	analysistest.Run(t, analysistest.TestData(), a, "carrierderive")
}

// TestFlagReportWaiters checks that -report-waiters reaches the run: the
// reportwaiters fixture is reported only with it set (#230).
func TestFlagReportWaiters(t *testing.T) {
	t.Parallel()
	a := goroutinectx.AnalyzerWithFlags()
	if err := a.Flags.Set("report-waiters", "true"); err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, analysistest.TestData(), a, "reportwaiters")
}

// TestFlagRejectsShortName checks that a function or a carrier with no
// import path is rejected when the flag is set (#90, #122).
func TestFlagRejectsShortName(t *testing.T) {
	t.Parallel()
	a := goroutinectx.AnalyzerWithFlags()
	for _, flag := range []string{"goroutine-deriver", "external-spawner", "context-carriers"} {
		if err := a.Flags.Set(flag, "apm.NewGoroutineContext"); err == nil {
			t.Errorf("-%s=apm.NewGoroutineContext was accepted", flag)
		}
		if got := a.Flags.Lookup(flag).Value.String(); got != "" {
			t.Errorf("-%s holds %q after a rejected value", flag, got)
		}
	}
	if err := a.Flags.Set("goroutine-deriver", "context.WithoutCancel"); err != nil {
		t.Errorf("a standard library function was rejected: %v", err)
	}
}

// TestResolveFanOutIsBounded checks that resolving a struct field copied
// from another variable on each of many branches, a few levels deep, takes
// time linear in the code, not exponential. Six levels took about 100s
// before the resolution remembered the stores it had resolved and kept each
// value once.
func TestResolveFanOutIsBounded(t *testing.T) {
	const levels, branches = 6, 31
	var src strings.Builder
	src.WriteString("package fanout\n\nimport \"context\"\n\ntype holder struct{ fn func() }\n\n")
	src.WriteString("func f(ctx context.Context, n int) {\n\tvar v0 holder\n\tv0.fn = func() {}\n\t_ = v0.fn\n")
	for i := 1; i <= levels; i++ {
		fmt.Fprintf(&src, "\tvar v%d holder\n\tswitch n {\n", i)
		for c := range branches {
			fmt.Fprintf(&src, "\tcase %d:\n\t\tv%d = v%d\n", c, i, i-1)
		}
		fmt.Fprintf(&src, "\t}\n\t_ = v%d.fn\n", i)
	}
	fmt.Fprintf(&src, "\tgo v%d.fn() // want `goroutine does not propagate context \"ctx\"`\n}\n", levels)

	dir := t.TempDir()
	pkg := filepath.Join(dir, "src", "fanout")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "fanout.go"), []byte(src.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Parallel()
	start := time.Now()
	analysistest.Run(t, dir, goroutinectx.Analyzer, "fanout")
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Errorf("analysis took %v, want well under 30s", elapsed)
	}
}

// TestFlagsPrintDefaults checks that the flags print their defaults, which
// asks each flag value for the text of its zero value.
func TestFlagsPrintDefaults(t *testing.T) {
	t.Parallel()
	a := goroutinectx.AnalyzerWithFlags()
	var buf strings.Builder
	a.Flags.SetOutput(&buf)
	a.Flags.PrintDefaults()
	for _, name := range []string{"goroutine-deriver", "external-spawner", "context-carriers"} {
		if !strings.Contains(buf.String(), "-"+name) {
			t.Errorf("defaults do not list -%s:\n%s", name, buf.String())
		}
	}
}
