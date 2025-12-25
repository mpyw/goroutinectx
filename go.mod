module github.com/mpyw/goroutinectx

go 1.24.0

// Retract all previous versions due to:
// - v0.1.0-v0.2.0: -goroutine-deriver flag incorrectly disabled the base goroutine checker
// - v0.1.0-v0.4.0: -test flag conflicted with singlechecker's built-in flag
retract [v0.1.0, v0.4.0]

// Retract v0.7.2-v0.7.3 due to:
// - -goroutine-deriver flag didn't work with errgroup/waitgroup/conc/spawner checkers
//   (deriver checking logic was lost during refactoring)
retract [v0.7.2, v0.7.3]

require golang.org/x/tools v0.40.0

require (
	golang.org/x/mod v0.31.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
)
