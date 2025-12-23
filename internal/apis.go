package internal

import (
	"github.com/mpyw/goroutinectx/internal/directives/deriver"
	"github.com/mpyw/goroutinectx/internal/directives/ignore"
	"github.com/mpyw/goroutinectx/internal/funcspec"
	"github.com/mpyw/goroutinectx/internal/patterns"
	"github.com/mpyw/goroutinectx/internal/registry"
)

// buildCallArgPatterns creates patterns for CallArg APIs.
// Always includes ClosureCapturesCtx, and adds CallbackCallsDeriver if derivers is set.
func buildCallArgPatterns(checkerName ignore.CheckerName, derivers *deriver.Matcher) []patterns.CallArgPattern {
	p := []patterns.CallArgPattern{patterns.NewClosureCapturesCtx(checkerName)}
	if derivers != nil {
		p = append(p, &patterns.CallbackCallsDeriver{Matcher: derivers})
	}
	return p
}

// RegisterGoroutinePatterns registers goroutine patterns.
func RegisterGoroutinePatterns(reg *registry.Registry, derivers *deriver.Matcher) {
	reg.RegisterGoStmt(&patterns.GoStmtCapturesCtx{})
	if derivers != nil {
		reg.RegisterGoStmt(&patterns.GoStmtCallsDeriver{Matcher: derivers})
	}
}

// RegisterErrgroupAPIs registers errgroup.Group APIs.
func RegisterErrgroupAPIs(reg *registry.Registry, derivers *deriver.Matcher) {
	p := buildCallArgPatterns(ignore.Errgroup, derivers)

	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "golang.org/x/sync/errgroup", TypeName: "Group", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       p,
	})

	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "golang.org/x/sync/errgroup", TypeName: "Group", FuncName: "TryGo"},
		CallbackArgIdx: 0,
		Patterns:       p,
	})
}

// RegisterWaitgroupAPIs registers sync.WaitGroup APIs (Go 1.25+).
func RegisterWaitgroupAPIs(reg *registry.Registry, derivers *deriver.Matcher) {
	p := buildCallArgPatterns(ignore.Waitgroup, derivers)

	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "sync", TypeName: "WaitGroup", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       p,
	})
}

// RegisterConcAPIs registers sourcegraph/conc pool APIs.
// Uses errgroup checker name since conc is conceptually similar.
func RegisterConcAPIs(reg *registry.Registry, derivers *deriver.Matcher) {
	p := buildCallArgPatterns(ignore.Errgroup, derivers)

	// conc.Pool.Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc", TypeName: "Pool", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       p,
	})

	// conc.WaitGroup.Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc", TypeName: "WaitGroup", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       p,
	})

	// pool.Pool.Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "Pool", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       p,
	})

	// pool.ResultPool[T].Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultPool", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       p,
	})

	// pool.ContextPool.Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ContextPool", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       p,
	})

	// pool.ResultContextPool[T].Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultContextPool", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       p,
	})

	// pool.ErrorPool.Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ErrorPool", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       p,
	})

	// pool.ResultErrorPool[T].Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultErrorPool", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       p,
	})

	// stream.Stream.Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/stream", TypeName: "Stream", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       p,
	})

	// iter.ForEach
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", FuncName: "ForEach"},
		CallbackArgIdx: 1,
		Patterns:       p,
	})

	// iter.ForEachIdx
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", FuncName: "ForEachIdx"},
		CallbackArgIdx: 1,
		Patterns:       p,
	})

	// iter.Map
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", FuncName: "Map"},
		CallbackArgIdx: 1,
		Patterns:       p,
	})

	// iter.MapErr
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", FuncName: "MapErr"},
		CallbackArgIdx: 1,
		Patterns:       p,
	})

	// iter.Iterator.ForEach
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", TypeName: "Iterator", FuncName: "ForEach"},
		CallbackArgIdx: 1,
		Patterns:       p,
	})

	// iter.Iterator.ForEachIdx
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", TypeName: "Iterator", FuncName: "ForEachIdx"},
		CallbackArgIdx: 1,
		Patterns:       p,
	})

	// iter.Mapper.Map
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", TypeName: "Mapper", FuncName: "Map"},
		CallbackArgIdx: 1,
		Patterns:       p,
	})

	// iter.Mapper.MapErr
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", TypeName: "Mapper", FuncName: "MapErr"},
		CallbackArgIdx: 1,
		Patterns:       p,
	})
}

// RegisterGotaskAPIs registers gotask APIs.
// If derivers is nil, APIs are registered for detection only (spawnerlabel).
// If derivers is set, APIs are registered with pattern checking.
func RegisterGotaskAPIs(reg *registry.Registry, derivers *deriver.Matcher) {
	// gotaskConstructor defines how gotask tasks are created.
	gotaskConstructor := &patterns.TaskConstructorConfig{
		Pkg:            "github.com/siketyan/gotask",
		Name:           "NewTask",
		CallbackArgIdx: 0,
	}

	// Build patterns only if derivers is configured
	var deriverPatterns []patterns.CallArgPattern
	var doAsyncPatterns []patterns.TaskSourcePattern
	if derivers != nil {
		callbackCallsDeriver := &patterns.CallbackCallsDeriver{Matcher: derivers}
		deriverPatterns = []patterns.CallArgPattern{callbackCallsDeriver}
		doAsyncPatterns = []patterns.TaskSourcePattern{callbackCallsDeriver.OrCtxDerived()}
	}

	// DoAll, DoAllSettled, DoRace - variadic Task arguments
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:            funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoAll"},
		CallbackArgIdx:  1,
		Variadic:        true,
		TaskConstructor: gotaskConstructor,
		Patterns:        deriverPatterns,
	})

	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:            funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoAllSettled"},
		CallbackArgIdx:  1,
		Variadic:        true,
		TaskConstructor: gotaskConstructor,
		Patterns:        deriverPatterns,
	})

	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:            funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoRace"},
		CallbackArgIdx:  1,
		Variadic:        true,
		TaskConstructor: gotaskConstructor,
		Patterns:        deriverPatterns,
	})

	// DoAllFns, DoAllFnsSettled, DoRaceFns - variadic functions
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoAllFns"},
		CallbackArgIdx: 1,
		Variadic:       true,
		Patterns:       deriverPatterns,
	})

	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoAllFnsSettled"},
		CallbackArgIdx: 1,
		Variadic:       true,
		Patterns:       deriverPatterns,
	})

	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoRaceFns"},
		CallbackArgIdx: 1,
		Variadic:       true,
		Patterns:       deriverPatterns,
	})

	// Task.DoAsync, CancelableTask.DoAsync
	reg.RegisterTaskSource(registry.TaskSourceEntry{
		Spec:            funcspec.Spec{PkgPath: "github.com/siketyan/gotask", TypeName: "Task", FuncName: "DoAsync"},
		TaskConstructor: gotaskConstructor,
		Patterns:        doAsyncPatterns,
	})

	reg.RegisterTaskSource(registry.TaskSourceEntry{
		Spec:            funcspec.Spec{PkgPath: "github.com/siketyan/gotask", TypeName: "CancelableTask", FuncName: "DoAsync"},
		TaskConstructor: gotaskConstructor,
		Patterns:        doAsyncPatterns,
	})
}
