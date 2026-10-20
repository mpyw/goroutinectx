// Package ssa provides SSA-based analysis utilities for goroutinectx.
//
// # Overview
//
// This package provides SSA (Static Single Assignment) analysis capabilities
// for more accurate context propagation detection than AST-only analysis.
//
// # SSA vs AST Analysis
//
// SSA analysis can detect context usage that AST analysis might miss:
//
//	func example(ctx context.Context) {
//	    c := ctx  // Variable assignment
//	    go func() {
//	        doWork(c)  // SSA can trace c back to ctx
//	    }()
//	}
//
// # Program Building
//
// Use [BuildProgram] to create an SSA program from analysis pass:
//
//	ssaProg := ssa.BuildProgram(pass)
//
// The [Program] type wraps the SSA program and the package's source functions:
//
//	type Program struct {
//	    *ssa.Program
//	    SrcFuncs []*ssa.Function
//	}
//
// # Finding Function Literals
//
// Use [Program.FindFuncLit] to get the SSA function for an AST func literal:
//
//	ssaFn := ssaProg.FindFuncLit(astFuncLit)
//	if ssaFn == nil {
//	    // Fall back to AST analysis
//	}
//
// # Closure Checks
//
// Plain functions check an SSA function for context propagation:
//
//	// Check if closure captures context
//	captures := ssa.ClosureCapturesContext(ssaFn, carriers)
//
//	// Check if closure calls deriver function
//	result := ssa.ClosureCallsDeriver(ssaFn, deriveMatcher)
//	if result.FoundAtStart {
//	    // Deriver called at goroutine start - OK
//	} else if result.FoundOnlyInDefer {
//	    // Deriver only in defer - warning
//	}
//
// # Resolving Spawned Values
//
// [Program.GoAt] and [Program.CallAt] find the instruction of an AST go
// statement or call, and [ResolvedArgs] lines its SSA arguments up with the
// AST ones. [FuncResolver] then follows a func-typed value back through Phi
// nodes, loads of local variables and calls of factories to each function
// literal it can hold, and judges each one:
//
//	r := &ssa.FuncResolver{
//	    Closure: func(fn *ssa.Function) bool { return ssa.ClosureCapturesContext(fn, carriers) },
//	}
//	ok := r.Passes(goInstr.Call.Value)
//
// A value it cannot follow passes, so that only what is resolved is reported.
//
// # Walking Calls
//
// [CallWalker] walks the Call, Defer and Go instructions of a function, going
// into IIFEs, and with Closures set into every closure. The deriver check and
// the spawnerlabel checker both walk calls with it.
//
// # Closure Free Variables
//
// SSA closures track their captured variables as "free variables":
//
//	func outer(ctx context.Context) {
//	    go func() {
//	        doWork(ctx)  // ctx is a FreeVar of this closure
//	    }()
//	}
//
// The capture check asks whether any free variable has context type:
//
//	for _, fv := range closure.FreeVars {
//	    if typeutil.IsContextType(fv.Type()) {
//	        return true  // Closure captures context
//	    }
//	}
//
// # Deriver Detection
//
// For goroutine-derive checking, ClosureCallsDeriver:
//
//  1. Collects all function calls in the closure body
//  2. Tracks whether calls are in defer statements
//  3. Checks if deriver functions are called at start vs only in defer
//
// This enables warnings like:
//
//	go func() {
//	    defer apm.NewGoroutineContext(ctx)  // Warning: should call at start
//	    doWork(ctx)
//	}()
//
// # Helper Functions
//
// The package exports helper functions for SSA analysis:
//
//   - [ExtractCalledFunc]: Get types.Func from CallCommon
//   - [CallWalker]: Walk calls, going into immediately-invoked function expressions
//   - [CallHasFuncArgs]: Check if call has function-typed arguments
package ssa
