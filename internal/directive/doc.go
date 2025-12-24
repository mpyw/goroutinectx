// Package directive provides directive parsing for goroutinectx.
//
// # Overview
//
// This package contains subpackages for parsing comment directives
// that control analyzer behavior:
//
//	directive/
//	├── carrier/   # Context carrier type configuration
//	├── ignore/    # //goroutinectx:ignore directive
//	└── spawner/   # //goroutinectx:spawner directive
//
// # Directive Format
//
// All directives follow the format:
//
//	//goroutinectx:<directive> [args]
//
// Examples:
//
//	//goroutinectx:ignore
//	//goroutinectx:ignore goroutine
//	//goroutinectx:ignore goroutine,errgroup
//	//goroutinectx:spawner
//
// # Carrier Directive
//
// Context carrier types are configured via flag, not directive:
//
//	-carrier=github.com/labstack/echo/v4.Context
//
// See [carrier] package for details.
//
// # Ignore Directive
//
// Suppresses warnings for the next line or same line:
//
//	//goroutinectx:ignore
//	go func() { ... }()  // No warning
//
//	go func() { ... }()  //goroutinectx:ignore  // Same line works too
//
// Checker-specific ignores:
//
//	//goroutinectx:ignore goroutine
//	go func() { ... }()  // Only goroutine checker ignored
//
// See [ignore] package for details.
//
// # Spawner Directive
//
// Marks a function as spawning goroutines with its func arguments:
//
//	//goroutinectx:spawner
//	func runWorkers(tasks ...func()) {
//	    for _, task := range tasks {
//	        go task()
//	    }
//	}
//
// See [spawner] package for details.
package directive
