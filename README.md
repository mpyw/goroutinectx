# goroutinectx

> [!NOTE]
> This project was 99% written by AI (Claude Code).

A Go linter that checks goroutine context propagation.

## Overview

`goroutinectx` detects cases where a [`context.Context`](https://pkg.go.dev/context#Context) is available in function parameters but not properly passed to downstream calls that should receive it.

## Installation

This analyzer is designed to be used as a library with [`go/analysis`](https://pkg.go.dev/golang.org/x/tools/go/analysis). To use it, import the analyzer in your own tool:

```go
import "github.com/mpyw/goroutinectx"

func main() {
    singlechecker.Main(goroutinectx.Analyzer)  // singlechecker from go/analysis
}
```

See [`singlechecker`](https://pkg.go.dev/golang.org/x/tools/go/analysis/singlechecker) for details.

Or use it with [`multichecker`](https://pkg.go.dev/golang.org/x/tools/go/analysis/multichecker) alongside other analyzers.

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

### gotask (requires `-goroutine-deriver`)

Detects [gotask](https://github.com/siketyan/gotask) calls where task functions don't call the context deriver. Since tasks run as goroutines, they need to call the deriver function (e.g., `apm.NewGoroutineContext`) inside their body - there's no way to wrap the context at the call site.

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

For `Task.DoAsync` and `CancelableTask.DoAsync`, the context argument must contain a deriver call:

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

### `-context-carriers`

Treat additional types as context carriers (like [`context.Context`](https://pkg.go.dev/context#Context)). Useful for web frameworks that have their own context types.

```bash
# Treat echo.Context as a context carrier
goroutinectx -context-carriers=github.com/labstack/echo/v4.Context ./...

# Multiple carriers (comma-separated)
goroutinectx -context-carriers=github.com/labstack/echo/v4.Context,github.com/urfave/cli/v2.Context ./...
```

When a function has a context carrier parameter, goroutinectx will check that it's properly propagated to goroutines and other APIs.

### Checker Enable/Disable Flags

Most checkers are enabled by default. Use these flags to enable or disable specific checkers:

Available flags:
- `-goroutine` (default: true)
- `-waitgroup` (default: true)
- `-errgroup` (default: true)
- `-spawner` (default: true)
- `-spawnerlabel` (default: false) - Check that spawner functions are properly labeled
- `-gotask` (default: true, requires `-goroutine-deriver`)

### `-spawnerlabel`

When enabled, checks that functions calling spawn methods have the `//goroutinectx:spawner` directive:

```go
// Bad: calls errgroup.Group.Go but missing directive
func runTasks() {  // Warning: should have //goroutinectx:spawner
    g := new(errgroup.Group)
    g.Go(func() error {
        return doWork()
    })
    _ = g.Wait()
}

// Good: properly labeled
//goroutinectx:spawner
func runTasks() {
    g := new(errgroup.Group)
    g.Go(func() error {
        return doWork()
    })
    _ = g.Wait()
}
```

Also warns about unnecessary labels on functions that don't spawn and have no func parameters.

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
