// Package run runs every checker of goroutinectx over one package, with the
// configuration the caller resolved. The analyzer resolves it from its flags,
// and the golangci-lint plugin from its settings.
//
// # Execution Flow
//
//  1. [Run] builds the ignore maps, the spawners, the deriver matcher and the
//     checkers from the [Config]
//  2. scope.Build finds the functions with context parameters
//  3. The inspector walks the AST. For each go statement and call inside such
//     a function, the go statement or call checkers run
//  4. Each checker returns its diagnostics. The runner reports each one that
//     no //goroutinectx:ignore for that checker silences at its own line
//  5. spawnerlabel, when on, checks each function declaration the same way
//  6. Ignores that silenced nothing, and malformed directives, are reported
package run
