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

// Module gives the path of the analyzed package's module, or "" when the
// driver does not give it.
func (c *Context) Module() string {
	if c.Pass == nil || c.Pass.Module == nil {
		return ""
	}
	return c.Pass.Module.Path
}
