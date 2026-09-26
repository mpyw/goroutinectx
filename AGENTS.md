# Repository instructions

goroutinectx is a Go analyzer for context propagation in goroutines. Its checker architecture, extension points, test metadata, and documentation rules are in [implementation notes](design/implementation.md). Read the relevant section before adding a checker or changing diagnostics.

Run `go test ./...` for Go changes and `./test_all.sh` before claiming the full repository gate passes. Update examples and architecture documentation when behavior changes.
