package internal

import (
	"github.com/mpyw/goroutinectx/internal/patterns"
	"github.com/mpyw/goroutinectx/internal/registry"
)

// RegisterDefaultAPIs registers all default APIs with the registry.
func RegisterDefaultAPIs(reg *registry.Registry, enableErrgroup, enableWaitgroup, enableConc bool, closurePatterns []patterns.Pattern) {
	// errgroup.Group.Go, TryGo - closure should capture ctx
	if enableErrgroup {
		reg.Register(registry.API{
			Pkg:            "golang.org/x/sync/errgroup",
			Type:           "Group",
			Name:           "Go",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 0,
		}, closurePatterns...)

		reg.Register(registry.API{
			Pkg:            "golang.org/x/sync/errgroup",
			Type:           "Group",
			Name:           "TryGo",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 0,
		}, closurePatterns...)
	}

	// sync.WaitGroup.Go (Go 1.25+) - closure should capture ctx
	if enableWaitgroup {
		reg.Register(registry.API{
			Pkg:            "sync",
			Type:           "WaitGroup",
			Name:           "Go",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 0,
		}, closurePatterns...)
	}

	// sourcegraph/conc pool APIs - closure should capture ctx
	if !enableConc {
		return
	}

	// conc.Pool.Go
	reg.Register(registry.API{
		Pkg:            "github.com/sourcegraph/conc",
		Type:           "Pool",
		Name:           "Go",
		Kind:           registry.KindMethod,
		CallbackArgIdx: 0,
	}, closurePatterns...)

	// conc.WaitGroup.Go
	reg.Register(registry.API{
		Pkg:            "github.com/sourcegraph/conc",
		Type:           "WaitGroup",
		Name:           "Go",
		Kind:           registry.KindMethod,
		CallbackArgIdx: 0,
	}, closurePatterns...)

	// pool.Pool.Go
	reg.Register(registry.API{
		Pkg:            "github.com/sourcegraph/conc/pool",
		Type:           "Pool",
		Name:           "Go",
		Kind:           registry.KindMethod,
		CallbackArgIdx: 0,
	}, closurePatterns...)

	// pool.ResultPool[T].Go
	reg.Register(registry.API{
		Pkg:            "github.com/sourcegraph/conc/pool",
		Type:           "ResultPool",
		Name:           "Go",
		Kind:           registry.KindMethod,
		CallbackArgIdx: 0,
	}, closurePatterns...)

	// pool.ContextPool.Go
	reg.Register(registry.API{
		Pkg:            "github.com/sourcegraph/conc/pool",
		Type:           "ContextPool",
		Name:           "Go",
		Kind:           registry.KindMethod,
		CallbackArgIdx: 0,
	}, closurePatterns...)

	// pool.ResultContextPool[T].Go
	reg.Register(registry.API{
		Pkg:            "github.com/sourcegraph/conc/pool",
		Type:           "ResultContextPool",
		Name:           "Go",
		Kind:           registry.KindMethod,
		CallbackArgIdx: 0,
	}, closurePatterns...)

	// pool.ErrorPool.Go
	reg.Register(registry.API{
		Pkg:            "github.com/sourcegraph/conc/pool",
		Type:           "ErrorPool",
		Name:           "Go",
		Kind:           registry.KindMethod,
		CallbackArgIdx: 0,
	}, closurePatterns...)

	// pool.ResultErrorPool[T].Go
	reg.Register(registry.API{
		Pkg:            "github.com/sourcegraph/conc/pool",
		Type:           "ResultErrorPool",
		Name:           "Go",
		Kind:           registry.KindMethod,
		CallbackArgIdx: 0,
	}, closurePatterns...)

	// stream.Stream.Go
	reg.Register(registry.API{
		Pkg:            "github.com/sourcegraph/conc/stream",
		Type:           "Stream",
		Name:           "Go",
		Kind:           registry.KindMethod,
		CallbackArgIdx: 0,
	}, closurePatterns...)

	// iter.ForEach
	reg.Register(registry.API{
		Pkg:            "github.com/sourcegraph/conc/iter",
		Type:           "",
		Name:           "ForEach",
		Kind:           registry.KindFunc,
		CallbackArgIdx: 1,
	}, closurePatterns...)

	// iter.ForEachIdx
	reg.Register(registry.API{
		Pkg:            "github.com/sourcegraph/conc/iter",
		Type:           "",
		Name:           "ForEachIdx",
		Kind:           registry.KindFunc,
		CallbackArgIdx: 1,
	}, closurePatterns...)

	// iter.Map
	reg.Register(registry.API{
		Pkg:            "github.com/sourcegraph/conc/iter",
		Type:           "",
		Name:           "Map",
		Kind:           registry.KindFunc,
		CallbackArgIdx: 1,
	}, closurePatterns...)

	// iter.MapErr
	reg.Register(registry.API{
		Pkg:            "github.com/sourcegraph/conc/iter",
		Type:           "",
		Name:           "MapErr",
		Kind:           registry.KindFunc,
		CallbackArgIdx: 1,
	}, closurePatterns...)

	// iter.Iterator.ForEach
	reg.Register(registry.API{
		Pkg:            "github.com/sourcegraph/conc/iter",
		Type:           "Iterator",
		Name:           "ForEach",
		Kind:           registry.KindMethod,
		CallbackArgIdx: 1,
	}, closurePatterns...)

	// iter.Iterator.ForEachIdx
	reg.Register(registry.API{
		Pkg:            "github.com/sourcegraph/conc/iter",
		Type:           "Iterator",
		Name:           "ForEachIdx",
		Kind:           registry.KindMethod,
		CallbackArgIdx: 1,
	}, closurePatterns...)

	// iter.Mapper.Map
	reg.Register(registry.API{
		Pkg:            "github.com/sourcegraph/conc/iter",
		Type:           "Mapper",
		Name:           "Map",
		Kind:           registry.KindMethod,
		CallbackArgIdx: 1,
	}, closurePatterns...)

	// iter.Mapper.MapErr
	reg.Register(registry.API{
		Pkg:            "github.com/sourcegraph/conc/iter",
		Type:           "Mapper",
		Name:           "MapErr",
		Kind:           registry.KindMethod,
		CallbackArgIdx: 1,
	}, closurePatterns...)
}

