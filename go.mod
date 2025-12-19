module github.com/mpyw/goroutinectx

go 1.24.0

// Retract all previous versions due to a bug where -goroutine-deriver flag
// incorrectly disabled the base goroutine checker instead of running both independently.
retract [v0.1.1, v0.2.0]

require golang.org/x/tools v0.40.0

require (
	golang.org/x/mod v0.31.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
)
