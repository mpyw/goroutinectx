package internal

import (
	"github.com/mpyw/goroutinectx/internal/directives/deriver"
	"github.com/mpyw/goroutinectx/internal/funcspec"
	"github.com/mpyw/goroutinectx/internal/patterns"
	"github.com/mpyw/goroutinectx/internal/registry"
)

// RegisterGoStmtPatterns registers GoStmt patterns based on flags.
func RegisterGoStmtPatterns(reg *registry.Registry, enableGoroutine bool, goroutineDeriver string) {
	if enableGoroutine {
		reg.RegisterGoStmt(&patterns.GoStmtCapturesCtx{})
	}

	if goroutineDeriver != "" {
		matcher := deriver.NewMatcher(goroutineDeriver)
		reg.RegisterGoStmt(&patterns.GoStmtCallsDeriver{Matcher: matcher})
	}
}

// RegisterErrgroupAPIs registers errgroup.Group APIs.
func RegisterErrgroupAPIs(reg *registry.Registry, enabled bool, closurePatterns []patterns.CallArgPattern) {
	if !enabled {
		return
	}

	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "golang.org/x/sync/errgroup", TypeName: "Group", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       closurePatterns,
	})

	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "golang.org/x/sync/errgroup", TypeName: "Group", FuncName: "TryGo"},
		CallbackArgIdx: 0,
		Patterns:       closurePatterns,
	})
}

// RegisterWaitgroupAPIs registers sync.WaitGroup APIs (Go 1.25+).
func RegisterWaitgroupAPIs(reg *registry.Registry, enabled bool, closurePatterns []patterns.CallArgPattern) {
	if !enabled {
		return
	}

	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "sync", TypeName: "WaitGroup", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       closurePatterns,
	})
}

// RegisterConcAPIs registers sourcegraph/conc pool APIs.
func RegisterConcAPIs(reg *registry.Registry, enabled bool, closurePatterns []patterns.CallArgPattern) {
	if !enabled {
		return
	}

	// conc.Pool.Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc", TypeName: "Pool", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       closurePatterns,
	})

	// conc.WaitGroup.Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc", TypeName: "WaitGroup", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       closurePatterns,
	})

	// pool.Pool.Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "Pool", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       closurePatterns,
	})

	// pool.ResultPool[T].Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultPool", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       closurePatterns,
	})

	// pool.ContextPool.Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ContextPool", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       closurePatterns,
	})

	// pool.ResultContextPool[T].Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultContextPool", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       closurePatterns,
	})

	// pool.ErrorPool.Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ErrorPool", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       closurePatterns,
	})

	// pool.ResultErrorPool[T].Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultErrorPool", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       closurePatterns,
	})

	// stream.Stream.Go
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/stream", TypeName: "Stream", FuncName: "Go"},
		CallbackArgIdx: 0,
		Patterns:       closurePatterns,
	})

	// iter.ForEach
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", FuncName: "ForEach"},
		CallbackArgIdx: 1,
		Patterns:       closurePatterns,
	})

	// iter.ForEachIdx
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", FuncName: "ForEachIdx"},
		CallbackArgIdx: 1,
		Patterns:       closurePatterns,
	})

	// iter.Map
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", FuncName: "Map"},
		CallbackArgIdx: 1,
		Patterns:       closurePatterns,
	})

	// iter.MapErr
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", FuncName: "MapErr"},
		CallbackArgIdx: 1,
		Patterns:       closurePatterns,
	})

	// iter.Iterator.ForEach
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", TypeName: "Iterator", FuncName: "ForEach"},
		CallbackArgIdx: 1,
		Patterns:       closurePatterns,
	})

	// iter.Iterator.ForEachIdx
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", TypeName: "Iterator", FuncName: "ForEachIdx"},
		CallbackArgIdx: 1,
		Patterns:       closurePatterns,
	})

	// iter.Mapper.Map
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", TypeName: "Mapper", FuncName: "Map"},
		CallbackArgIdx: 1,
		Patterns:       closurePatterns,
	})

	// iter.Mapper.MapErr
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", TypeName: "Mapper", FuncName: "MapErr"},
		CallbackArgIdx: 1,
		Patterns:       closurePatterns,
	})
}

// RegisterGotaskAPIs registers gotask APIs.
// If patterns are empty, APIs are registered for detection only (spawnerlabel).
// If patterns are provided, APIs are registered with pattern checking (unified checker).
func RegisterGotaskAPIs(reg *registry.Registry, deriverPatterns []patterns.CallArgPattern, doAsyncPatterns []patterns.TaskSourcePattern) {
	// gotaskConstructor defines how gotask tasks are created.
	// Used to trace DoAll/DoAsync calls back to NewTask to find the callback.
	gotaskConstructor := &patterns.TaskConstructorConfig{
		Pkg:            "github.com/siketyan/gotask",
		Name:           "NewTask",
		CallbackArgIdx: 0,
	}

	// DoAll, DoAllSettled, DoRace - variadic Task arguments
	// Each Task arg is traced through NewTask to check the callback body
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:            funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoAll"},
		CallbackArgIdx:  1, // Tasks start at index 1 (after ctx)
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
	// Each fn argument should call deriver in its body (no task constructor needed)
	reg.RegisterCallArg(registry.CallArgEntry{
		Spec:           funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoAllFns"},
		CallbackArgIdx: 1, // fns start at index 1
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

	// Task.DoAsync, CancelableTask.DoAsync - ctx arg should BE a deriver call
	// OR the task's callback (from NewTask) should call deriver
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
