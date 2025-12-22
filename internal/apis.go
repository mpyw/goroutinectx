package internal

import (
	"github.com/mpyw/goroutinectx/internal/patterns"
	"github.com/mpyw/goroutinectx/internal/registry"
)

// RegisterDefaultAPIs registers all default APIs with the registry.
func RegisterDefaultAPIs(reg *registry.Registry, enableErrgroup, enableWaitgroup, enableConc bool) {
	closureCapturesCtx := &patterns.ClosureCapturesCtx{}
	// errgroup.Group.Go - closure should capture ctx
	if enableErrgroup {
		reg.Register(closureCapturesCtx,
			registry.API{
				Pkg:            "golang.org/x/sync/errgroup",
				Type:           "Group",
				Name:           "Go",
				Kind:           registry.KindMethod,
				CallbackArgIdx: 0,
			},
			registry.API{
				Pkg:            "golang.org/x/sync/errgroup",
				Type:           "Group",
				Name:           "TryGo",
				Kind:           registry.KindMethod,
				CallbackArgIdx: 0,
			},
		)
	}

	// sync.WaitGroup.Go (Go 1.25+) - closure should capture ctx
	if enableWaitgroup {
		reg.Register(closureCapturesCtx,
			registry.API{
				Pkg:            "sync",
				Type:           "WaitGroup",
				Name:           "Go",
				Kind:           registry.KindMethod,
				CallbackArgIdx: 0,
			},
		)
	}

	// sourcegraph/conc pool APIs - closure should capture ctx
	if !enableConc {
		return
	}
	reg.Register(closureCapturesCtx,
		// Pool.Go
		registry.API{
			Pkg:            "github.com/sourcegraph/conc",
			Type:           "Pool",
			Name:           "Go",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 0,
		},
		// WaitGroup.Go
		registry.API{
			Pkg:            "github.com/sourcegraph/conc",
			Type:           "WaitGroup",
			Name:           "Go",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 0,
		},
		// pool.Pool.Go
		registry.API{
			Pkg:            "github.com/sourcegraph/conc/pool",
			Type:           "Pool",
			Name:           "Go",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 0,
		},
		// pool.ResultPool[T].Go
		registry.API{
			Pkg:            "github.com/sourcegraph/conc/pool",
			Type:           "ResultPool",
			Name:           "Go",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 0,
		},
		// pool.ContextPool.Go
		registry.API{
			Pkg:            "github.com/sourcegraph/conc/pool",
			Type:           "ContextPool",
			Name:           "Go",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 0,
		},
		// pool.ResultContextPool[T].Go
		registry.API{
			Pkg:            "github.com/sourcegraph/conc/pool",
			Type:           "ResultContextPool",
			Name:           "Go",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 0,
		},
		// pool.ErrorPool.Go
		registry.API{
			Pkg:            "github.com/sourcegraph/conc/pool",
			Type:           "ErrorPool",
			Name:           "Go",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 0,
		},
		// pool.ResultErrorPool[T].Go
		registry.API{
			Pkg:            "github.com/sourcegraph/conc/pool",
			Type:           "ResultErrorPool",
			Name:           "Go",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 0,
		},
		// stream.Stream.Go
		registry.API{
			Pkg:            "github.com/sourcegraph/conc/stream",
			Type:           "Stream",
			Name:           "Go",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 0,
		},
		// iter.ForEach
		registry.API{
			Pkg:            "github.com/sourcegraph/conc/iter",
			Type:           "",
			Name:           "ForEach",
			Kind:           registry.KindFunc,
			CallbackArgIdx: 1,
		},
		// iter.ForEachIdx
		registry.API{
			Pkg:            "github.com/sourcegraph/conc/iter",
			Type:           "",
			Name:           "ForEachIdx",
			Kind:           registry.KindFunc,
			CallbackArgIdx: 1,
		},
		// iter.Map
		registry.API{
			Pkg:            "github.com/sourcegraph/conc/iter",
			Type:           "",
			Name:           "Map",
			Kind:           registry.KindFunc,
			CallbackArgIdx: 1,
		},
		// iter.MapErr
		registry.API{
			Pkg:            "github.com/sourcegraph/conc/iter",
			Type:           "",
			Name:           "MapErr",
			Kind:           registry.KindFunc,
			CallbackArgIdx: 1,
		},
		// iter.Iterator.ForEach
		registry.API{
			Pkg:            "github.com/sourcegraph/conc/iter",
			Type:           "Iterator",
			Name:           "ForEach",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 1,
		},
		// iter.Iterator.ForEachIdx
		registry.API{
			Pkg:            "github.com/sourcegraph/conc/iter",
			Type:           "Iterator",
			Name:           "ForEachIdx",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 1,
		},
		// iter.Mapper.Map
		registry.API{
			Pkg:            "github.com/sourcegraph/conc/iter",
			Type:           "Mapper",
			Name:           "Map",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 1,
		},
		// iter.Mapper.MapErr
		registry.API{
			Pkg:            "github.com/sourcegraph/conc/iter",
			Type:           "Mapper",
			Name:           "MapErr",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 1,
		},
	)
}

