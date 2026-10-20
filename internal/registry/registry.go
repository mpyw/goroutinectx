package registry

import (
	"go/types"

	"github.com/mpyw/goroutinectx/internal/checker"
	"github.com/mpyw/goroutinectx/internal/funcspec"
)

// Entry is one supported API: a function or method that runs a callback, or
// its receiver task, in a new goroutine.
type Entry struct {
	Spec funcspec.Spec
	// Checker is the checker that checks calls to the API.
	Checker checker.Name
	// CallbackArgIdx is the index of the callback argument, the first one
	// when the API is variadic.
	CallbackArgIdx int
	// Variadic is true when every argument from CallbackArgIdx on is a
	// callback.
	Variadic bool
	// AlwaysSpawns is true when the API spawns its receiver task, with no
	// callback argument.
	AlwaysSpawns bool
}

const (
	errgroupPkg = "golang.org/x/sync/errgroup"
	concPkg     = "github.com/sourcegraph/conc"
	poolPkg     = "github.com/sourcegraph/conc/pool"
	streamPkg   = "github.com/sourcegraph/conc/stream"
	iterPkg     = "github.com/sourcegraph/conc/iter"
	gotaskPkg   = "github.com/siketyan/gotask"
)

// entries is the table of every supported API.
var entries = []Entry{
	// golang.org/x/sync/errgroup
	{Spec: funcspec.Spec{PkgPath: errgroupPkg, TypeName: "Group", FuncName: "Go"}, Checker: checker.Errgroup},
	{Spec: funcspec.Spec{PkgPath: errgroupPkg, TypeName: "Group", FuncName: "TryGo"}, Checker: checker.Errgroup},

	// sync (Go 1.25+)
	{Spec: funcspec.Spec{PkgPath: "sync", TypeName: "WaitGroup", FuncName: "Go"}, Checker: checker.Waitgroup},

	// github.com/sourcegraph/conc
	{Spec: funcspec.Spec{PkgPath: concPkg, TypeName: "Pool", FuncName: "Go"}, Checker: checker.Conc},
	{Spec: funcspec.Spec{PkgPath: concPkg, TypeName: "WaitGroup", FuncName: "Go"}, Checker: checker.Conc},
	{Spec: funcspec.Spec{PkgPath: poolPkg, TypeName: "Pool", FuncName: "Go"}, Checker: checker.Conc},
	{Spec: funcspec.Spec{PkgPath: poolPkg, TypeName: "ResultPool", FuncName: "Go"}, Checker: checker.Conc},
	{Spec: funcspec.Spec{PkgPath: poolPkg, TypeName: "ContextPool", FuncName: "Go"}, Checker: checker.Conc},
	{Spec: funcspec.Spec{PkgPath: poolPkg, TypeName: "ResultContextPool", FuncName: "Go"}, Checker: checker.Conc},
	{Spec: funcspec.Spec{PkgPath: poolPkg, TypeName: "ErrorPool", FuncName: "Go"}, Checker: checker.Conc},
	{Spec: funcspec.Spec{PkgPath: poolPkg, TypeName: "ResultErrorPool", FuncName: "Go"}, Checker: checker.Conc},
	{Spec: funcspec.Spec{PkgPath: streamPkg, TypeName: "Stream", FuncName: "Go"}, Checker: checker.Conc},
	{Spec: funcspec.Spec{PkgPath: iterPkg, FuncName: "ForEach"}, Checker: checker.Conc, CallbackArgIdx: 1},
	{Spec: funcspec.Spec{PkgPath: iterPkg, FuncName: "ForEachIdx"}, Checker: checker.Conc, CallbackArgIdx: 1},
	{Spec: funcspec.Spec{PkgPath: iterPkg, FuncName: "Map"}, Checker: checker.Conc, CallbackArgIdx: 1},
	{Spec: funcspec.Spec{PkgPath: iterPkg, FuncName: "MapErr"}, Checker: checker.Conc, CallbackArgIdx: 1},
	{Spec: funcspec.Spec{PkgPath: iterPkg, TypeName: "Iterator", FuncName: "ForEach"}, Checker: checker.Conc, CallbackArgIdx: 1},
	{Spec: funcspec.Spec{PkgPath: iterPkg, TypeName: "Iterator", FuncName: "ForEachIdx"}, Checker: checker.Conc, CallbackArgIdx: 1},
	{Spec: funcspec.Spec{PkgPath: iterPkg, TypeName: "Mapper", FuncName: "Map"}, Checker: checker.Conc, CallbackArgIdx: 1},
	{Spec: funcspec.Spec{PkgPath: iterPkg, TypeName: "Mapper", FuncName: "MapErr"}, Checker: checker.Conc, CallbackArgIdx: 1},

	// github.com/siketyan/gotask: Do* take tasks or functions after ctx
	{Spec: funcspec.Spec{PkgPath: gotaskPkg, FuncName: "DoAll"}, Checker: checker.Gotask, CallbackArgIdx: 1, Variadic: true},
	{Spec: funcspec.Spec{PkgPath: gotaskPkg, FuncName: "DoAllSettled"}, Checker: checker.Gotask, CallbackArgIdx: 1, Variadic: true},
	{Spec: funcspec.Spec{PkgPath: gotaskPkg, FuncName: "DoRace"}, Checker: checker.Gotask, CallbackArgIdx: 1, Variadic: true},
	{Spec: funcspec.Spec{PkgPath: gotaskPkg, FuncName: "DoAllFns"}, Checker: checker.Gotask, CallbackArgIdx: 1, Variadic: true},
	{Spec: funcspec.Spec{PkgPath: gotaskPkg, FuncName: "DoAllFnsSettled"}, Checker: checker.Gotask, CallbackArgIdx: 1, Variadic: true},
	{Spec: funcspec.Spec{PkgPath: gotaskPkg, FuncName: "DoRaceFns"}, Checker: checker.Gotask, CallbackArgIdx: 1, Variadic: true},
	// DoAsync spawns its receiver task
	{Spec: funcspec.Spec{PkgPath: gotaskPkg, TypeName: "Task", FuncName: "DoAsync"}, Checker: checker.Gotask, AlwaysSpawns: true},
	{Spec: funcspec.Spec{PkgPath: gotaskPkg, TypeName: "CancelableTask", FuncName: "DoAsync"}, Checker: checker.Gotask, AlwaysSpawns: true},
}

// Match gives the entry fn matches, of any checker.
func Match(fn *types.Func) (Entry, bool) {
	for _, e := range entries {
		if e.Spec.Matches(fn) {
			return e, true
		}
	}
	return Entry{}, false
}

// MatchFor gives the entry of the named checker that fn matches.
func MatchFor(name checker.Name, fn *types.Func) (Entry, bool) {
	for _, e := range entries {
		if e.Checker == name && e.Spec.Matches(fn) {
			return e, true
		}
	}
	return Entry{}, false
}
