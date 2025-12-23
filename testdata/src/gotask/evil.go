// Package gotask contains evil edge case tests for the gotask context derivation checker.
package gotask

import (
	"context"

	"github.com/my-example-app/telemetry/apm"
	gotask "github.com/siketyan/gotask/v2"
)

// ===== VARIADIC EXPANSION - SHOULD REPORT =====

// [BAD]: Variadic expansion without deriver
//
// Task function does not call the required context deriver.
func badVariadicExpansion(ctx context.Context) {
	tasks := []func(context.Context) error{
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { return nil },
	}
	_ = gotask.DoAllFnsSettled(ctx, tasks...) // want `gotask\.DoAllFnsSettled\(\) variadic argument should call goroutine deriver`
}

// ===== VARIABLE TASK - SHOULD REPORT =====

// [BAD]: Task stored in variable (func literal without deriver)
//
// Task function does not call the required context deriver.
func badVariableTaskNoDeriver(ctx context.Context) {
	fn := func(ctx context.Context) error {
		return nil
	}
	_ = gotask.DoAllFnsSettled(ctx, fn) // want `gotask\.DoAllFnsSettled\(\) 2nd argument should call goroutine deriver`
}

// [BAD]: NewTask stored in variable
//
// NewTask wrapper without context derivation inside.
func badNewTaskVariableNoDeriver(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		return nil
	})
	_ = gotask.DoAllSettled(ctx, task) // want `gotask\.DoAllSettled\(\) 2nd argument should call goroutine deriver`
}

// ===== NESTED CLOSURE - SHOULD REPORT (deriver in nested closure doesn't count) =====

// [BAD]: Deriver only in nested closure
//
// Deriver in nested closure is not detected by the analyzer.
func badDerivedInNestedClosure(ctx context.Context) {
	_ = gotask.DoAllFnsSettled( // want `gotask\.DoAllFnsSettled\(\) 2nd argument should call goroutine deriver`
		ctx,
		func(ctx context.Context) error {
			// Deriver is in a nested closure - won't be detected at top level
			go func() {
				_ = apm.NewGoroutineContext(ctx)
			}()
			return nil
		},
	)
}

// ===== CONDITIONAL DERIVER - SHOULD NOT REPORT =====

// [GOOD]: Deriver in if branch (any presence should satisfy)
//
// Deriver call in any branch satisfies the requirement.
func goodDerivedInIfBranch(ctx context.Context) {
	_ = gotask.DoAllFnsSettled(
		ctx,
		func(ctx context.Context) error {
			if true {
				_ = apm.NewGoroutineContext(ctx)
			}
			return nil
		},
	)
}

// ===== METHOD CHAINING - SHOULD REPORT =====

// [BAD]: Chained task creation
//
// Chained task creation - DoAsync on result of method chain
func badChainedTaskDoAsync(ctx context.Context) {
	gotask.NewTask(func(ctx context.Context) error {
		return nil
	}).DoAsync(ctx, nil) // want `\(\*gotask\.Task\)\.DoAsync\(\) 1st argument should call goroutine deriver`
}

// [BAD]: Cancelable chain DoAsync without deriver
//
// Task function does not call the required context deriver.
func badCancelableChainDoAsync(ctx context.Context) {
	gotask.NewTask(func(ctx context.Context) error {
		return nil
	}).Cancelable().DoAsync(ctx, nil) // want `\(\*gotask\.CancelableTask\)\.DoAsync\(\) 1st argument should call goroutine deriver`
}

// ===== METHOD CHAINING - SHOULD NOT REPORT =====

// [GOOD]: Chained task creation with derived ctx
//
// Chained task creation with proper context derivation.
func goodChainedTaskDoAsyncWithDeriver(ctx context.Context) {
	gotask.NewTask(func(ctx context.Context) error {
		return nil
	}).DoAsync(apm.NewGoroutineContext(ctx), nil)
}

// [GOOD]: Cancelable chain DoAsync with derived ctx
//
// DoAsync is called with a properly derived context.
func goodCancelableChainDoAsyncWithDeriver(ctx context.Context) {
	gotask.NewTask(func(ctx context.Context) error {
		return nil
	}).Cancelable().DoAsync(apm.NewGoroutineContext(ctx), nil)
}

// ===== VARIADIC EXPANSION FROM VARIABLE - LIMITATION (can't trace variable) =====

// [LIMITATION]: Variable slice expansion
//
// Function in slice element does not capture context.
func limitationVariadicExpansionVariable(ctx context.Context) {
	tasks := []func(context.Context) error{
		func(ctx context.Context) error {
			_ = apm.NewGoroutineContext(ctx)
			return nil
		},
	}
	// Reports because we can't trace into variable assignment
	_ = gotask.DoAllFnsSettled(ctx, tasks...) // want `gotask\.DoAllFnsSettled\(\) variadic argument should call goroutine deriver`
}

// ===== VARIABLE TASK - SHOULD NOT REPORT (variable tracing works) =====

