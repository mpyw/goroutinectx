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
func RegisterGotaskAPIs(reg *registry.Registry, deriverPattern *patterns.ShouldCallDeriver) {
	// gotask Do* functions - task callbacks should call deriver
	reg.Register(deriverPattern,
		registry.API{
			Pkg:            "github.com/siketyan/gotask",
			Type:           "",
			Name:           "Do",
			Kind:           registry.KindFunc,
			CallbackArgIdx: 1, // tasks start at index 1
		},
		registry.API{
			Pkg:            "github.com/siketyan/gotask",
			Type:           "",
			Name:           "DoAll",
			Kind:           registry.KindFunc,
			CallbackArgIdx: 1,
		},
		registry.API{
			Pkg:            "github.com/siketyan/gotask",
			Type:           "",
			Name:           "DoAllSettled",
			Kind:           registry.KindFunc,
			CallbackArgIdx: 1,
		},
		// DoAllFns, DoAllFnsSettled - callback receives ctx
		registry.API{
			Pkg:            "github.com/siketyan/gotask",
			Type:           "",
			Name:           "DoAllFns",
			Kind:           registry.KindFunc,
			CallbackArgIdx: 1,
		},
		registry.API{
			Pkg:            "github.com/siketyan/gotask",
			Type:           "",
			Name:           "DoAllFnsSettled",
			Kind:           registry.KindFunc,
			CallbackArgIdx: 1,
		},
	)

	// Task/CancelableTask.DoAsync - ctx should be derived
	reg.Register(deriverPattern,
		registry.API{
			Pkg:            "github.com/siketyan/gotask",
			Type:           "Task",
			Name:           "DoAsync",
			Kind:           registry.KindMethod,
			CallbackArgIdx: -1, // ctx is in call args, not callback
		},
		registry.API{
			Pkg:            "github.com/siketyan/gotask",
			Type:           "CancelableTask",
			Name:           "DoAsync",
			Kind:           registry.KindMethod,
			CallbackArgIdx: -1,
		},
	)
}
