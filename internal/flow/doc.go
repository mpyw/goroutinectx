// Package flow follows values through the SSA form of a package for
// goroutinectx: where a spawned function comes from, what a closure
// captures, and what a function calls.
//
// # Overview
//
// It is built on golang.org/x/tools/go/ssa, which its callers import as ssa.
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
//	ssaProg := flow.BuildProgram(pass)
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
//	captures := e.Flow.ClosureCapturesContext(ssaFn, e.Kinds, nil)
//
//	// Check if closure calls deriver function
//	result := flow.ClosureCallsDeriver(ssaFn, deriveMatcher)
//	if result.FoundAtStart {
//	    // Deriver called at goroutine start - OK
//	} else if result.FoundOnlyInDefer {
//	    // Deriver only in defer - warning
//	}
//
// # Sources of a Value
//
// A [Flow] is the one engine that follows values, built once per pass.
// [Flow.Sources] gives the values a value can come from, through Phi nodes,
// type changes, loads of local variables, of their struct fields and
// constant-index elements, and map lookups at constant keys, each with its
// [SourceKind], and whether the list is complete. Every check below that
// follows a value folds over it.
//
// Sources never looks through a call, so that [Flow.IsFreshContext] does
// not. Two other questions read what it does not, each for the checks that
// need it. [Flow.CapturedSources] reads a variable a closure captures, or a
// part of a captured value, by every value stored to it, for the closure
// checks. [Flow.Returns] gives what a function returns with the sources of
// each, for the checks that follow a factory.
//
// A question to a Flow, as any of these or [FuncResolver.Passes], has one bound
// on its work, which the questions it asks in turn share, so nested checks
// do not multiply it. What a question finds is remembered for the pass,
// unless a bound cut it short.
//
// # Resolving Spawned Values
//
// [Program.GoAt] and [Program.CallAt] find the instruction of an AST go
// statement or call, and [ResolvedArgs] lines its SSA arguments up with the
// AST ones. [FuncResolver] then folds over the sources of a func-typed
// value, and over what the factories it calls return, as Returns gives it,
// to each function literal it can hold, and judges each one. Of a factory,
// one value it returns passing is enough:
//
//	r := &flow.FuncResolver{
//	    Flow:    e.Flow,
//	    Closure: func(fn *ssa.Function) bool { return e.Flow.ClosureCapturesContext(fn, e.Kinds, nil) },
//	}
//	ok := r.Passes(goInstr.Call.Value)
//
// A value it cannot follow passes, so that only what is resolved is reported.
// That includes a variable whose address is passed on, or a part written in
// a closure, since the code there may write it.
//
// # Function Kinds
//
// [Classify] tells what kind of function an SSA function is: a function
// literal, the body of a range-over-func loop, a named function, a method, a
// bound method wrapper, a thunk, a method wrapper, or a function built from
// type information. An instantiation of a generic function is classified as
// its generic origin. Every check that tells these apart asks it.
//
// # What a Function Runs
//
// [Runs] gives the Call, Defer and Go instructions a function runs, each
// with whether it is deferred and whether it is a go statement. It goes into
// IIFEs and range-over-func loop bodies, and as [RunOptions] asks, into the
// literal a go statement runs and the closures a called factory returns.
// The deriver check walks only the function and what it calls in place;
// spawnerlabel walks goroutines and returned closures too.
//
// # Returned Values
//
// One scanner reads the returns of a function, in two ways. A factory is
// followed through the first result of each return only, as [Flow.Returns]
// gives it, while spawnerlabel takes a closure returned as any result. The two are kept apart so that
// neither report changes.
//
// # Fresh Contexts
//
// [Flow.IsFreshContext] reports whether a value is only ever a
// context.Background() or TODO(). [Flow.GivesOnlyFreshContext] reports whether a
// call gives such a context and nothing else that may hold one. What counts
// as a context is up to the ctxkind package.
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
// The capture check asks whether any free variable is a context, as the
// classifier judges it:
//
//	for _, fv := range closure.FreeVars {
//	    if kinds.Kind(fv.Type())&ctxkind.Captured != 0 {
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
//   - [Classify]: Tell a function literal, a named function, a method and the synthetic wrappers apart
//   - [Runs]: Give the calls a function runs, going into immediately-invoked function expressions and range-over-func loop bodies
package flow