// [GOOD]: Variable func assignment with deriver is traced correctly
//
// Function stored in variable captures and uses context.
func goodVariableTaskWithDeriver(ctx context.Context) {
	fn := func(ctx context.Context) error {
		_ = apm.NewGoroutineContext(ctx)
		return nil
	}
	_ = gotask.DoAllFnsSettled(ctx, fn)
}

// [GOOD]: NewTask in variable with deriver is traced correctly
//
// Task function properly calls the context deriver.
func goodNewTaskVariableWithDeriver(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		_ = apm.NewGoroutineContext(ctx)
		return nil
	})
	_ = gotask.DoAllSettled(ctx, task)
}

// ===== DERIVER IN DEFER CLOSURE - LIMITATION (defer closure is a nested FuncLit) =====

// [LIMITATION]: Defer closure not traversed
//
// Deriver call in deferred closure is not detected by analyzer.
func limitationDerivedInDeferClosure(ctx context.Context) {
	_ = gotask.DoAllFnsSettled( // want `gotask\.DoAllFnsSettled\(\) 2nd argument should call goroutine deriver`
		ctx,
		func(ctx context.Context) error {
			// The defer's func() is a FuncLit, so we don't look inside
			defer func() {
				_ = apm.NewGoroutineContext(ctx)
			}()
			return nil
		},
	)
}

// ===== MIXED DERIVER AND NON-DERIVER - SHOULD REPORT =====

// [BAD]: Multiple tasks, only some have deriver
//
// Multiple task arguments with inconsistent deriver usage.
func badMixedDerivers(ctx context.Context) {
	_ = gotask.DoAllSettled( // want `gotask\.DoAllSettled\(\) 3rd argument should call goroutine deriver`
		ctx,
		gotask.NewTask(func(ctx context.Context) error {
			_ = apm.NewGoroutineContext(ctx)
			return nil
		}),
		gotask.NewTask(func(ctx context.Context) error {
			return nil // No deriver!
		}),
	)
}

// ===== DOASYNC ON POINTER - SHOULD REPORT =====

// [BAD]: Task pointer DoAsync with deriver
//
// Task pointer DoAsync
func badTaskPointerDoAsync(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		return nil
	})
	taskPtr := &task
	taskPtr.DoAsync(ctx, nil) // want `\(\*gotask\.Task\)\.DoAsync\(\) 1st argument should call goroutine deriver`
}

// ===== DOASYNC ON POINTER - SHOULD NOT REPORT =====

// [GOOD]: Task pointer DoAsync with deriver
//
// Task function properly calls the context deriver.
func goodTaskPointerDoAsyncWithDeriver(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		return nil
	})
	taskPtr := &task
	taskPtr.DoAsync(apm.NewGoroutineContext(ctx), nil)
}

// ===== HIGHER-ORDER FUNCTIONS - SHOULD REPORT/NOT REPORT =====

// [BAD]: Higher-order function returning task WITH deriver
//
// Higher-order function returning task WITHOUT deriver - should report
func badHigherOrderTaskFactoryNoDeriver(ctx context.Context) {
	makeTask := func() gotask.Task[error] {
		return gotask.NewTask(func(ctx context.Context) error {
			return nil // No deriver
		})
	}
	_ = gotask.DoAllSettled(ctx, makeTask()) // want `gotask\.DoAllSettled\(\) 2nd argument should call goroutine deriver`
}

// [GOOD]: Higher-order function returning task WITH deriver
//
// Higher-order function returning task WITH deriver - should NOT report
func goodHigherOrderTaskFactoryWithDeriver(ctx context.Context) {
	makeTask := func() gotask.Task[error] {
		return gotask.NewTask(func(ctx context.Context) error {
			_ = apm.NewGoroutineContext(ctx)
			return nil
		})
	}
	_ = gotask.DoAllSettled(ctx, makeTask())
}

// ===== INTERFACE - LIMITATION (reports because can't trace) =====

type taskMaker interface {
	MakeTask() gotask.Task[error]
}

// [LIMITATION]: Interface method returns
//
// Context captured but lost through interface type assertion.
func limitationInterfaceTaskMaker(ctx context.Context, maker taskMaker) {
	// Reports because maker.MakeTask() can't be traced
	_ = gotask.DoAllSettled(ctx, maker.MakeTask()) // want `gotask\.DoAllSettled\(\) 2nd argument should call goroutine deriver`
}

// ===== EDGE CASES - SHOULD NOT REPORT (not gotask or edge behavior) =====

// [GOOD]: Edge case: Empty call (less than 2 args)
//
// Function call with insufficient arguments is not checked.
func goodEmptyDoAll(ctx context.Context) {
	_ = gotask.DoAll[int](ctx)
}

// [GOOD]: Edge case: Only ctx arg
//
// Function with only context argument is handled correctly.
func goodOnlyCtxArg(ctx context.Context) {
	// This would be invalid Go code if DoAll required args, but tests analyzer edge
}

