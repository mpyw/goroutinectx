# goroutinectx

[![Go Reference](https://pkg.go.dev/badge/github.com/mpyw/goroutinectx.svg)](https://pkg.go.dev/github.com/mpyw/goroutinectx)
[![Go Report Card](https://goreportcard.com/badge/github.com/mpyw/goroutinectx)](https://goreportcard.com/report/github.com/mpyw/goroutinectx)
[![Codecov](https://codecov.io/gh/mpyw/goroutinectx/graph/badge.svg)](https://codecov.io/gh/mpyw/goroutinectx)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

> [!NOTE]
> This project was 99% written by AI (Claude Code).

A Go linter that checks goroutine context propagation.

## Overview

`goroutinectx` detects cases where a [`context.Context`](https://pkg.go.dev/context#Context) is available in function parameters but not properly passed to downstream calls that should receive it.

## Installation & Usage

### Using [`go vet`](https://pkg.go.dev/cmd/go#hdr-Report_likely_mistakes_in_packages) (Recommended)

```bash
# Install the analyzer
go install github.com/mpyw/goroutinectx/cmd/goroutinectx@latest

# Run with go vet
go vet -vettool=$(which goroutinectx) ./...
```

### Using [`go tool`](https://pkg.go.dev/cmd/go#hdr-Run_specified_go_tool) (Go 1.24+)

```bash
# Add to go.mod as a tool dependency
go get -tool github.com/mpyw/goroutinectx/cmd/goroutinectx@latest

# Run via go tool
go tool goroutinectx ./...
```

### As a Library

```go
import "github.com/mpyw/goroutinectx"

func main() {
    singlechecker.Main(goroutinectx.Analyzer)
}
```

See [`singlechecker`](https://pkg.go.dev/golang.org/x/tools/go/analysis/singlechecker) for details.

Or use it with [`multichecker`](https://pkg.go.dev/golang.org/x/tools/go/analysis/multichecker) alongside other analyzers.

### golangci-lint

Not currently integrated with golangci-lint. PRs welcome if someone wants to add it, but not actively pursuing integration.

## What It Checks

### goroutines

Detects goroutines that don't propagate context:

```go
func handler(ctx context.Context) {
    // Bad: goroutine doesn't use ctx
    go func() {
        doSomething()
    }()

    // Good: goroutine uses ctx
    go func() {
        doSomething(ctx)
    }()
}
```

**Important**: Each goroutine must **directly** reference the context in its own function body. Context usage in nested closures doesn't count:

```go
func handler(ctx context.Context) {
    // Bad: ctx is only used in the nested closure, not in the goroutine itself
    go func() {
        go func() {
            doSomething(ctx)
        }()
    }()

    // Good: goroutine directly references ctx (even if not "using" it)
    go func() {
        _ = ctx  // Explicit acknowledgment of context
        go func() {
            doSomething(ctx)
        }()
    }()
}
```

This design ensures every goroutine explicitly acknowledges context propagation. If your goroutine doesn't need to use context directly but spawns nested goroutines that do, add `_ = ctx` to signal intentional propagation.

### [`errgroup.Group`](https://pkg.go.dev/golang.org/x/sync/errgroup#Group)

Detects [`errgroup.Group.Go`](https://pkg.go.dev/golang.org/x/sync/errgroup#Group.Go) closures that don't use context:

```go
func handler(ctx context.Context) {
    g := new(errgroup.Group)

    // Bad: closure doesn't use ctx
    g.Go(func() error {
        return doSomething()
    })

    // Good: closure uses ctx
    g.Go(func() error {
        return doSomething(ctx)
    })
}
```

### [`sync.WaitGroup`](https://pkg.go.dev/sync#WaitGroup) (Go 1.25+)

Detects [`sync.WaitGroup.Go`](https://pkg.go.dev/sync#WaitGroup.Go) closures that don't use context:

```go
func handler(ctx context.Context) {
    var wg sync.WaitGroup

    // Bad: closure doesn't use ctx
    wg.Go(func() {
        doSomething()
    })

    // Good: closure uses ctx
    wg.Go(func() {
        doSomething(ctx)
    })
}
```

### [gotask](https://pkg.go.dev/github.com/siketyan/gotask/v2) (requires `-goroutine-deriver`)

Detects [gotask](https://pkg.go.dev/github.com/siketyan/gotask/v2) calls where task functions don't call the context deriver. Since tasks run as goroutines, they need to call the deriver function (e.g., `apm.NewGoroutineContext`) inside their body - there's no way to wrap the context at the call site.

Supported functions:
- [`gotask.DoAll`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoAll)
- [`gotask.DoAllFns`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoAllFns)
- [`gotask.DoAllSettled`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoAllSettled)
- [`gotask.DoAllFnsSettled`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoAllFnsSettled)
- [`gotask.DoRace`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoRace)
- [`gotask.DoRaceFns`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoRaceFns)
- [`gotask.Task.DoAsync`](https://pkg.go.dev/github.com/siketyan/gotask/v2#Task.DoAsync)
- [`gotask.CancelableTask.DoAsync`](https://pkg.go.dev/github.com/siketyan/gotask/v2#CancelableTask.DoAsync)

```go
func handler(ctx context.Context) {
    // Bad: task function doesn't call deriver
    _ = gotask.DoAllFnsSettled(
        ctx,
        func(ctx context.Context) error {
            return doSomething(ctx)  // ctx is NOT derived!
        },
    )

    // Good: task function calls deriver
    _ = gotask.DoAllFnsSettled(
        ctx,
        func(ctx context.Context) error {
            ctx = apm.NewGoroutineContext(ctx)  // Properly derived
            return doSomething(ctx)
        },
    )
}
```

For [`gotask.Task.DoAsync`](https://pkg.go.dev/github.com/siketyan/gotask/v2#Task.DoAsync) and [`gotask.CancelableTask.DoAsync`](https://pkg.go.dev/github.com/siketyan/gotask/v2#CancelableTask.DoAsync), the context argument must contain a deriver call:

```go
func handler(ctx context.Context) {
    task := gotask.NewTask(func(ctx context.Context) error {
        return nil
    })

    // Bad: ctx is not derived
    task.DoAsync(ctx, errChan)

    // Good: ctx is derived
    task.DoAsync(apm.NewGoroutineContext(ctx), errChan)
}
```

**Note**: This checker only activates when `-goroutine-deriver` is set.

## Directives

### `//goroutinectx:ignore`

Suppress warnings for a specific line:

```go
func handler(ctx context.Context) {
    //goroutinectx:ignore - intentionally not passing context
    go func() {
        backgroundTask()
    }()
}
```

The comment can be on the same line or the line above.

#### Checker-Specific Ignore

You can specify which checker(s) to ignore:

```go
func handler(ctx context.Context) {
    //goroutinectx:ignore goroutine - only ignore goroutine checker
    go func() {
        backgroundTask()
    }()

    //goroutinectx:ignore goroutine,errgroup - ignore multiple checkers
    g.Go(func() error {
        return backgroundTask()
    })
}
```

**Available checker names:**
- `goroutine` - `go func()` statements
- `goroutinederive` - goroutine derive function requirement
- `waitgroup` - `sync.WaitGroup.Go()` calls
- `errgroup` - `errgroup.Group.Go()` calls
- `spawner` - spawner directive checks
- `spawnerlabel` - spawner label requirement
- `gotask` - gotask library checks

#### Unused Ignore Detection

The analyzer reports unused `//goroutinectx:ignore` directives. If an ignore directive doesn't suppress any warning, it will be flagged as unused. This helps keep your codebase clean from stale ignore comments.

### `//goroutinectx:spawner`

Mark a function as one that spawns goroutines with its func arguments. The analyzer will check that func arguments passed to marked functions properly use context:

```go
//goroutinectx:spawner
func runAsync(g *errgroup.Group, fn func() error) {
    g.Go(fn)
}

func handler(ctx context.Context) {
    g := new(errgroup.Group)

    // Bad: func argument doesn't use ctx
    runAsync(g, func() error {
        return doSomething()
    })

    // Good: func argument uses ctx
    runAsync(g, func() error {
        return doSomething(ctx)
    })
}
```

This is useful for wrapper functions that abstract away goroutine spawning patterns.

## Flags

> [!TIP]
> When using `go vet -vettool`, pass flags directly:
> ```bash
> go vet -vettool=$(which goroutinectx) -goroutine-deriver='pkg.Func' ./...
> ```

### `-goroutine-deriver`

Require goroutines to call a specific function to derive context. Useful for APM libraries like New Relic.

```bash
# Single deriver - require apm.NewGoroutineContext() in goroutines
goroutinectx -goroutine-deriver='github.com/my-example-app/telemetry/apm.NewGoroutineContext' ./...

# AND (plus) - require BOTH txn.NewGoroutine() AND newrelic.NewContext()
goroutinectx -goroutine-deriver='github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+github.com/newrelic/go-agent/v3/newrelic.NewContext' ./...

# Mixed AND/OR - (txn.NewGoroutine AND NewContext) OR apm.NewGoroutineContext
goroutinectx -goroutine-deriver='github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+github.com/newrelic/go-agent/v3/newrelic.NewContext,github.com/my-example-app/telemetry/apm.NewGoroutineContext' ./...
```

**Format:**
- `pkg/path.Func` for functions
- `pkg/path.Type.Method` for methods
- `,` (comma) for OR - at least one group must be satisfied
- `+` (plus) for AND - all functions in the group must be called

> [!TIP]
> When both parent and child goroutines require instrumentation inheritance (e.g., [New Relic Go Agent](https://pkg.go.dev/github.com/newrelic/go-agent/v3/newrelic)), you need to call [`Transaction.NewGoroutine`](https://pkg.go.dev/github.com/newrelic/go-agent/v3/newrelic#Transaction.NewGoroutine) and [`NewContext`](https://pkg.go.dev/github.com/newrelic/go-agent/v3/newrelic#NewContext):
>
> ```go
> go func() {
>     txn := newrelic.FromContext(ctx).NewGoroutine()
>     ctx := newrelic.NewContext(context.Background(), txn)
>     doSomething(ctx)
> }()
> ```
>
> It's common to define a wrapper function that takes [`context.Context`](https://pkg.go.dev/context#Context) and returns [`context.Context`](https://pkg.go.dev/context#Context):
>
> ```go
> // apm.NewGoroutineContext derives context for goroutine instrumentation
> func NewGoroutineContext(ctx context.Context) context.Context {
>     txn := newrelic.FromContext(ctx)
>     if txn == nil {
>         return ctx
>     }
>     return newrelic.NewContext(ctx, txn.NewGoroutine())
> }
> ```
>
> See also: [New Relic Go Agent 完全理解・実践導入ガイド - Zenn (in Japanese)](https://zenn.dev/mpyw/articles/new-relic-go-agent-struggle)

### `-context-carriers`

Treat additional types as context carriers (like [`context.Context`](https://pkg.go.dev/context#Context)). Useful for web frameworks that have their own context types.

```bash
# Treat echo.Context as a context carrier
goroutinectx -context-carriers=github.com/labstack/echo/v4.Context ./...

# Multiple carriers (comma-separated)
goroutinectx -context-carriers=github.com/labstack/echo/v4.Context,github.com/urfave/cli/v2.Context ./...
```

When a function has a context carrier parameter, goroutinectx will check that it's properly propagated to goroutines and other APIs.

### `-external-spawner`

Mark external package functions as spawners. This is the flag-based alternative to `//goroutinectx:spawner` directive for functions you don't control.

```bash
# Single external spawner
goroutinectx -external-spawner='github.com/example/workerpool.Pool.Submit' ./...

# Multiple external spawners (comma-separated)
goroutinectx -external-spawner='github.com/example/workerpool.Pool.Submit,github.com/example/workerpool.Run' ./...
```

**Format:**
- `pkg/path.Func` for package-level functions
- `pkg/path.Type.Method` for methods

When an external spawner is called, goroutinectx checks that func arguments properly use context.

### Checker Enable/Disable Flags

Most checkers are enabled by default. Use these flags to enable or disable specific checkers:

Available flags:
- `-goroutine` (default: true)
- `-waitgroup` (default: true)
- `-errgroup` (default: true)
- `-spawner` (default: true)
- `-spawnerlabel` (default: false) - Check that spawner functions are properly labeled
- `-gotask` (default: true, requires `-goroutine-deriver`)

### File Filtering

| Flag | Default | Description |
|------|---------|-------------|
| `-test` | `true` | Analyze test files (`*_test.go`) |

Generated files (containing `// Code generated ... DO NOT EDIT.`) are always excluded and cannot be opted in.

```bash
# Exclude test files from analysis
goroutinectx -test=false ./...

# With go vet
go vet -vettool=$(which goroutinectx) -goroutinectx.test=false ./...
```

### `-spawnerlabel`

When enabled, checks that functions calling spawn methods with func arguments have the `//goroutinectx:spawner` directive:

```go
// Bad: calls errgroup.Group.Go with func argument but missing directive
func runTask(task func() error) {  // Warning: should have //goroutinectx:spawner
    g := new(errgroup.Group)
    g.Go(task)
    _ = g.Wait()
}

// Good: properly labeled
//goroutinectx:spawner
func runTask(task func() error) {
    g := new(errgroup.Group)
    g.Go(task)
    _ = g.Wait()
}
```

Also warns about unnecessary labels on functions that don't spawn and have no func parameters:

```go
// Bad: unnecessary directive (no spawn calls, no func parameters)
//goroutinectx:spawner
func simpleHelper() {  // Warning: unnecessary //goroutinectx:spawner
    fmt.Println("hello")
}
```

## Design Principles

1. **Zero false positives** - Prefer missing issues over false alarms
2. **Type-safe analysis** - Uses [`go/types`](https://pkg.go.dev/go/types) for accurate detection
3. **Nested function support** - Correctly tracks context through closures

## Related Tools

- [contextcheck](https://github.com/kkHAIKE/contextcheck) - Detects [`context.Background`](https://pkg.go.dev/context#Background)/[`context.TODO`](https://pkg.go.dev/context#TODO) usage and missing context parameters

`goroutinectx` is complementary to `contextcheck`:
- `contextcheck` warns about creating new contexts when one should be propagated
- `goroutinectx` warns about not using an available context in specific APIs

## License

MIT
