package flow

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// Judge judges every function v can hold by p, v being the function a
// goroutine or a callback runs. It follows the value back to its sources
// with f, and through calls of factories, to each function literal, and
// asks p about it at [SiteSpawned], in the order [Policy] tells. The
// verdict is the worst of the sources, and of a factory, the best of the
// values it returns. A value it cannot follow is Unknown, which passes, so
// that only what is resolved is reported.
func Judge(f *Flow, p Policy, v ssa.Value) Verdict {
	defer f.ask()()
	return newJudgement(f, p, SiteSpawned).value(v)
}

// judgeCaptured judges every function v can hold, as [Judge] does, v being
// read from a variable a closure captures, as [Flow.CapturedSources] reads
// it, and each function literal at [SiteCaptured]. A free variable captured
// by value from an outer closure is read the same way, out to the function
// that makes it. A value that cannot be followed is Unknown.
//
//declscope:shared // closure.go judges the func values a closure runs
func judgeCaptured(f *Flow, p Policy, v ssa.Value) Verdict {
	defer f.ask()()
	j := newJudgement(f, p, SiteCaptured)
	read := make(map[ssa.Value]bool)
	var captured func(v ssa.Value) Verdict
	captured = func(v ssa.Value) Verdict {
		if read[v] {
			return Pass
		}
		read[v] = true
		sources, complete := f.CapturedSources(v)
		if !complete {
			return Unknown
		}
		return judgeAll(sources, func(s Source) Verdict {
			if fv, ok := s.Value.(*ssa.FreeVar); ok && fv != v {
				return captured(fv)
			}
			return j.value(s.Value)
		})
	}
	return captured(v)
}

// JudgeResult judges the function calling v gives, as [Judge] does, as for
// a callback that makes tasks.
func JudgeResult(f *Flow, p Policy, v ssa.Value) Verdict {
	defer f.ask()()
	sources, _ := f.Sources(v)
	return newJudgement(f, p, SiteSpawned).result(sources, 1)
}

// judgement is one question to [Judge] or the like.
type judgement struct {
	f    *Flow
	p    Policy
	site Site
	// seen are the values judged so far in the question: one judged again
	// passes, as its verdict is already counted.
	seen map[ssa.Value]bool
}

func newJudgement(f *Flow, p Policy, site Site) *judgement {
	return &judgement{f: f, p: p, site: site, seen: make(map[ssa.Value]bool)}
}

// judgeRank orders the verdicts from the worst: Fail, then FailDeferOnly,
// then Pass and Unknown alike.
func judgeRank(v Verdict) int {
	switch v {
	case Fail:
		return 0
	case FailDeferOnly:
		return 1
	}
	return 2
}

// judgeAll gives the worst verdict of judge over items, Pass for none. It
// stops at the first Fail.
func judgeAll[T any](items []T, judge func(T) Verdict) Verdict {
	verdict := Pass
	for _, item := range items {
		if v := judge(item); judgeRank(v) < judgeRank(verdict) {
			verdict = v
		}
		if verdict == Fail {
			break
		}
	}
	return verdict
}

// judgeAny gives the best verdict of judge over items, Fail for none. It
// stops at the first that passes.
func judgeAny[T any](items []T, judge func(T) Verdict) Verdict {
	verdict := Fail
	for _, item := range items {
		if v := judge(item); judgeRank(v) > judgeRank(verdict) {
			verdict = v
		}
		if verdict.Passes() {
			break
		}
	}
	return verdict
}

// judgeBetter gives the better of a and b, a when they rank the same.
func judgeBetter(a, b Verdict) Verdict {
	if judgeRank(b) > judgeRank(a) {
		return b
	}
	return a
}

// value judges every source of v.
func (j *judgement) value(v ssa.Value) Verdict {
	if v == nil || j.seen[v] {
		return Pass
	}
	sources, _ := j.f.Sources(v)
	return j.sources(v, sources)
}

// sources judges every one of sources, those of v, v being judged for the
// first time.
func (j *judgement) sources(v ssa.Value, sources []Source) Verdict {
	j.seen[v] = true
	return judgeAll(sources, func(s Source) Verdict {
		if s.Value != v {
			if j.seen[s.Value] {
				return Pass
			}
			j.seen[s.Value] = true
		}
		return j.source(s)
	})
}

