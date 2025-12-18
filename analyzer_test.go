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

func TestGoroutineCreator(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, goroutinectx.Analyzer, "goroutinecreator")
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
