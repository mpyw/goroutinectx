package flow

import (
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa"
)

// judgeTestPolicy judges a function literal by literal, and leaves every
// call to the built-in rules.
//
//declscope:shared // closure_test.go judges closures by it
type judgeTestPolicy struct {
	literal func(fn *ssa.Function) Verdict
}

func (p *judgeTestPolicy) Literal(fn *ssa.Function, _ Site) Verdict { return p.literal(fn) }

func (*judgeTestPolicy) Call(*ssa.CallCommon) Verdict { return Unknown }

func (*judgeTestPolicy) FactoryCall(*ssa.CallCommon) bool { return false }

func (*judgeTestPolicy) Factory(*ssa.Function) bool { return false }

// judgeCapturingPolicy passes a literal that captures a variable.
func judgeCapturingPolicy() *judgeTestPolicy {
	return &judgeTestPolicy{literal: func(fn *ssa.Function) Verdict {
		if len(fn.FreeVars) > 0 {
			return Pass
		}
		return Fail
	}}
}

// TestJudgeVerdictOrder checks how verdicts combine: of all, the worst
// counts, and of any, the best. Unknown counts as Pass, and FailDeferOnly
// ranks between Fail and Pass.
func TestJudgeVerdictOrder(t *testing.T) {
	t.Parallel()

	id := func(v Verdict) Verdict { return v }
	for _, tt := range []struct {
		name     string
		verdicts []Verdict
		all, any Verdict
	}{
		{name: "none", verdicts: nil, all: Pass, any: Fail},
		{name: "defer and pass", verdicts: []Verdict{FailDeferOnly, Pass}, all: FailDeferOnly, any: Pass},
		{name: "fail and defer", verdicts: []Verdict{Fail, FailDeferOnly}, all: Fail, any: FailDeferOnly},
		{name: "unknown and defer", verdicts: []Verdict{Unknown, FailDeferOnly}, all: FailDeferOnly, any: Unknown},
	} {
		if got := judgeAll(tt.verdicts, id); got != tt.all {
			t.Errorf("%s: judgeAll = %d, want %d", tt.name, got, tt.all)
		}
		if got := judgeAny(tt.verdicts, id); got != tt.any {
			t.Errorf("%s: judgeAny = %d, want %d", tt.name, got, tt.any)
		}
	}
	if got := judgeBetter(Fail, FailDeferOnly); got != FailDeferOnly {
		t.Errorf("judgeBetter(Fail, FailDeferOnly) = %d", got)
	}
	if !Unknown.Passes() || !Pass.Passes() || Fail.Passes() || FailDeferOnly.Passes() {
		t.Error("only Pass and Unknown pass")
	}
}

// judgeSrc gives sink a value in each function, for the factory rules, and
// captures func values in closures, for judgeCaptured.
var judgeSrc = `package p

import "sync"

func sink(func()) {}

func sinkState(state) {}

func wrap(a, b func()) func() { return nil }

func never() func() { panic("never") }

func rec() func() { return rec() }

type state func() state

func next() state { return next }

func take(*func()) {}

func mk() func() { return func() {} }

func made() { sink(mk()) }

func external() {
	bad := func() {}
	sink(sync.OnceFunc(bad))
}

func neverReturns() { sink(never()) }

func recursive() { sink(rec()) }

func judgedBefore(n int) {
	bad := func() {}
	g := func() { _ = n }
	if n > 0 {
		g = bad
	}
	sink(wrap(bad, g))
}

func deep() { sinkState(next()` + strings.Repeat("()", 18) + `) }

func escapes() {
	f := func() {}
	take(&f)
	go func() { f() }()
}

func nested() {
	f := func() {}
	go func() { go func() { f() }() }()
}
`

// judgeSinkArg gives the value fn gives sink or sinkState.
func judgeSinkArg(t *testing.T, fn *ssa.Function) ssa.Value {
	t.Helper()
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			if call, ok := instr.(*ssa.Call); ok {
				if callee := call.Common().StaticCallee(); callee != nil && strings.HasPrefix(callee.Name(), "sink") {
					return call.Common().Args[0]
				}
			}
		}
	}
	t.Fatalf("%s gives sink nothing", fn.Name())
	return nil
}

// TestJudgeFactories checks the factory rules at their edges: what cannot
// be followed passes, and so does a value judged before in the question, as
// its verdict is counted already.
func TestJudgeFactories(t *testing.T) {
	t.Parallel()

	pkg := buildClosurePackage(t, judgeSrc)
	for _, tt := range []struct {
		fn     string
		passes bool
	}{
		// A factory that returns a literal that fails fails.
		{fn: "made", passes: false},
		// A factory of another package has no body to follow.
		{fn: "external", passes: true},
		// A factory that never returns gives nothing to judge.
		{fn: "neverReturns", passes: true},
		// A factory that returns its own call adds nothing to its verdict.
		{fn: "recursive", passes: true},
		// bad fails as the first argument, and passes as a source of g.
		{fn: "judgedBefore", passes: true},
		// A result more calls away than judgeMaxCalls is not followed.
		{fn: "deep", passes: true},
	} {
		v := judgeSinkArg(t, pkg.Func(tt.fn))
		if got := Judge(NewFlow(), judgeCapturingPolicy(), v); got.Passes() != tt.passes {
			t.Errorf("%s: Judge = %d, want passes %v", tt.fn, got, tt.passes)
		}
	}
}

// TestJudgeCaptured checks that a captured variable whose address is
// passed on is Unknown, and that a free variable of an outer closure is
// read out to the function that makes it.
func TestJudgeCaptured(t *testing.T) {
	t.Parallel()

	pkg := buildClosurePackage(t, judgeSrc)
	escaped := pkg.Func("escapes").AnonFuncs[1].FreeVars[0]
	if got := judgeCaptured(NewFlow(), judgeCapturingPolicy(), escaped); got != Unknown {
		t.Errorf("a variable whose address is passed on: judgeCaptured = %d, want Unknown", got)
	}
	inner := pkg.Func("nested").AnonFuncs[1].AnonFuncs[0].FreeVars[0]
	if got := judgeCaptured(NewFlow(), judgeCapturingPolicy(), inner); got != Fail {
		t.Errorf("a variable of an outer closure: judgeCaptured = %d, want Fail", got)
	}
}
