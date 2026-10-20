// Package registry holds the table of supported APIs: the functions and
// methods that run a callback, or a task, in a new goroutine.
//
// Each [Entry] gives the API's spec, the checker that checks its calls, the
// index of its callback argument, whether it is variadic, and whether it
// always spawns.
//
// The checkers of errgroup, waitgroup, conc and gotask look up the entry of a
// call with [MatchFor]. The spawnerlabel checker asks [Match] whether a
// function calls any supported API.
//
//	entry, ok := registry.MatchFor(checker.Errgroup, fn)
//	if ok {
//	    arg := call.Args[entry.CallbackArgIdx]
//	}
package registry
