package checker

import (
	"github.com/mpyw/goroutinectx/internal/patterns"
	"github.com/mpyw/goroutinectx/internal/registry"
)

// closureCapturesCtx is the shared pattern instance for closure-capturing APIs.
var closureCapturesCtx = &patterns.ClosureCapturesCtx{}

// RegisterDefaultAPIs registers all default APIs with the registry.
func RegisterDefaultAPIs(reg *registry.Registry, enableErrgroup, enableWaitgroup bool) {
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
	)
}

// RegisterGotaskAPIs registers gotask APIs with deriver pattern.
func RegisterGotaskAPIs(reg *registry.Registry, deriverPattern *patterns.ShouldCallDeriver, doAsyncPattern *patterns.ArgIsDeriverCall) {
	// DoAll, DoAllSettled, DoRace - variadic Task arguments
	// Each Task arg is traced through NewTask to check the callback body
	reg.Register(deriverPattern,
		registry.API{
			Pkg:            "github.com/siketyan/gotask",
			Type:           "",
			Name:           "DoAll",
			Kind:           registry.KindFunc,
			CallbackArgIdx: 1, // Tasks start at index 1 (after ctx)
			Variadic:       true,
		},
		registry.API{
			Pkg:            "github.com/siketyan/gotask",
			Type:           "",
			Name:           "DoAllSettled",
			Kind:           registry.KindFunc,
			CallbackArgIdx: 1,
			Variadic:       true,
		},
		registry.API{
			Pkg:            "github.com/siketyan/gotask",
			Type:           "",
			Name:           "DoRace",
			Kind:           registry.KindFunc,
			CallbackArgIdx: 1,
			Variadic:       true,
		},
	)

	// DoAllFns, DoAllFnsSettled, DoRaceFns - variadic functions
	// Each fn argument should call deriver in its body
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
	reg.Register(doAsyncPattern,
		registry.API{
			Pkg:            "github.com/siketyan/gotask",
			Type:           "Task",
			Name:           "DoAsync",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 0, // ctx is first argument
		},
		registry.API{
			Pkg:            "github.com/siketyan/gotask",
			Type:           "CancelableTask",
			Name:           "DoAsync",
			Kind:           registry.KindMethod,
			CallbackArgIdx: 0,
		},
	)
}
