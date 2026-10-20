// Package resolve tests that a goroutine or callback given through a variable
// is resolved through SSA, and checked as the same closure written inline.
package resolve

import (
	"context"

	"golang.org/x/sync/errgroup"
)

//vt:helper
func use(ctx context.Context) { _ = ctx }

// ===== SHOULD REPORT =====

// [BAD]: A closure declared with var is checked (#82).
func badVarDecl(ctx context.Context) {
	var fn = func() {}
	go fn() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: The same, given to errgroup.
func badVarDeclErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	var fn = func() error { return nil }
	g.Go(fn) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
}

// [BAD]: The value at the go statement is checked, not a later one (#82).
func badEarlierValue(ctx context.Context) {
	fn := func() {}
	go fn() // want `goroutine does not propagate context "ctx"`
	fn = func() { use(ctx) }
	_ = fn
}

// [BAD]: A closure that uses a context of its own does not propagate ctx (#83).
func badLocalCtxViaIdent(ctx context.Context) {
	fn := func() {
		c := context.Background()
		use(c)
	}
	go fn() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: The same, given to errgroup (#83).
func badLocalCtxViaIdentErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error {
		c := context.Background()
		use(c)
		return nil
	}
	g.Go(fn) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
}

// [BAD]: A closure captured by another closure is still resolved.
func badCapturedVariable(ctx context.Context) {
	fn := func() {}
	defer func() { fn() }()
	go fn() // want `goroutine does not propagate context "ctx"`
}

// ===== SHOULD NOT REPORT =====

// [GOOD]: A later assignment does not replace the closure run (#82).
func goodLaterReassign(ctx context.Context) {
	fn := func() { use(ctx) }
	go fn()
	fn = func() {}
	_ = fn
}

// [GOOD]: The same, given to errgroup (#82).
func goodLaterReassignErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	fn := func() error { use(ctx); return nil }
	g.Go(fn)
	fn = func() error { return nil }
	_ = fn
}

// [GOOD]: ctx used only in a nested closure counts, as it does inline (#83).
func goodNestedViaIdent(ctx context.Context) {
	fn := func() {
		defer func() { use(ctx) }()
	}
	go fn()
}

// [GOOD]: Every branch assigns a closure that uses ctx.
func goodBranches(ctx context.Context, cond bool) {
	var fn func()
	if cond {
		fn = func() { use(ctx) }
	} else {
		fn = func() { use(ctx) }
	}
	go fn()
}

// ===== EDGE CASES =====

// [BAD]: One branch assigns a closure without ctx.
func badOneBranch(ctx context.Context, cond bool) {
	fn := func() { use(ctx) }
	if cond {
		fn = func() {}
	}
	go fn() // want `goroutine does not propagate context "ctx"`
}

// [GOOD]: A closure written through a captured variable is not followed.
func goodWrittenByClosure(ctx context.Context) {
	fn := func() {}
	set := func() { fn = func() { use(ctx) } }
	set()
	go fn()
}

// ===== CONTEXT PARAMETERS (#84) =====

// [BAD]: A context parameter given context.Background() does not propagate ctx.
func badFreshCtxArg(ctx context.Context) {
	go func(c context.Context) { use(c) }(context.Background()) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: The same through a variable, given context.TODO().
func badFreshCtxArgViaIdent(ctx context.Context) {
	fn := func(c context.Context) { use(c) }
	bg := context.TODO()
	go fn(bg) // want `goroutine does not propagate context "ctx"`
}

// [GOOD]: A context parameter given ctx.
func goodCtxArg(ctx context.Context) {
	go func(c context.Context) { use(c) }(ctx)
}

// [GOOD]: A context parameter given a context derived from ctx.
func goodDerivedCtxArg(ctx context.Context) {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	go func(c context.Context) { use(c) }(c)
}

// [GOOD]: A context parameter given a context the caller cannot see the source of.
func goodUnknownCtxArg(ctx context.Context, other func() context.Context) {
	go func(c context.Context) { use(c) }(other())
}