// source judges one source: a function literal by the policy, and the
// result of a call by the rules of call. The result of a call with several
// results, as fn, err := makeFn() gives, is not followed. A cancel func of
// context.WithCancel and its kind is the exception: it passes when the
// context it is made from is not fresh (#229). A parameter is Unknown, as
// what it is given is not known, unless it is of a factory whose result is
// followed from a call: call has found that the call gives it no func that
// passes, as when the closure a decorator returns runs the func the
// decorator is given.
func (j *judgement) source(s Source) Verdict {
	switch s.Kind {
	case SourceLiteral:
		return j.p.Literal(s.Func(), j.site)
	case SourceParam:
		if j.f.called[s.Value.Parent()] == 0 {
			return Unknown
		}
		return Fail
	case SourceCall:
		if call, ok := s.Value.(*ssa.Call); ok {
			return j.call(call.Common())
		}
		if parent, ok := contextCancelOf(s.Value); ok {
			if j.f.IsFreshContext(parent) {
				return Fail
			}
			return Pass
		}
	}
	return Unknown
}

// call judges the result of call, the function a factory returns. A call
// given a function that passes passes too: a decorator, as one that
// recovers panics, runs the function it is given.
func (j *judgement) call(call *ssa.CallCommon) Verdict {
	if v := j.p.Call(call); v != Unknown {
		return v
	}
	if call.IsInvoke() {
		return Unknown
	}
	if j.p.FactoryCall(call) {
		return Pass
	}
	decorated := judgeAny(call.Args, func(arg ssa.Value) Verdict {
		if _, isFunc := arg.Type().Underlying().(*types.Signature); !isFunc {
			return Fail
		}
		return j.value(arg)
	})
	if decorated.Passes() {
		return Pass
	}
	callees, _ := j.f.Sources(call.Value)
	for _, s := range callees {
		if fn := s.Func(); fn != nil {
			j.f.called[fn]++
			defer func() { j.f.called[fn]-- }()
		}
	}
	return judgeBetter(decorated, j.result(callees, 1))
}

// judgeMaxCalls bounds how many calls away a factory's result is followed,
// so that a factory whose type returns itself, as type stateFn func()
// stateFn, ends.
const judgeMaxCalls = 16

// result judges the function calling a value of sources depth times gives,
// depth being at least 1. Of a factory, the best value it returns counts. A
// result more than judgeMaxCalls calls away is Unknown.
func (j *judgement) result(sources []Source, depth int) Verdict {
	if depth > judgeMaxCalls {
		return Unknown
	}
	return judgeAll(sources, func(s Source) Verdict {
		if fn := s.Func(); fn != nil {
			return j.factory(fn, depth)
		}
		call, ok := s.Value.(*ssa.Call)
		switch {
		case !ok:
			return Unknown
		case j.p.FactoryCall(call.Common()):
			return Pass
		case call.Common().IsInvoke():
			return Unknown
		}
		callees, _ := j.f.Sources(call.Common().Value)
		return j.result(callees, depth+1)
	})
}

// factory judges fn by whether it takes what it needs, or by the best of
// the values it returns, called depth-1 more times.
func (j *judgement) factory(fn *ssa.Function, depth int) Verdict {
	if fn != nil && fn.Origin() != nil {
		// An instantiation of a generic function is a synthetic wrapper, or
		// has no body of its own.
		fn = fn.Origin()
	}
	if fn == nil || fn.Blocks == nil || fn.Synthetic != "" {
		return Unknown
	}
	if j.p.Factory(fn) {
		return Pass
	}
	returned := j.f.Returns(fn)
	if len(returned) == 0 {
		return Unknown
	}
	return judgeAny(returned, func(ret Return) Verdict {
		if depth == 1 {
			if j.seen[ret.Value] {
				return Pass
			}
			return j.sources(ret.Value, ret.Sources)
		}
		return j.result(ret.Sources, depth-1)
	})
}
