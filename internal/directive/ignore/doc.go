// Package ignore provides //goroutinectx:ignore directive parsing.
//
// # Overview
//
// The ignore directive suppresses analyzer warnings for specific lines
// or specific checkers.
//
// # Directive Placement
//
// The directive can appear on the line before or the same line:
//
//	//goroutinectx:ignore
//	go func() { ... }()  // Warning suppressed
//
//	go func() { ... }()  //goroutinectx:ignore  // Also works
//
// # Checker-Specific Ignores
//
// Specify checker names to ignore only specific checks:
//
//	//goroutinectx:ignore goroutine
//	go func() { ... }()  // Only goroutine checker ignored
//
//	//goroutinectx:ignore goroutine,errgroup
//	g.Go(func() { ... })  // Both checkers ignored
//
// # Valid Checker Names
//
//	┌─────────────────┬─────────────────────────────────────────────┐
//	│ Name            │ Description                                 │
//	├─────────────────┼─────────────────────────────────────────────┤
//	│ goroutine       │ go statement context propagation            │
//	│ goroutinederive │ go statement deriver function calls         │
//	│ errgroup        │ errgroup.Group.Go callback context          │
//	│ waitgroup       │ sync.WaitGroup.Go callback context          │
//	│ spawner         │ //goroutinectx:spawner function calls       │
//	│ spawnerlabel    │ Spawner label directive validation          │
//	│ gotask          │ gotask library function calls               │
//	└─────────────────┴─────────────────────────────────────────────┘
//
// # Parsing
//
// Use [BuildIgnoreMaps] to parse ignore directives from all files:
//
//	ignoreMaps, skipFiles := ignore.BuildIgnoreMaps(pass, checkerNames)
//
// This returns:
//   - ignoreMaps: Map of filename → line → ignored checkers
//   - skipFiles: Files to skip entirely (//goroutinectx:ignore file)
//
// # Map Structure
//
//	type Map map[int]CheckerSet  // line number → ignored checkers
//
//	type CheckerSet map[CheckerName]bool
//
// # Checking Ignores
//
// Use [Map.ShouldIgnore] to check if a line should be ignored:
//
//	ignoreMap := ignoreMaps[filename]
//	if ignoreMap.ShouldIgnore(lineNum, checkerName) {
//	    return  // Skip this check
//	}
//
// # Unused Ignore Detection
//
// The package tracks which ignore directives are used and reports
// unused ones as warnings:
//
//	//goroutinectx:ignore  // Warning: unused ignore directive
//	normalCode()           // No warning to suppress
package ignore
