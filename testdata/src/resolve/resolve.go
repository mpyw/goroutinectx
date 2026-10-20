// Package resolve tests that a goroutine or callback given through a variable
// is resolved through SSA, and checked as the same closure written inline.
package resolve

import (
	"context"

	"golang.org/x/sync/errgroup"
)

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

// ===== CAPTURED CONTEXTS (#119) =====

// [BAD]: A closure that captures only a fresh context.Background() does not propagate ctx.
func badCapturedFreshCtx(ctx context.Context) {
	bg := context.Background()
	go func() { use(bg) }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: The same in an errgroup callback, with context.TODO().
func badCapturedFreshCtxErrgroup(ctx context.Context) {
	todo := context.TODO()
	g := new(errgroup.Group)
	g.Go(func() error { use(todo); return nil }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: A fresh context captured through two closures.
func badCapturedFreshCtxNested(ctx context.Context) {
	bg := context.Background()
	go func() { // want `goroutine does not propagate context "ctx"`
		go func() { use(bg) }() // want `goroutine does not propagate context "ctx"`
	}()
}

// [GOOD]: A captured variable that holds ctx on one path.
func goodCapturedCtxOnOnePath(ctx context.Context, b bool) {
	c := context.Background()
	if b {
		c = ctx
	}
	go func() { use(c) }()
}

// [GOOD]: A captured context derived from ctx.
func goodCapturedDerivedCtx(ctx context.Context) {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { use(c) }()
}

// [GOOD]: A captured context whose address is taken, so its stores are not followed.
func goodCapturedCtxAddressTaken(ctx context.Context, set func(*context.Context)) {
	c := context.Background()
	set(&c)
	go func() { use(c) }()
}

// ===== NAMED FUNCTIONS GIVEN A CONTEXT (#151) =====

func worker(c context.Context) { use(c) }

func workerWith(n int, c context.Context) { _ = n; use(c) }

type service struct{ ctx context.Context }

func (s *service) run(c context.Context) { use(s.ctx); use(c) }

// [BAD]: A named function given only context.Background() does not propagate ctx.
func badNamedFuncFreshCtx(ctx context.Context) {
	go worker(context.Background()) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: The same with other arguments and context.TODO() through a variable.
func badNamedFuncFreshCtxViaIdent(ctx context.Context) {
	todo := context.TODO()
	go workerWith(1, todo) // want `goroutine does not propagate context "ctx"`
}

// [GOOD]: A named function given ctx.
func goodNamedFuncCtx(ctx context.Context) {
	go worker(ctx)
}

// [GOOD]: A named function given no context is not looked into.
func goodNamedFuncNoCtx(ctx context.Context) {
	go use(nil)
}

// [NOTCHECKED]: A method given a fresh context may get ctx from its receiver.
func notCheckedMethodFreshCtx(ctx context.Context) {
	s := &service{ctx: ctx}
	go s.run(context.Background())
}

// ===== PARTS OF VALUES THAT ARE NOT VARIABLES =====

type partHolder struct{ fn func() }

type nestedPartHolder struct{ inner partHolder }

func makePartHolders() [1]partHolder { return [1]partHolder{{fn: func() {}}} }

func makeNestedPartHolder() nestedPartHolder {
	return nestedPartHolder{inner: partHolder{fn: func() {}}}
}

func makePartHolder() partHolder { return partHolder{fn: func() {}} }

// [NOTCHECKED]: A part of a call's result, or of a value chosen on a branch, is not followed.
func notCheckedPartOfCallResult(ctx context.Context, c bool) {
	a := makePartHolders()[0]
	go a.fn()
	b := makeNestedPartHolder().inner
	go b.fn()
	var t partHolder
	if c {
		t = makePartHolder()
	} else {
		t = makePartHolder()
	}
	s := t
	go s.fn()
}

// [BAD]: A struct chosen on a branch from two variables, one holding a func without ctx.
func badPartOfValueChosenOnBranch(ctx context.Context, c bool) {
	var x, y partHolder
	x.fn = func() { use(ctx) }
	y.fn = func() {}
	var t partHolder
	if c {
		t = x
	} else {
		t = y
	}
	s := t
	go s.fn() // want `goroutine does not propagate context "ctx"`
}

// ===== BASES THAT ARE NOT FOLLOWED =====

var globalFn = func() {}

// [NOTCHECKED]: A package-level variable is not followed.
func notCheckedGlobalFunc(ctx context.Context) {
	go globalFn()
}

// [NOTCHECKED]: A field of a struct given as a parameter is not followed.
func notCheckedParamField(ctx context.Context, s *partHolder) {
	go s.fn()
}

// [NOTCHECKED]: A field through a pointer a closure writes is not followed.
func notCheckedPointerWrittenInClosure(ctx context.Context) {
	p := &partHolder{fn: func() {}}
	func() { p = &partHolder{fn: func() {}} }()
	go p.fn()
}

func takePart(h *partHolder) { _ = h }

// [NOTCHECKED]: A struct whose nested field's address is passed on is not followed.
func notCheckedNestedFieldAddressPassed(ctx context.Context) {
	s := nestedPartHolder{inner: partHolder{fn: func() {}}}
	takePart(&s.inner)
	go s.inner.fn()
}

// [GOOD]: A closure taking an error, a named type with no package, beside ctx it uses.
func goodClosureWithErrorParam(ctx context.Context) {
	go func(err error) { _ = err; use(ctx) }(nil)
}
