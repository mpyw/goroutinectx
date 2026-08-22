//go:build go1.27

package goroutinectx_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/mpyw/goroutinectx"
)

// TestGenericMethod covers Go 1.27 generic methods, whose receiver carries both a
// named type and its own type parameter list.
func TestGenericMethod(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, goroutinectx.Analyzer, "genericmethod")
}
