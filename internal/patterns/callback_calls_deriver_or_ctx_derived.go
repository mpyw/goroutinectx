package patterns

import (
	"go/ast"

	"github.com/mpyw/goroutinectx/internal/context"
	"github.com/mpyw/goroutinectx/internal/directives/ignore"
)

// CallbackCallsDeriverOrCtxDerived checks that EITHER:
// - The ctx argument IS a deriver call (ctx is derived), OR
// - The receiver's callback (passed to a task constructor like NewTask) calls the deriver
//
// This is a TaskSourcePattern - the task is always the method receiver.
//
// Example (ctx is derived):
//
//	task := lib.NewTask(func(ctx context.Context) error { return nil })
//	task.DoAsync(apm.NewGoroutineContext(ctx), nil) // OK - ctx is derived
//
// Example (callback calls deriver):
//
//	task := lib.NewTask(func(ctx context.Context) error {
//	    _ = apm.NewGoroutineContext(ctx)  // Deriver called here
//	    return nil
//	})
//	task.DoAsync(ctx, nil) // OK - callback already calls deriver
//
// Example (neither):
//
//	task := lib.NewTask(func(ctx context.Context) error { return nil })
//	task.DoAsync(ctx, nil) // BAD - neither ctx is derived nor callback calls deriver
type CallbackCallsDeriverOrCtxDerived struct {
	// CallbackCallsDeriver is embedded for callback checking logic.
	// After tracing the task back to its constructor, we delegate to this
	// for checking whether the callback calls the deriver.
	CallbackCallsDeriver
}

func (*CallbackCallsDeriverOrCtxDerived) Name() string {
	return "CallbackCallsDeriverOrCtxDerived"
}

func (*CallbackCallsDeriverOrCtxDerived) CheckerName() ignore.CheckerName {
	return ignore.Gotask
}

func (p *CallbackCallsDeriverOrCtxDerived) Check(tcctx *TaskCheckContext, call *ast.CallExpr) bool {
	if p.Matcher == nil || p.Matcher.IsEmpty() {
		return true // No deriver configured
	}

	// Check 1: Is the ctx argument (first arg) a deriver call?
	if len(call.Args) > 0 {
		if p.argIsDeriverCall(tcctx.CheckContext, call.Args[0]) {
			return true
		}
	}

	// Check 2: Does the task's callback call the deriver?
	if tcctx.Constructor == nil {
		return false // No constructor info, can't trace
	}
	return p.taskCallbackCallsDeriver(tcctx, call)
}

// argIsDeriverCall checks if the argument expression IS a call to the deriver.
func (p *CallbackCallsDeriverOrCtxDerived) argIsDeriverCall(cctx *context.CheckContext, expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		// Not a call expression - check if it's a variable assigned a deriver call
		if ident, ok := expr.(*ast.Ident); ok {
			return p.identIsDeriverCall(cctx, ident)
		}
		return false
	}

	// Check if this call IS a deriver call
	fn := cctx.FuncOf(call)
	return fn != nil && p.Matcher.MatchesFunc(fn)
}

// identIsDeriverCall checks if a variable holds a deriver call result.
func (p *CallbackCallsDeriverOrCtxDerived) identIsDeriverCall(cctx *context.CheckContext, ident *ast.Ident) bool {
	v := cctx.VarOf(ident)
	if v == nil {
		return false
	}

	call := cctx.CallExprAssignedTo(v, ident.Pos())
	if call == nil {
		return false
	}

	fn := cctx.FuncOf(call)
	return fn != nil && p.Matcher.MatchesFunc(fn)
}

// taskCallbackCallsDeriver checks if the task's callback (from constructor) calls the deriver.
// Task is always the method receiver.
func (p *CallbackCallsDeriverOrCtxDerived) taskCallbackCallsDeriver(tcctx *TaskCheckContext, call *ast.CallExpr) bool {
	// Task is always the method receiver (e.g., task.DoAsync)
	taskExpr := getMethodReceiver(call)
	if taskExpr == nil {
		return false
	}

	// Find the constructor call that created this task
	constructorCall := p.findConstructorCall(tcctx.CheckContext, taskExpr, tcctx.Constructor)
	if constructorCall == nil {
		return false
	}

	// Check if constructor's callback argument calls the deriver
	argIdx := tcctx.Constructor.CallbackArgIdx
	if argIdx < 0 || argIdx >= len(constructorCall.Args) {
		return false
	}

	callbackArg := constructorCall.Args[argIdx]
	// Delegate to embedded CallbackCallsDeriver for callback checking.
	// Pass nil as constructor since we've already traced to the callback arg.
	return p.CallbackCallsDeriver.Check(tcctx.CheckContext, callbackArg, nil)
}

// getMethodReceiver extracts the receiver from a method call.
// For task.DoAsync(...), returns the task expression.
func getMethodReceiver(call *ast.CallExpr) ast.Expr {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	return sel.X
}

// findConstructorCall traces the receiver back to find the constructor call.
// Handles:
// - lib.NewTask(fn) - direct call
// - lib.NewTask(fn).Cancelable() - chained call
// - task (variable) - traces to assignment
// - taskPtr (pointer) - traces through address-of
func (p *CallbackCallsDeriverOrCtxDerived) findConstructorCall(cctx *context.CheckContext, receiver ast.Expr, tc *TaskConstructorConfig) *ast.CallExpr {
	switch r := receiver.(type) {
	case *ast.CallExpr:
		// Could be NewTask(...) or NewTask(...).Cancelable()
		return p.findConstructorFromCall(cctx, r, tc)

	case *ast.Ident:
		// Variable: task.DoAsync(...)
		return p.findConstructorFromIdent(cctx, r, tc)

	case *ast.UnaryExpr:
		// Pointer dereference: (*taskPtr).DoAsync(...)
		if r.Op.String() == "*" {
			return p.findConstructorCall(cctx, r.X, tc)
		}

	case *ast.StarExpr:
		// Type assertion or pointer type - try inner expression
		return p.findConstructorCall(cctx, r.X, tc)
	}

	return nil
}

// findConstructorFromCall handles call expressions in the receiver chain.
// - If it's the constructor (e.g., NewTask) → return it
// - If it's a method call (e.g., Cancelable()) → recurse into its receiver
func (p *CallbackCallsDeriverOrCtxDerived) findConstructorFromCall(cctx *context.CheckContext, call *ast.CallExpr, tc *TaskConstructorConfig) *ast.CallExpr {
	if isTaskConstructorCall(cctx, call, tc) {
		return call
	}

	// Check if it's a method call on a Task (e.g., Cancelable())
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	// Recurse into the receiver
	return p.findConstructorCall(cctx, sel.X, tc)
}

// findConstructorFromIdent traces a variable back to its constructor assignment.
func (p *CallbackCallsDeriverOrCtxDerived) findConstructorFromIdent(cctx *context.CheckContext, ident *ast.Ident, tc *TaskConstructorConfig) *ast.CallExpr {
	v := cctx.VarOf(ident)
	if v == nil {
		return nil
	}

	// Find call expression assigned to this variable
	call := cctx.CallExprAssignedTo(v, ident.Pos())
	if call == nil {
		return nil
	}

	return p.findConstructorFromCall(cctx, call, tc)
}

// Message formats the error message.
func (*CallbackCallsDeriverOrCtxDerived) Message(apiName string, _ string) string {
	parts := splitAPIName(apiName)
	if len(parts) == 3 {
		return "(*" + parts[0] + "." + parts[1] + ")." + parts[2] + "() 1st argument should call goroutine deriver"
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
