// Package registry provides function registry for spawner API detection.
//
// # Overview
//
// The registry tracks functions that spawn goroutines with callback arguments.
// It is used by the spawner checker to detect when closures passed to these
// functions don't properly propagate context.
//
// # Registry Structure
//
//	type Registry struct {
//	    entries []Entry
//	}
//
//	type Entry struct {
//	    Spec           funcspec.Spec  // Function specification
//	    CallbackArgIdx int            // Index of callback argument
//	    AlwaysSpawns   bool           // Whether function always spawns goroutine
//	}
//
// # Registering Functions
//
// Use [Registry.Register] to add entries:
//
//	reg := registry.New()
//	reg.Register(registry.Entry{
//	    Spec: funcspec.Spec{
//	        PkgPath:  "golang.org/x/sync/errgroup",
//	        TypeName: "Group",
//	        FuncName: "Go",
//	    },
//	    CallbackArgIdx: 0,
//	    AlwaysSpawns:   true,
//	})
//
// # Matching Functions
//
// Use [Registry.MatchFunc] to check if a function is registered:
//
//	fn := funcspec.ExtractFunc(pass, call)
//	if match := reg.MatchFunc(fn); match != nil {
//	    // Function is a registered spawner
//	    argIdx := match.CallbackArgIdx
//	    fullName := match.FullName
//	}
//
// # Built-in Registrations
//
// The internal/apis.go file provides registration functions for known APIs:
//
//	RegisterErrgroupAPIs(reg)   // errgroup.Group.Go, TryGo
//	RegisterWaitgroupAPIs(reg)  // sync.WaitGroup.Go (Go 1.25+)
//	RegisterConcAPIs(reg)       // conc pool functions
//	RegisterGotaskAPIs(reg)     // gotask library functions
//
// # External Spawners
//
// Users can register external spawners via the -external-spawner flag:
//
//	-external-spawner=mycompany/pkg.RunAsync
//
// The analyzer parses this flag and registers the function at runtime.
//
// # FuncMatch Result
//
// When a function matches, [Registry.MatchFunc] returns:
//
//	type FuncMatch struct {
//	    FullName       string  // Display name for error messages
//	    CallbackArgIdx int     // Index of callback argument to check
//	    AlwaysSpawns   bool    // Whether spawning is guaranteed
//	}
package registry