// RegisterGotaskAPIs registers gotask APIs.
// If patterns are empty, APIs are registered for detection only (spawnerlabel).
// If patterns are provided, APIs are registered with pattern checking (unified checker).
func RegisterGotaskAPIs(reg *registry.Registry, deriverPatterns []patterns.Pattern, doAsyncPatterns []patterns.Pattern) {
	// gotaskConstructor defines how gotask tasks are created.
	// Used to trace DoAll/DoAsync calls back to NewTask to find the callback.
	gotaskConstructor := &patterns.TaskConstructorConfig{
		Pkg:            "github.com/siketyan/gotask",
		Name:           "NewTask",
		CallbackArgIdx: 0,
	}

	// taskArgConfig for variadic Task arguments (DoAll, etc.)
	// Idx is not used for variadic APIs - each argument is checked independently
	variadicTaskArgConfig := &patterns.TaskArgumentConfig{
		Constructor: gotaskConstructor,
		Idx:         0, // Not used for variadic
	}

	// taskArgConfig for DoAsync (task comes from receiver)
	doAsyncTaskArgConfig := &patterns.TaskArgumentConfig{
		Constructor: gotaskConstructor,
		Idx:         patterns.TaskReceiverIdx,
	}

	// DoAll, DoAllSettled, DoRace - variadic Task arguments
	// Each Task arg is traced through NewTask to check the callback body
	reg.Register(registry.API{
		Pkg:            "github.com/siketyan/gotask",
		Type:           "",
		Name:           "DoAll",
		Kind:           registry.KindFunc,
		CallbackArgIdx: 1, // Tasks start at index 1 (after ctx)
		Variadic:       true,
		TaskArgConfig:  variadicTaskArgConfig,
	}, deriverPatterns...)

	reg.Register(registry.API{
		Pkg:            "github.com/siketyan/gotask",
		Type:           "",
		Name:           "DoAllSettled",
		Kind:           registry.KindFunc,
		CallbackArgIdx: 1,
		Variadic:       true,
		TaskArgConfig:  variadicTaskArgConfig,
	}, deriverPatterns...)

	reg.Register(registry.API{
		Pkg:            "github.com/siketyan/gotask",
		Type:           "",
		Name:           "DoRace",
		Kind:           registry.KindFunc,
		CallbackArgIdx: 1,
		Variadic:       true,
		TaskArgConfig:  variadicTaskArgConfig,
	}, deriverPatterns...)

	// DoAllFns, DoAllFnsSettled, DoRaceFns - variadic functions
	// Each fn argument should call deriver in its body (no task constructor needed)
	reg.Register(registry.API{
		Pkg:            "github.com/siketyan/gotask",
		Type:           "",
		Name:           "DoAllFns",
		Kind:           registry.KindFunc,
		CallbackArgIdx: 1, // fns start at index 1
		Variadic:       true,
	}, deriverPatterns...)

	reg.Register(registry.API{
		Pkg:            "github.com/siketyan/gotask",
		Type:           "",
		Name:           "DoAllFnsSettled",
		Kind:           registry.KindFunc,
		CallbackArgIdx: 1,
		Variadic:       true,
	}, deriverPatterns...)

	reg.Register(registry.API{
		Pkg:            "github.com/siketyan/gotask",
		Type:           "",
		Name:           "DoRaceFns",
		Kind:           registry.KindFunc,
		CallbackArgIdx: 1,
		Variadic:       true,
	}, deriverPatterns...)

	// Task.DoAsync, CancelableTask.DoAsync - ctx arg should BE a deriver call
	// OR the task's callback (from NewTask) should call deriver
	reg.Register(registry.API{
		Pkg:            "github.com/siketyan/gotask",
		Type:           "Task",
		Name:           "DoAsync",
		Kind:           registry.KindMethod,
		CallbackArgIdx: 0, // ctx is first argument
		TaskArgConfig:  doAsyncTaskArgConfig,
	}, doAsyncPatterns...)

	reg.Register(registry.API{
		Pkg:            "github.com/siketyan/gotask",
		Type:           "CancelableTask",
		Name:           "DoAsync",
		Kind:           registry.KindMethod,
		CallbackArgIdx: 0,
		TaskArgConfig:  doAsyncTaskArgConfig,
	}, doAsyncPatterns...)
}
