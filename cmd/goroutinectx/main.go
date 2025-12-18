// Command goroutinectx is a linter that checks goroutine context propagation.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/mpyw/goroutinectx"
)

func main() {
	singlechecker.Main(goroutinectx.Analyzer)
}