// RegisterGotaskAPIs registers gotask APIs.
// If deriverPattern/doAsyncPattern are nil, APIs are registered for detection only (spawnerlabel).
// If patterns are provided, APIs are registered with pattern checking (unified checker).
func RegisterGotaskAPIs(reg *registry.Registry, deriverPattern patterns.Pattern, doAsyncPattern patterns.Pattern) {
	// gotaskTaskConstructor defines how gotask tasks are created.
	// Used to trace DoAll/DoAsync calls back to NewTask to find the callback.
	gotaskTaskConstructor := &patterns.TaskConstructor{
		Pkg:            "github.com/siketyan/gotask",
		Name:           "NewTask",
		CallbackArgIdx: 0,
	}

	// DoAll, DoAllSettled, DoRace - variadic Task arguments
	// Each Task arg is traced through NewTask to check the callback body
	reg.Register(deriverPattern,
		registry.API{
			Pkg:             "github.com/siketyan/gotask",
			Type:            "",
			Name:            "DoAll",
			Kind:            registry.KindFunc,
			CallbackArgIdx:  1, // Tasks start at index 1 (after ctx)
			Variadic:        true,
			TaskConstructor: gotaskTaskConstructor,
		},
		registry.API{
			Pkg:             "github.com/siketyan/gotask",
			Type:            "",
			Name:            "DoAllSettled",
			Kind:            registry.KindFunc,
			CallbackArgIdx:  1,
			Variadic:        true,
			TaskConstructor: gotaskTaskConstructor,
		},
		registry.API{
			Pkg:             "github.com/siketyan/gotask",
			Type:            "",
			Name:            "DoRace",
			Kind:            registry.KindFunc,
			CallbackArgIdx:  1,
			Variadic:        true,
			TaskConstructor: gotaskTaskConstructor,
		},
	)

	// DoAllFns, DoAllFnsSettled, DoRaceFns - variadic functions
	// Each fn argument should call deriver in its body (no task constructor needed)
	reg.Register(deriverPattern,
		registry.API{
			Pkg:            "github.com/siketyan/gotask",
			Type:           "",
			Name:           "DoAllFns",
			Kind:           registry.KindFunc,
			CallbackArgIdx: 1, // fns start at index 1
			Variadic:       true,
		},
		registry.API{
			Pkg:            "github.com/siketyan/gotask",
			Type:           "",
			Name:           "DoAllFnsSettled",
			Kind:           registry.KindFunc,
			CallbackArgIdx: 1,
			Variadic:       true,
		},
		registry.API{
			Pkg:            "github.com/siketyan/gotask",
			Type:           "",
			Name:           "DoRaceFns",
			Kind:           registry.KindFunc,
			CallbackArgIdx: 1,
			Variadic:       true,
		},
	)

	// Task.DoAsync, CancelableTask.DoAsync - ctx arg should BE a deriver call
	// OR the task's callback (from NewTask) should call deriver
	reg.Register(doAsyncPattern,
		registry.API{
			Pkg:             "github.com/siketyan/gotask",
			Type:            "Task",
			Name:            "DoAsync",
			Kind:            registry.KindMethod,
			CallbackArgIdx:  0, // ctx is first argument
			TaskSourceIdx:   patterns.TaskReceiverIdx,
			TaskConstructor: gotaskTaskConstructor,
		},
		registry.API{
			Pkg:             "github.com/siketyan/gotask",
			Type:            "CancelableTask",
			Name:            "DoAsync",
			Kind:            registry.KindMethod,
			CallbackArgIdx:  0,
			TaskSourceIdx:   patterns.TaskReceiverIdx,
			TaskConstructor: gotaskTaskConstructor,
		},
	)
}
