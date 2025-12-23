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

func TestSpawner(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, goroutinectx.Analyzer, "spawner")
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
