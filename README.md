# goroutinectx

> [!CAUTION]
> This project is under heavy construction. The repository will be renamed from `ctxrelay` to `goroutinectx` before the first release. See [TASK_ARCH_REFACTOR.md](./TASK_ARCH_REFACTOR.md) for details.

> [!NOTE]
> This project was 99% written by AI (Claude Code).

A Go linter that checks goroutine context propagation.

## Overview

`ctxrelay` detects cases where a `context.Context` is available in function parameters but not properly passed to downstream calls that should receive it.

## Installation

```bash
go install github.com/mpyw/ctxrelay/cmd/ctxrelay@latest
```

## Usage

### Standalone

```bash
ctxrelay ./...
```

### With go vet

```bash
go vet -vettool=$(which ctxrelay) ./...
```

## What It Checks

### zerolog

Detects zerolog logging chains missing `.Ctx(ctx)`:

```go
func handler(ctx context.Context, log zerolog.Logger) {
    // Bad: missing .Ctx(ctx)
    log.Info().Str("key", "value").Msg("hello")

    // Good: includes .Ctx(ctx)
    log.Info().Ctx(ctx).Str("key", "value").Msg("hello")
}
```

### slog

Detects slog calls that should use context-aware variants:

```go
func handler(ctx context.Context) {
    // Bad: use InfoContext instead
    slog.Info("hello")

    // Good: uses context
    slog.InfoContext(ctx, "hello")
}
```

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

### errgroup.Group

Detects `errgroup.Group.Go()` closures that don't use context:

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

### sync.WaitGroup (Go 1.25+)

Detects `sync.WaitGroup.Go()` closures that don't use context:

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

### `//ctxrelay:ignore`

Suppress warnings for a specific line:

```go
func handler(ctx context.Context) {
    //ctxrelay:ignore - intentionally not passing context
    go func() {
        backgroundTask()
    }()
}
```

The comment can be on the same line or the line above.

### `//ctxrelay:goroutine_creator`

Mark a function as one that spawns goroutines with its func arguments. The analyzer will check that func arguments passed to marked functions properly use context:

```go
//ctxrelay:goroutine_creator
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
ctxrelay -goroutine-deriver=github.com/my-example-app/telemetry/apm.NewGoroutineContext ./...

# AND (plus) - require BOTH txn.NewGoroutine() AND newrelic.NewContext()
ctxrelay -goroutine-deriver='github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+github.com/newrelic/go-agent/v3/newrelic.NewContext' ./...

# Mixed AND/OR - (txn.NewGoroutine AND NewContext) OR apm.NewGoroutineContext
ctxrelay -goroutine-deriver='github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+github.com/newrelic/go-agent/v3/newrelic.NewContext,github.com/my-example-app/telemetry/apm.NewGoroutineContext' ./...
```

**Format:**
- `pkg/path.Func` for functions
- `pkg/path.Type.Method` for methods
- `,` (comma) for OR - at least one group must be satisfied
- `+` (plus) for AND - all functions in the group must be called

### `-context-carriers`

Treat additional types as context carriers (like `context.Context`). Useful for web frameworks that have their own context types.

```bash
# Treat echo.Context as a context carrier
ctxrelay -context-carriers=github.com/labstack/echo/v4.Context ./...

# Multiple carriers (comma-separated)
ctxrelay -context-carriers=github.com/labstack/echo/v4.Context,github.com/urfave/cli/v2.Context ./...
```

When a function has a context carrier parameter, ctxrelay will check that it's properly propagated to goroutines and other APIs.

**Note**: This flag applies to AST-based checkers (slog, goroutine, errgroup, waitgroup). zerolog analysis only checks `context.Context` because `zerolog.Ctx()` only accepts `context.Context`.

### Checker Enable/Disable Flags

All checkers are enabled by default. Use these flags to disable specific checkers:

```bash
# Disable specific checkers
ctxrelay -slog=false -zerolog=false ./...

# Run only goroutine-related checks
ctxrelay -slog=false -zerolog=false ./...
```

Available flags:
- `-slog` (default: true)
- `-zerolog` (default: true)
- `-goroutine` (default: true)
- `-errgroup` (default: true)
- `-waitgroup` (default: true)
- `-goroutine-creator` (default: true)
- `-gotask` (default: true, requires `-goroutine-deriver`)

## Design Principles

1. **Zero false positives** - Prefer missing issues over false alarms
2. **Type-safe analysis** - Uses `go/types` for accurate detection
3. **Nested function support** - Correctly tracks context through closures

## Related Tools

- [contextcheck](https://github.com/kkHAIKE/contextcheck) - Detects `context.Background()`/`context.TODO()` usage and missing context parameters

`ctxrelay` is complementary to `contextcheck`:
- `contextcheck` warns about creating new contexts when one should be propagated
- `ctxrelay` warns about not using an available context in specific APIs

## License

MIT
