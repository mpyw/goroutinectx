package probe

import (
	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/carrier"
	"github.com/mpyw/goroutinectx/internal/ssa"
)

// Context provides context for pattern checking.
type Context struct {
	Pass     *analysis.Pass
	SSAProg  *ssa.Program
	CtxNames []string
	Carriers []carrier.Carrier
}
