// Package ignore provides //goroutinectx:ignore directive parsing.
//
// # Overview
//
// The ignore directive suppresses analyzer warnings for specific lines
// or specific checkers.
//
// # Directive Placement
//
// The directive can appear on the line before or the same line. One that
// follows code covers its own line only:
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
//	│ conc            │ sourcegraph/conc callback context           │
//	│ spawner         │ //goroutinectx:spawner function calls       │
//	│ spawnerlabel    │ Spawner label directive validation          │
//	│ gotask          │ gotask library function calls               │
//	└─────────────────┴─────────────────────────────────────────────┘
//
// # Parsing
//
// [directive.Scan] finds the ignore directives of a file, and [Build] makes
// its map: line number → ignore. Scan drops, and reports, each ignore that
// names a checker goroutinectx does not have, which silences nothing.
//
//	ignoreMap := ignore.Build(fset, file, directive.Scan(file).Ignores)
//
// # Checking Ignores
//
// [Map.Covering] gives the ignore that silences a checker's reports on a
// line, and marks nothing. The caller marks the ignore that silenced a
// report with [Entry.MarkUsed]:
//
//	if e := ignoreMap.Covering(line, checkerName); e != nil {
//	    e.MarkUsed(checkerName)
//	    return  // Skip this report
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
