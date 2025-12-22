package patterns

import (
	"go/ast"

	"github.com/mpyw/goroutinectx/internal/context"
	"github.com/mpyw/goroutinectx/internal/directives/deriver"
)

// ArgIsDeriverCall checks that an argument IS a call to the deriver function.
// Unlike ShouldCallDeriver (which checks if a callback body CONTAINS a deriver call),
// this pattern checks if the argument expression itself IS a deriver call.
// Used by: Task.DoAsync, CancelableTask.DoAsync (ctx argument).
type ArgIsDeriverCall struct {
	Matcher *deriver.Matcher
}

func (*ArgIsDeriverCall) Name() string {
	return "ArgIsDeriverCall"
}

func (p *ArgIsDeriverCall) Check(cctx *context.CheckContext, _ *ast.CallExpr, arg ast.Expr) bool {
	if p.Matcher == nil || p.Matcher.IsEmpty() {
		return true // No deriver configured
	}

	return p.argIsDeriverCall(cctx, arg)
}

// argIsDeriverCall checks if the argument expression IS a call to the deriver.
func (p *ArgIsDeriverCall) argIsDeriverCall(cctx *context.CheckContext, expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		// Not a call expression - check if it's a variable that was assigned a deriver call
		if ident, ok := expr.(*ast.Ident); ok {
			return p.identIsDeriverCall(cctx, ident)
		}
		return false
	}

	// Check if this call IS a deriver call
	fn := cctx.FuncOf(call)
	if fn != nil && p.Matcher.MatchesFunc(fn) {
		return true
	}

	// Also check nested calls like: apm.NewGoroutineContext(ctx)
	// The call itself might wrap the deriver
	return false
}

// identIsDeriverCall checks if a variable holds a deriver call result.
func (p *ArgIsDeriverCall) identIsDeriverCall(cctx *context.CheckContext, ident *ast.Ident) bool {
	v := cctx.VarOf(ident)
	if v == nil {
		return false
	}

	// Find call expression assigned to this variable
	call := cctx.CallExprAssignedTo(v, ident.Pos())
	if call == nil {
		return false
	}

	// Check if RHS is a deriver call
	fn := cctx.FuncOf(call)
	return fn != nil && p.Matcher.MatchesFunc(fn)
}

// Message formats the error message for DoAsync-style methods.
// apiName comes from API.FullName() like "gotask.Task.DoAsync"
// We convert to pointer receiver format: "(*gotask.Task).DoAsync()"
func (p *ArgIsDeriverCall) Message(apiName string, _ string) string {
	// Convert "pkg.Type.Method" to "(*pkg.Type).Method"
	// apiName is like "gotask.Task.DoAsync"
	parts := argDeriverSplitAPIName(apiName)
	if len(parts) == 3 {
		// pkg.Type.Method -> (*pkg.Type).Method
		return "(*" + parts[0] + "." + parts[1] + ")." + parts[2] + "() 1st argument should call goroutine deriver"
	}
	return apiName + "() 1st argument should call goroutine deriver"
}

// argDeriverSplitAPIName splits an API name like "pkg.Type.Method" into parts.
func argDeriverSplitAPIName(name string) []string {
	var parts []string
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			parts = append([]string{name[i+1:]}, parts...)
			name = name[:i]
		}
	}
	if name != "" {
		parts = append([]string{name}, parts...)
	}
	return parts
}