// [BAD]: Edge case: Multiple DoAsync calls
//
// DoAsync is called without deriving the context first.
func badMultipleDoAsync(ctx context.Context) {
	task1 := gotask.NewTask(func(ctx context.Context) error { return nil })
	task2 := gotask.NewTask(func(ctx context.Context) error { return nil })

	task1.DoAsync(ctx, nil)                          // want `\(\*gotask\.Task\)\.DoAsync\(\) 1st argument should call goroutine deriver`
	task2.DoAsync(apm.NewGoroutineContext(ctx), nil) // OK - has deriver
	task1.DoAsync(ctx, nil)                          // want `\(\*gotask\.Task\)\.DoAsync\(\) 1st argument should call goroutine deriver`
}

// [BAD]: Edge case: Context with different param name
//
// Context parameter with non-standard naming is detected.
func badDifferentCtxName(c context.Context) {
	_ = gotask.DoAllFnsSettled( // want `gotask\.DoAllFnsSettled\(\) 2nd argument should call goroutine deriver`
		c,
		func(ctx context.Context) error {
			return nil
		},
	)
}

// [BAD]: Edge case: Context param with unusual name
//
// Context parameter with non-standard naming is detected.
func badContextParamUnusualName(myCtx context.Context) {
	_ = gotask.DoAllFnsSettled( // want `gotask\.DoAllFnsSettled\(\) 2nd argument should call goroutine deriver`
		myCtx,
		func(ctx context.Context) error {
			return nil
		},
	)
}

// [GOOD]: Edge case: Good with different ctx param names
//
// Non-standard context parameter names are properly detected.
func goodDifferentCtxNames(c context.Context) {
	_ = gotask.DoAllFnsSettled(
		c,
		func(ctx context.Context) error {
			_ = apm.NewGoroutineContext(ctx)
			return nil
		},
	)
}

// ===== RECURSIVE TASKS - SHOULD REPORT =====

// [BAD]: Edge case: Nested task creation
//
// Nested task creation pattern requires careful tracing.
func badNestedTaskCreation(ctx context.Context) {
	_ = gotask.DoAllFnsSettled( // want `gotask\.DoAllFnsSettled\(\) 2nd argument should call goroutine deriver`
		ctx,
		func(ctx context.Context) error {
			// This outer task doesn't call deriver
			_ = gotask.DoAllFnsSettled(
				ctx,
				func(ctx context.Context) error {
					_ = apm.NewGoroutineContext(ctx) // Inner has deriver but outer doesn't
					return nil
				},
			)
			return nil
		},
	)
}

// ===== DERIVER CALL IN EXPRESSION CONTEXT =====

// [GOOD]: Deriver result used directly in expression
//
// Deriver result used inline without storing in variable.
func goodDerivedUsedInExpression(ctx context.Context) {
	_ = gotask.DoAllFnsSettled(
		ctx,
		func(ctx context.Context) error {
			// Deriver called in expression context
			doSomethingWithContext(apm.NewGoroutineContext(ctx))
			return nil
		},
	)
}

//vt:helper
func doSomethingWithContext(_ context.Context) {}

// [GOOD]: Deriver result stored and used
//
// Deriver result stored in variable and used.
func goodDerivedStoredAndUsed(ctx context.Context) {
	_ = gotask.DoAllFnsSettled(
		ctx,
		func(ctx context.Context) error {
			derivedCtx := apm.NewGoroutineContext(ctx)
			doSomethingWithContext(derivedCtx)
			return nil
		},
	)
}

// ===== EARLY RETURN PATHS =====

// [GOOD]: Deriver called before early return
//
// Deriver is called before function returns early.
func goodDerivedBeforeEarlyReturn(ctx context.Context) {
	_ = gotask.DoAllFnsSettled(
		ctx,
		func(ctx context.Context) error {
			_ = apm.NewGoroutineContext(ctx)
			if true {
				return nil // Early return
			}
			return nil
		},
	)
}

// [GOOD]: Deriver only on one branch (but detected)
//
// Deriver on single branch detected as valid.
func goodDerivedOnOneBranch(ctx context.Context) {
	_ = gotask.DoAllFnsSettled(
		ctx,
		func(ctx context.Context) error {
			if false {
				_ = apm.NewGoroutineContext(ctx) // Only called conditionally but detected
			}
			return nil
		},
	)
}

// ===== POINTER DEREFERENCE PATTERNS =====

// [BAD]: Pointer dereference DoAsync without deriver
//
// Task pointer dereference pattern.
func badPointerDereferenceDoAsync(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		return nil
	})
	taskPtr := &task
	(*taskPtr).DoAsync(ctx, nil) // want `\(\*gotask\.Task\)\.DoAsync\(\) 1st argument should call goroutine deriver`
}

// [GOOD]: Pointer dereference DoAsync without deriver
//
// Task pointer dereference pattern with deriver.
func goodPointerDereferenceDoAsync(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		return nil
	})
	taskPtr := &task
	(*taskPtr).DoAsync(apm.NewGoroutineContext(ctx), nil)
}
