package goroutinectx_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/mpyw/goroutinectx"
)

func TestGoroutine(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, goroutinectx.Analyzer, "goroutine")
}

func TestErrgroup(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, goroutinectx.Analyzer, "errgroup")
}

// TestResolve checks closures given through variables, resolved through SSA
// (#82, #83), and context parameters given a fresh context (#84).
func TestResolve(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, goroutinectx.Analyzer, "resolve")
}

// TestResolveDerive is TestResolve with -goroutine-deriver set.
func TestResolveDerive(t *testing.T) {
	testdata := analysistest.TestData()

	deriveFunc := "github.com/my-example-app/telemetry/apm.NewGoroutineContext"
	if err := goroutinectx.Analyzer.Flags.Set("goroutine-deriver", deriveFunc); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = goroutinectx.Analyzer.Flags.Set("goroutine-deriver", "")
	}()

	analysistest.Run(t, testdata, goroutinectx.Analyzer, "resolvederive")
}

func TestErrgroupDerive(t *testing.T) {
	testdata := analysistest.TestData()

	deriveFunc := "github.com/my-example-app/telemetry/apm.NewGoroutineContext"
	if err := goroutinectx.Analyzer.Flags.Set("goroutine-deriver", deriveFunc); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = goroutinectx.Analyzer.Flags.Set("goroutine-deriver", "")
	}()

	analysistest.Run(t, testdata, goroutinectx.Analyzer, "errgroupderive")
}

// TestIgnoreForms checks how //goroutinectx:ignore is read (#65).
func TestIgnoreForms(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, goroutinectx.Analyzer, "ignoreforms")
}

func TestConc(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, goroutinectx.Analyzer, "conc")
}

func TestGoroutineDerive(t *testing.T) {
	testdata := analysistest.TestData()

	deriveFunc := "github.com/my-example-app/telemetry/apm.NewGoroutineContext"
	if err := goroutinectx.Analyzer.Flags.Set("goroutine-deriver", deriveFunc); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = goroutinectx.Analyzer.Flags.Set("goroutine-deriver", "")
	}()

	analysistest.Run(t, testdata, goroutinectx.Analyzer, "goroutinederive")
}

func TestGoroutineDeriveAnd(t *testing.T) {
	testdata := analysistest.TestData()
	// AND: all must be called (Transaction.NewGoroutine + NewContext)
	deriveFunc := "github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+" +
		"github.com/newrelic/go-agent/v3/newrelic.NewContext"
	if err := goroutinectx.Analyzer.Flags.Set("goroutine-deriver", deriveFunc); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = goroutinectx.Analyzer.Flags.Set("goroutine-deriver", "")
	}()

	analysistest.Run(t, testdata, goroutinectx.Analyzer, "goroutinederiveand")
}

func TestGoroutineDeriveMixed(t *testing.T) {
	testdata := analysistest.TestData()
	// Mixed: (Transaction.NewGoroutine AND NewContext) OR apm.NewGoroutineContext
	deriveFunc := "github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+" +
		"github.com/newrelic/go-agent/v3/newrelic.NewContext," +
		"github.com/my-example-app/telemetry/apm.NewGoroutineContext"
	if err := goroutinectx.Analyzer.Flags.Set("goroutine-deriver", deriveFunc); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = goroutinectx.Analyzer.Flags.Set("goroutine-deriver", "")
	}()

	analysistest.Run(t, testdata, goroutinectx.Analyzer, "goroutinederivemixed")
}

func TestContextCarriers(t *testing.T) {
	testdata := analysistest.TestData()

	carriers := "github.com/labstack/echo/v4.Context"
	if err := goroutinectx.Analyzer.Flags.Set("context-carriers", carriers); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = goroutinectx.Analyzer.Flags.Set("context-carriers", "")
	}()

	analysistest.Run(t, testdata, goroutinectx.Analyzer, "carrier")
}

func TestCarrierDerive(t *testing.T) {
	testdata := analysistest.TestData()

	carriers := "github.com/labstack/echo/v4.Context"
	if err := goroutinectx.Analyzer.Flags.Set("context-carriers", carriers); err != nil {
		t.Fatal(err)
	}

	deriveFunc := "github.com/my-example-app/telemetry/apm.NewGoroutineContext"
	if err := goroutinectx.Analyzer.Flags.Set("goroutine-deriver", deriveFunc); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = goroutinectx.Analyzer.Flags.Set("context-carriers", "")
		_ = goroutinectx.Analyzer.Flags.Set("goroutine-deriver", "")
	}()

	analysistest.Run(t, testdata, goroutinectx.Analyzer, "carrierderive")
}

func TestSpawnerDerive(t *testing.T) {
	testdata := analysistest.TestData()

	deriveFunc := "github.com/my-example-app/telemetry/apm.NewGoroutineContext"
	if err := goroutinectx.Analyzer.Flags.Set("goroutine-deriver", deriveFunc); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = goroutinectx.Analyzer.Flags.Set("goroutine-deriver", "")
	}()

	analysistest.Run(t, testdata, goroutinectx.Analyzer, "spawnerderive")
}

func TestSpawner(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, goroutinectx.Analyzer, "spawner")
}

