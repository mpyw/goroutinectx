package internal

import (
	"github.com/mpyw/goroutinectx/internal/funcspec"
	"github.com/mpyw/goroutinectx/internal/registry"
)

// RegisterErrgroupAPIs registers errgroup.Group APIs.
func RegisterErrgroupAPIs(reg *registry.Registry) {
	reg.Register(registry.Entry{
		Spec:           funcspec.Spec{PkgPath: "golang.org/x/sync/errgroup", TypeName: "Group", FuncName: "Go"},
		CallbackArgIdx: 0,
	})
	reg.Register(registry.Entry{
		Spec:           funcspec.Spec{PkgPath: "golang.org/x/sync/errgroup", TypeName: "Group", FuncName: "TryGo"},
		CallbackArgIdx: 0,
	})
}

// RegisterWaitgroupAPIs registers sync.WaitGroup APIs (Go 1.25+).
func RegisterWaitgroupAPIs(reg *registry.Registry) {
	reg.Register(registry.Entry{
		Spec:           funcspec.Spec{PkgPath: "sync", TypeName: "WaitGroup", FuncName: "Go"},
		CallbackArgIdx: 0,
	})
}

// RegisterConcAPIs registers sourcegraph/conc pool APIs.
func RegisterConcAPIs(reg *registry.Registry) {
	entries := []registry.Entry{
		// conc.Pool.Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc", TypeName: "Pool", FuncName: "Go"}, CallbackArgIdx: 0},
		// conc.WaitGroup.Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc", TypeName: "WaitGroup", FuncName: "Go"}, CallbackArgIdx: 0},
		// pool.Pool.Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "Pool", FuncName: "Go"}, CallbackArgIdx: 0},
		// pool.ResultPool[T].Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultPool", FuncName: "Go"}, CallbackArgIdx: 0},
		// pool.ContextPool.Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ContextPool", FuncName: "Go"}, CallbackArgIdx: 0},
		// pool.ResultContextPool[T].Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultContextPool", FuncName: "Go"}, CallbackArgIdx: 0},
		// pool.ErrorPool.Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ErrorPool", FuncName: "Go"}, CallbackArgIdx: 0},
		// pool.ResultErrorPool[T].Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/pool", TypeName: "ResultErrorPool", FuncName: "Go"}, CallbackArgIdx: 0},
		// stream.Stream.Go
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/stream", TypeName: "Stream", FuncName: "Go"}, CallbackArgIdx: 0},
		// iter.ForEach
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", FuncName: "ForEach"}, CallbackArgIdx: 1},
		// iter.ForEachIdx
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", FuncName: "ForEachIdx"}, CallbackArgIdx: 1},
		// iter.Map
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", FuncName: "Map"}, CallbackArgIdx: 1},
		// iter.MapErr
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", FuncName: "MapErr"}, CallbackArgIdx: 1},
		// iter.Iterator.ForEach
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", TypeName: "Iterator", FuncName: "ForEach"}, CallbackArgIdx: 1},
		// iter.Iterator.ForEachIdx
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", TypeName: "Iterator", FuncName: "ForEachIdx"}, CallbackArgIdx: 1},
		// iter.Mapper.Map
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", TypeName: "Mapper", FuncName: "Map"}, CallbackArgIdx: 1},
		// iter.Mapper.MapErr
		{Spec: funcspec.Spec{PkgPath: "github.com/sourcegraph/conc/iter", TypeName: "Mapper", FuncName: "MapErr"}, CallbackArgIdx: 1},
	}
	for _, e := range entries {
		reg.Register(e)
	}
}

// RegisterGotaskAPIs registers gotask APIs.
func RegisterGotaskAPIs(reg *registry.Registry) {
	entries := []registry.Entry{
		// DoAll, DoAllSettled, DoRace - variadic Task arguments
		{Spec: funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoAll"}, CallbackArgIdx: 1},
		{Spec: funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoAllSettled"}, CallbackArgIdx: 1},
		{Spec: funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoRace"}, CallbackArgIdx: 1},
		// DoAllFns, DoAllFnsSettled, DoRaceFns - variadic functions
		{Spec: funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoAllFns"}, CallbackArgIdx: 1},
		{Spec: funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoAllFnsSettled"}, CallbackArgIdx: 1},
		{Spec: funcspec.Spec{PkgPath: "github.com/siketyan/gotask", FuncName: "DoRaceFns"}, CallbackArgIdx: 1},
		// Task.DoAsync, CancelableTask.DoAsync
		{Spec: funcspec.Spec{PkgPath: "github.com/siketyan/gotask", TypeName: "Task", FuncName: "DoAsync"}, AlwaysSpawns: true},
		{Spec: funcspec.Spec{PkgPath: "github.com/siketyan/gotask", TypeName: "CancelableTask", FuncName: "DoAsync"}, AlwaysSpawns: true},
	}
	for _, e := range entries {
		reg.Register(e)
	}
}
