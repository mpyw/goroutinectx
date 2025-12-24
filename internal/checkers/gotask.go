package checkers

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/directive/ignore"
	"github.com/mpyw/goroutinectx/internal/funcspec"
	"github.com/mpyw/goroutinectx/internal/probe"
)

// gotaskConstructor defines how gotask tasks are created.
var gotaskConstructor = &taskConstructorConfig{
	PkgPath:        "github.com/siketyan/gotask",
	FuncName:       "NewTask",
	CallbackArgIdx: 0,
}

// taskConstructorConfig defines how tasks are created for task-based APIs.
type taskConstructorConfig struct {
	PkgPath        string
	FuncName       string
	CallbackArgIdx int
}

// GotaskChecker checks gotask library API calls.
type GotaskChecker struct {
	derivers *deriver.Matcher
	entries  []gotaskEntry
}

// gotaskEntry defines a gotask API to check.
type gotaskEntry struct {
	Spec           funcspec.Spec
	CallbackArgIdx int
	Variadic       bool
	IsDoAsync      bool
}

// NewGotaskChecker creates a gotask checker.
func NewGotaskChecker(derivers *deriver.Matcher) *GotaskChecker {
	if derivers == nil {
		return nil
	}

	return &GotaskChecker{
		derivers: derivers,
		entries: []gotaskEntry{
			// DoAll variants
			{Spec: funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoAll"}, CallbackArgIdx: 1, Variadic: true},
			{Spec: funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoAllSettled"}, CallbackArgIdx: 1, Variadic: true},
			{Spec: funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoRace"}, CallbackArgIdx: 1, Variadic: true},
			// DoAllFns variants
			{Spec: funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoAllFns"}, CallbackArgIdx: 1, Variadic: true},
			{Spec: funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoAllFnsSettled"}, CallbackArgIdx: 1, Variadic: true},
			{Spec: funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoRaceFns"}, CallbackArgIdx: 1, Variadic: true},
			// DoAsync
			{Spec: funcspec.Spec{PkgPath: "github.com/siketyan/gotask", TypeName: "Task", FuncName: "DoAsync"}, CallbackArgIdx: 0, IsDoAsync: true},
			{Spec: funcspec.Spec{PkgPath: "github.com/siketyan/gotask", TypeName: "CancelableTask", FuncName: "DoAsync"}, CallbackArgIdx: 0, IsDoAsync: true},
		},
	}
}

// Name returns the checker name.
func (*GotaskChecker) Name() ignore.CheckerName {
	return ignore.Gotask
}

// MatchCall returns true if this checker should handle the call.
func (c *GotaskChecker) MatchCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	if c.derivers == nil || c.derivers.IsEmpty() {
		return false
	}

	fn := funcspec.ExtractFunc(pass, call)
	if fn == nil {
		return false
	}

	for _, entry := range c.entries {
		if entry.Spec.Matches(fn) {
			return true
		}
	}
	return false
}

// CheckCall checks the call expression.
// Note: This checker may report multiple diagnostics directly to pass.
func (c *GotaskChecker) CheckCall(cctx *probe.Context, call *ast.CallExpr) *internal.Result {
	fn := funcspec.ExtractFunc(cctx.Pass, call)
	if fn == nil {
		return internal.OK()
	}

	for _, entry := range c.entries {
		if !entry.Spec.Matches(fn) {
			continue
		}

		if entry.IsDoAsync {
			return c.checkDoAsync(cctx, call, entry)
		}

		// For variadic APIs, we report each failing argument separately
		c.checkVariadic(cctx, call, entry)
		return internal.OK() // We handle reporting ourselves
	}

	return internal.OK()
}

func (c *GotaskChecker) checkDoAsync(cctx *probe.Context, call *ast.CallExpr, entry gotaskEntry) *internal.Result {
	if len(call.Args) == 0 {
		return internal.OK()
	}

	ctxArg := call.Args[0]

	// Check 1: Is the ctx argument (first arg) a deriver call?
	if c.argIsDeriverCall(cctx, ctxArg) {
		return internal.OK()
	}

	// Check 2: Does the task's callback call the deriver?
	if c.taskCallbackCallsDeriver(cctx, call) {
		return internal.OK()
	}

	// Neither condition satisfied - report error with pointer receiver format
	msg := formatMethodMessage(entry.Spec.FullName())
	return internal.Fail(msg)
}

// formatMethodMessage formats a method name with pointer receiver.
// Input: "gotask.Task.DoAsync"
// Output: "gotask.(*Task).DoAsync() 1st argument should call goroutine deriver"
func formatMethodMessage(apiName string) string {
	parts := splitAPIName(apiName)
	if len(parts) == 3 {
		return parts[0] + ".(*" + parts[1] + ")." + parts[2] + "() 1st argument should call goroutine deriver"
	}
	return apiName + "() 1st argument should call goroutine deriver"
}

// splitAPIName splits an API name like "pkg.Type.Method" into parts.
func splitAPIName(name string) []string {
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

func (c *GotaskChecker) checkVariadic(cctx *probe.Context, call *ast.CallExpr, entry gotaskEntry) {
	startIdx := entry.CallbackArgIdx
	if startIdx >= len(call.Args) {
		return
	}

	// Check if this is a variadic expansion (e.g., DoAllFns(ctx, slice...))
	isVariadicExpansion := call.Ellipsis.IsValid()

	for i := startIdx; i < len(call.Args); i++ {
		if !c.argCallsDeriver(cctx, call.Args[i], entry) {
			var msg string
			if isVariadicExpansion {
				msg = fmt.Sprintf("%s() variadic argument should call goroutine deriver", entry.Spec.FullName())
			} else {
				// Report each failing argument with 1-based position
				argNum := i + 1
				msg = fmt.Sprintf("%s() %s argument should call goroutine deriver",
					entry.Spec.FullName(), ordinal(argNum))
			}
			cctx.Pass.Reportf(call.Pos(), "%s", msg)
		}
	}
}

// argIsDeriverCall checks if the argument expression IS a call to the deriver.
func (c *GotaskChecker) argIsDeriverCall(cctx *probe.Context, expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		// Not a call expression - check if it's a variable assigned a deriver call
		if ident, ok := expr.(*ast.Ident); ok {
			return c.identIsDeriverCall(cctx, ident)
		}
		return false
	}

	// Check if this call IS a deriver call
	fn := funcspec.ExtractFunc(cctx.Pass, call)
	return fn != nil && c.derivers.MatchesFunc(fn)
}

// identIsDeriverCall checks if a variable holds a deriver call result.
func (c *GotaskChecker) identIsDeriverCall(cctx *probe.Context, ident *ast.Ident) bool {
	call := cctx.CallExprAssignedToIdent(ident)
	if call == nil {
		return false
	}

	fn := funcspec.ExtractFunc(cctx.Pass, call)
	return fn != nil && c.derivers.MatchesFunc(fn)
}

// taskCallbackCallsDeriver checks if the task's callback (from constructor) calls the deriver.
func (c *GotaskChecker) taskCallbackCallsDeriver(cctx *probe.Context, call *ast.CallExpr) bool {
	// Task is always the method receiver (e.g., task.DoAsync)
	taskExpr := getMethodReceiver(call)
	if taskExpr == nil {
		return false
	}

	// Find the constructor call that created this task
	constructorCall := c.findConstructorCall(cctx, taskExpr)
	if constructorCall == nil {
		return false
	}

	// Check if constructor's callback argument calls the deriver
	argIdx := gotaskConstructor.CallbackArgIdx
	if argIdx < 0 || argIdx >= len(constructorCall.Args) {
		return false
	}

	callbackArg := constructorCall.Args[argIdx]
	return c.callbackCallsDeriver(cctx, callbackArg)
}

// getMethodReceiver extracts the receiver from a method call.
func getMethodReceiver(call *ast.CallExpr) ast.Expr {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	return sel.X
}

// findConstructorCall traces the receiver back to find the constructor call.
func (c *GotaskChecker) findConstructorCall(cctx *probe.Context, receiver ast.Expr) *ast.CallExpr {
	switch r := receiver.(type) {
	case *ast.CallExpr:
		return c.findConstructorFromCall(cctx, r)

	case *ast.Ident:
		return c.findConstructorFromIdent(cctx, r)

	case *ast.UnaryExpr:
		if r.Op.String() == "*" {
			return c.findConstructorCall(cctx, r.X)
		}

	case *ast.StarExpr:
		return c.findConstructorCall(cctx, r.X)

	case *ast.ParenExpr:
		return c.findConstructorCall(cctx, r.X)
	}

	return nil
}

// findConstructorFromCall handles call expressions in the receiver chain.
func (c *GotaskChecker) findConstructorFromCall(cctx *probe.Context, call *ast.CallExpr) *ast.CallExpr {
	if c.isTaskConstructorCall(cctx, call) {
		return call
	}

	// Check if it's a method call on a Task (e.g., Cancelable())
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	// Recurse into the receiver
	return c.findConstructorCall(cctx, sel.X)
}

// findConstructorFromIdent traces a variable back to its constructor assignment.
func (c *GotaskChecker) findConstructorFromIdent(cctx *probe.Context, ident *ast.Ident) *ast.CallExpr {
	call := cctx.CallExprAssignedToIdent(ident)
	if call == nil {
		return nil
	}

	return c.findConstructorFromCall(cctx, call)
}

// isTaskConstructorCall checks if call matches the gotask task constructor.
func (c *GotaskChecker) isTaskConstructorCall(cctx *probe.Context, call *ast.CallExpr) bool {
	fn := funcspec.ExtractFunc(cctx.Pass, call)
	if fn == nil {
		return false
	}

	spec := funcspec.Spec{
		PkgPath:  gotaskConstructor.PkgPath,
		FuncName: gotaskConstructor.FuncName,
	}
	return spec.Matches(fn)
}

// callbackCallsDeriver checks if a callback expression calls the deriver.
func (c *GotaskChecker) callbackCallsDeriver(cctx *probe.Context, arg ast.Expr) bool {
	// For function literals, check if body calls deriver
	if lit, ok := arg.(*ast.FuncLit); ok {
		// Try SSA-based check first
		if cctx.SSAProg != nil && cctx.Tracer != nil {
			ssaFn := cctx.SSAProg.FindFuncLit(lit)
			if ssaFn != nil {
				result := cctx.Tracer.ClosureCallsDeriver(ssaFn, c.derivers)
				return result.FoundAtStart
			}
		}
		// Fall back to AST-based check
		return c.derivers.SatisfiesAnyGroup(cctx.Pass, lit.Body)
	}

	// For identifiers, try to trace to FuncLit
	if ident, ok := arg.(*ast.Ident); ok {
		funcLit := cctx.FuncLitOfIdent(ident)
		if funcLit != nil {
			return c.derivers.SatisfiesAnyGroup(cctx.Pass, funcLit.Body)
		}
	}

	// For other expressions, check if deriver is called in the expression
	return c.derivers.SatisfiesAnyGroup(cctx.Pass, arg)
}

// argCallsDeriver checks if a variadic argument calls deriver.
func (c *GotaskChecker) argCallsDeriver(cctx *probe.Context, arg ast.Expr, entry gotaskEntry) bool {
	// For function literals, check if body calls deriver
	if lit, ok := arg.(*ast.FuncLit); ok {
		// Try SSA-based check first
		if cctx.SSAProg != nil && cctx.Tracer != nil {
			ssaFn := cctx.SSAProg.FindFuncLit(lit)
			if ssaFn != nil {
				result := cctx.Tracer.ClosureCallsDeriver(ssaFn, c.derivers)
				return result.FoundAtStart
			}
		}
		// Fall back to AST-based check
		return c.derivers.SatisfiesAnyGroup(cctx.Pass, lit.Body)
	}

	// For call expressions
	if call, ok := arg.(*ast.CallExpr); ok {
		return c.checkCallExpr(cctx, call)
	}

	// For identifiers, try to trace to the value
	if ident, ok := arg.(*ast.Ident); ok {
		return c.checkIdent(cctx, ident)
	}

	// Can't trace - assume OK to avoid false positives
	return true
}

// checkIdent checks if a variable contains a deriver by tracing its assignment.
func (c *GotaskChecker) checkIdent(cctx *probe.Context, ident *ast.Ident) bool {
	v := cctx.VarOf(ident)
	if v == nil {
		return true // Can't trace (not a variable)
	}

	// Check if this is a slice type - we can't trace slice contents
	if _, isSlice := v.Type().Underlying().(*types.Slice); isSlice {
		return false // Can't trace slice contents, report error
	}

	// Try to find FuncLit assignment
	funcLit := cctx.FuncLitOfIdent(ident)
	if funcLit != nil {
		return c.derivers.SatisfiesAnyGroup(cctx.Pass, funcLit.Body)
	}

	// Try to find CallExpr assignment (e.g., task := NewTask(fn))
	callExpr := cctx.CallExprAssignedToIdent(ident)
	if callExpr != nil {
		return c.checkCallExpr(cctx, callExpr)
	}

	// Can't trace
	return true
}

// checkCallExpr checks if a call expression contains a deriver.
func (c *GotaskChecker) checkCallExpr(cctx *probe.Context, call *ast.CallExpr) bool {
	// Case 1: Task constructor (e.g., NewTask(fn)) - check fn
	if c.isTaskConstructorCall(cctx, call) {
		argIdx := gotaskConstructor.CallbackArgIdx
		if argIdx >= 0 && argIdx < len(call.Args) {
			return c.callbackCallsDeriver(cctx, call.Args[argIdx])
		}
	}

	// Case 2: Factory function - trace return statements
	if c.factoryReturnCallsDeriver(cctx, call) {
		return true
	}

	// Case 3: Higher-order callback returns deriver-calling func
	if c.callbackReturnCallsDeriver(cctx, call) {
		return true
	}

	// Case 4: Check if the call itself calls deriver
	return c.derivers.SatisfiesAnyGroup(cctx.Pass, call)
}

// factoryReturnCallsDeriver traces a factory call to its FuncLit and checks returns.
func (c *GotaskChecker) factoryReturnCallsDeriver(cctx *probe.Context, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}

	funcLit := cctx.FuncLitOfIdent(ident)
	if funcLit == nil {
		return false
	}

	return c.funcLitReturnCallsDeriver(cctx, funcLit)
}

// callbackReturnCallsDeriver checks if any FuncLit argument returns a deriver-calling func.
func (c *GotaskChecker) callbackReturnCallsDeriver(cctx *probe.Context, call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		funcLit, ok := arg.(*ast.FuncLit)
		if !ok {
			continue
		}
		if c.funcLitReturnCallsDeriver(cctx, funcLit) {
			return true
		}
	}
	return false
}

// funcLitReturnCallsDeriver checks if any return statement returns a deriver-calling expr.
func (c *GotaskChecker) funcLitReturnCallsDeriver(cctx *probe.Context, funcLit *ast.FuncLit) bool {
	var found bool

	ast.Inspect(funcLit.Body, func(n ast.Node) bool {
		if found {
			return false
		}
		// Skip nested func literals
		if fl, ok := n.(*ast.FuncLit); ok && fl != funcLit {
			return false
		}

		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		for _, result := range ret.Results {
			switch r := result.(type) {
			case *ast.CallExpr:
				if c.checkCallExpr(cctx, r) {
					found = true
					return false
				}
			case *ast.FuncLit:
				if c.derivers.SatisfiesAnyGroup(cctx.Pass, r.Body) {
					found = true
					return false
				}
			}
		}
		return true
	})

	return found
}

// ordinal converts a number to its ordinal string (1st, 2nd, 3rd, etc.).
func ordinal(n int) string {
	suffix := "th"
	switch n % 10 {
	case 1:
		if n%100 != 11 {
			suffix = "st"
		}
	case 2:
		if n%100 != 12 {
			suffix = "nd"
		}
	case 3:
		if n%100 != 13 {
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", n, suffix)
}