// TestSpawnerFact checks spawner directives across packages (#89) and on
// any doc comment line (#85).
func TestSpawnerFact(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, goroutinectx.Analyzer, "spawnerfact")
}

// TestSpawnerlabelDoc checks -spawnerlabel with the directive on any doc
// comment line (#85).
func TestSpawnerlabelDoc(t *testing.T) {
	testdata := analysistest.TestData()

	if err := goroutinectx.Analyzer.Flags.Set("spawnerlabel", "true"); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = goroutinectx.Analyzer.Flags.Set("spawnerlabel", "false")
	}()

	analysistest.Run(t, testdata, goroutinectx.Analyzer, "spawnerlabeldoc")
}

// TestGenerics checks calls of generic functions and of methods of generic
// types (#86).
func TestGenerics(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, goroutinectx.Analyzer, "generics")
}

// TestGenericsDerive is TestGenerics for gotask, with -goroutine-deriver set.
func TestGenericsDerive(t *testing.T) {
	testdata := analysistest.TestData()

	deriveFunc := "github.com/my-example-app/telemetry/apm.NewGoroutineContext"
	if err := goroutinectx.Analyzer.Flags.Set("goroutine-deriver", deriveFunc); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = goroutinectx.Analyzer.Flags.Set("goroutine-deriver", "")
	}()

	analysistest.Run(t, testdata, goroutinectx.Analyzer, "genericsderive")
}

// TestGenericsLabel checks -spawnerlabel with generic spawners (#86).
func TestGenericsLabel(t *testing.T) {
	testdata := analysistest.TestData()

	if err := goroutinectx.Analyzer.Flags.Set("spawnerlabel", "true"); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = goroutinectx.Analyzer.Flags.Set("spawnerlabel", "false")
	}()

	analysistest.Run(t, testdata, goroutinectx.Analyzer, "genericslabel")
}

// TestFlagRejectsShortName checks that a function with no import path is
// rejected when the flag is set (#90).
func TestFlagRejectsShortName(t *testing.T) {
	for _, flag := range []string{"goroutine-deriver", "external-spawner"} {
		if err := goroutinectx.Analyzer.Flags.Set(flag, "apm.NewGoroutineContext"); err == nil {
			t.Errorf("-%s=apm.NewGoroutineContext was accepted", flag)
		}
		if got := goroutinectx.Analyzer.Flags.Lookup(flag).Value.String(); got != "" {
			t.Errorf("-%s holds %q after a rejected value", flag, got)
		}
	}
	if err := goroutinectx.Analyzer.Flags.Set("goroutine-deriver", "context.WithoutCancel"); err != nil {
		t.Errorf("a standard library function was rejected: %v", err)
	}
	_ = goroutinectx.Analyzer.Flags.Set("goroutine-deriver", "")
}

func TestDirective(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, goroutinectx.Analyzer, "directive")
}

func TestExternalSpawner(t *testing.T) {
	testdata := analysistest.TestData()

	// Set external spawner flag for workerpool package
	externalSpawners := "github.com/example/workerpool.Pool.Submit," +
		"github.com/example/workerpool.Run"
	if err := goroutinectx.Analyzer.Flags.Set("external-spawner", externalSpawners); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = goroutinectx.Analyzer.Flags.Set("external-spawner", "")
	}()

	analysistest.Run(t, testdata, goroutinectx.Analyzer, "externalspawner")
}

func TestSpawnerlabel(t *testing.T) {
	testdata := analysistest.TestData()

	if err := goroutinectx.Analyzer.Flags.Set("spawnerlabel", "true"); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = goroutinectx.Analyzer.Flags.Set("spawnerlabel", "false")
	}()

	analysistest.Run(t, testdata, goroutinectx.Analyzer, "spawnerlabel")
}

func TestGotask(t *testing.T) {
	testdata := analysistest.TestData()

	deriveFunc := "github.com/my-example-app/telemetry/apm.NewGoroutineContext"
	if err := goroutinectx.Analyzer.Flags.Set("goroutine-deriver", deriveFunc); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = goroutinectx.Analyzer.Flags.Set("goroutine-deriver", "")
	}()

	analysistest.Run(t, testdata, goroutinectx.Analyzer, "gotask")
}

func TestFileFilter(t *testing.T) {
	testdata := analysistest.TestData()
	// Tests that generated files are skipped
	analysistest.Run(t, testdata, goroutinectx.Analyzer, "filefilter")
}

func TestLineDirective(t *testing.T) {
	testdata := analysistest.TestData()
	// Tests that //line directives do not move ignore and spawner directives
	analysistest.Run(t, testdata, goroutinectx.Analyzer, "linedirective")
}

func TestLineDirectiveGenerated(t *testing.T) {
	testdata := analysistest.TestData()
	// Tests that generated files with //line directives are still skipped
	analysistest.Run(t, testdata, goroutinectx.Analyzer, "linedirectivegen")
}

func TestLineDirectiveSpawnerlabel(t *testing.T) {
	testdata := analysistest.TestData()

	if err := goroutinectx.Analyzer.Flags.Set("spawnerlabel", "true"); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = goroutinectx.Analyzer.Flags.Set("spawnerlabel", "false")
	}()

	analysistest.Run(t, testdata, goroutinectx.Analyzer, "linedirectivelabel")
}
