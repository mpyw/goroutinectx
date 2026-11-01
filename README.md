# goroutinectx

[![Go Reference](https://pkg.go.dev/badge/github.com/mpyw/goroutinectx.svg)](https://pkg.go.dev/github.com/mpyw/goroutinectx)
[![CI](https://github.com/mpyw/goroutinectx/actions/workflows/ci.yml/badge.svg)](https://github.com/mpyw/goroutinectx/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/gh/mpyw/goroutinectx/graph/badge.svg)](https://codecov.io/gh/mpyw/goroutinectx)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A Go linter that checks goroutine context propagation.

## Overview

`goroutinectx` detects cases where a [`context.Context`](https://pkg.go.dev/context#Context) is available in function parameters but not properly passed to downstream calls that should receive it.

A parameter is a context when its type is one of these:

- `context.Context`
- An alias of it, such as `type Ctx = context.Context` or [`golang.org/x/net/context.Context`](https://pkg.go.dev/golang.org/x/net/context#Context)
- An interface that embeds it, named or not: `type MyCtx interface{ context.Context }` or `interface{ context.Context; Name() string }`
- A type parameter whose constraint is one of the above: `[C context.Context]` or `[C MyCtx]`

Only an embedded `context.Context` counts. An interface that declares some or all of its methods itself is not a context.

## Installation & Usage

### <a href="https://mise.jdx.dev/"><img src="https://mise.jdx.dev/logo.svg" height="28" alt=""></a> Using [mise](https://mise.jdx.dev/) (macOS/Linux/Windows)

**Recommended.** goroutinectx is installable directly from GitHub Releases via mise's `github` backend — no extra registry required, and no Go toolchain needed because the binaries are prebuilt:

```bash
mise use "github:mpyw/goroutinectx@0.14.1"
goroutinectx ./...
```

Run it in the project root. It pins the version in the project's `mise.toml`, so every checkout and CI run the same one:

```toml
[tools]
"github:mpyw/goroutinectx" = "0.14.1"
```

Add `-g` to install it for every project on your machine instead.

> [!IMPORTANT]
> Building goroutinectx from source requires Go 1.27 or later.

### Using [`go tool`](https://pkg.go.dev/cmd/go#hdr-Run_specified_go_tool)

```bash
# Add to go.mod as a tool dependency
go get -tool github.com/mpyw/goroutinectx/cmd/goroutinectx@latest

# Run via go tool
go tool goroutinectx ./...
```

### Using [`go install`](https://pkg.go.dev/cmd/go#hdr-Compile_and_install_packages_and_dependencies)

```bash
go install github.com/mpyw/goroutinectx/cmd/goroutinectx@latest
goroutinectx ./...
```

### Using [`go run`](https://pkg.go.dev/cmd/go#hdr-Compile_and_run_Go_program)

```bash
go run github.com/mpyw/goroutinectx/cmd/goroutinectx@latest ./...
```

> [!CAUTION]
> To prevent supply chain attacks, pin to a specific version tag instead of `@latest` in CI/CD pipelines (e.g., `@v0.14.1`).

<details>
<summary><a href="https://curl.se/"><img src="https://cdn.simpleicons.org/curl" height="20" alt=""></a> Downloading the tarball directly (macOS/Linux/Windows)</summary>

No package manager? Grab the archive for your platform from [GitHub Releases](https://github.com/mpyw/goroutinectx/releases):

```bash
export VERSION=0.14.1
export OS=linux    # or darwin
export ARCH=amd64  # or arm64
export BASE_URL="https://github.com/mpyw/goroutinectx/releases/download/v${VERSION}"

# Download the archive and the release's checksum list
curl -LO "${BASE_URL}/goroutinectx_${VERSION}_${OS}_${ARCH}.tar.gz"
curl -LO "${BASE_URL}/checksums.txt"

# Verify before installing (use `shasum -a 256 -c` on macOS)
sha256sum --ignore-missing -c checksums.txt

tar xzf "goroutinectx_${VERSION}_${OS}_${ARCH}.tar.gz"
sudo mv goroutinectx /usr/local/bin/
```

On Windows, download `goroutinectx_${VERSION}_windows_${ARCH}.zip` and extract `goroutinectx.exe` somewhere on your `PATH`.

</details>

### As a Library

```go
import "github.com/mpyw/goroutinectx"

func main() {
    singlechecker.Main(goroutinectx.Analyzer)
}
```

See [`singlechecker`](https://pkg.go.dev/golang.org/x/tools/go/analysis/singlechecker) for details.

Or use it with [`multichecker`](https://pkg.go.dev/golang.org/x/tools/go/analysis/multichecker) alongside other analyzers.

<details>
<summary>Using <a href="https://golangci-lint.run/">golangci-lint</a> (module plugin)</summary>

goroutinectx is not bundled with golangci-lint. Build a binary that holds it as a [module plugin](https://golangci-lint.run/plugins/module-plugins/). Write `.custom-gcl.yml`:

```yaml
version: v2.15.0  # the golangci-lint release to build
plugins:
  - module: github.com/mpyw/goroutinectx
    import: github.com/mpyw/goroutinectx/plugin
    version: v0.14.1
```

Turn it on in `.golangci.yml`. The keys under `settings` are the names of the [flags](#flags). `external-spawner` and `context-carriers` are lists here, not comma-separated strings. A key left out keeps its default. An unknown key stops the run:

```yaml
version: "2"
linters:
  enable:
    - goroutinectx
  settings:
    custom:
      goroutinectx:
        type: module
        description: Checks goroutine context propagation.
        settings:
          goroutine-deriver: github.com/my-example-app/telemetry/apm.NewGoroutineContext
          context-carriers:
            - github.com/labstack/echo/v4.Context
          external-spawner:
            - github.com/example/workerpool.Pool.Submit
          spawnerlabel: true
```

Then build and run it:

```bash
golangci-lint custom  # writes ./custom-gcl
./custom-gcl run ./...
```

From a Go program of your own, build an analyzer with `goroutinectx.New`. It takes the same settings as a `goroutinectx.Config`, and has no flags. `goroutinectx.Analyzer` is the one with flags:

```go
a, err := goroutinectx.New(goroutinectx.Config{
	ContextCarriers: []string{"github.com/labstack/echo/v4.Context"},
	Checkers:        map[string]bool{"spawnerlabel": true},
})
```

</details>

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

A goroutine passes when its function captures the context, in its own body or in any closure inside it. Each `go` statement is checked on its own, so the inner goroutine below must use the context too:

```go
func handler(ctx context.Context) {
    // Good: the deferred closure inside the goroutine uses ctx
    go func() {
        defer func() {
            doSomething(ctx)
        }()
    }()

    // Good: the outer goroutine captures ctx through the inner one
    go func() {
        go func() {
            doSomething(ctx)
        }()
    }()

    // Bad: the inner goroutine does not use ctx
    go func() {
        _ = ctx
        go func() {
            doSomething()
        }()
    }()
}
```

A context made with [`context.Background`](https://pkg.go.dev/context#Background) or [`context.TODO`](https://pkg.go.dev/context#TODO) does not count, wherever the goroutine gets it.

A context the goroutine reads from a field or an element of a value it captures counts. So does a value whose type implements `context.Context`, such as a struct that embeds one. A method called on a captured value counts too when it returns a context, as `req.Context()` does. A call that takes a captured value that holds a context your code can have put there counts too, whether the value is the receiver, as in `s.run()`, or an argument, as in `handle(s)`. The callee is not looked into, the same as for `go s.run()` and `go handle(s)`. A context field counts when it is exported, or when it is declared in your package or another package of your module. When the driver does not tell which module, a field of any package outside the standard library counts. So the unexported context of a `*testing.T` or an `*http.Request` does not count, and `go func() { t.Log("x") }()` is reported.

A goroutine that takes a signal of the context, not the context itself, passes too. The signal can be captured or given as an argument, to a function literal or a named function. It is one of these:

- The channel of `ctx.Done()`
- The cancel func of a context made from `ctx`, by `context.WithCancel`, `WithCancelCause`, `WithTimeout`, `WithDeadline`, `WithTimeoutCause` or `WithDeadlineCause`

A signal of a context made from `context.Background()` or `TODO()` does not count. A channel that can't be followed, such as a parameter, does not count either:

```go
func handler(ctx context.Context) {
    // Good: the goroutine ends when ctx is done
    done := ctx.Done()
    go func() {
        <-done
        cleanup()
    }()

    // Good: the goroutine cancels a context made from ctx
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()
    go func() {
        waitSignal()
        cancel()
    }()

    // Good: worker is given ctx.Done() along with a fresh context
    go worker(context.Background(), ctx.Done())

    // Bad: the context it cancels is not made from ctx
    _, stop := context.WithCancel(context.Background())
    go func() {
        stop()
    }()
}
```

A func value the goroutine calls is judged as if it were spawned directly, when the goroutine gets it from a variable it captures. A func that can't be traced passes, in both cases. Examples are a parameter, a field of a parameter, a slice element read in a loop, and a result of a call that returns several values. A func returned by a factory that is given the context passes too:

```go
func handler(ctx context.Context, f func()) {
    // Good: f can't be traced, the same as for go f()
    go func() {
        f()
    }()

    // Good: serve is given ctx, the same as for go run()
    run, _ := serve(ctx)
    go func() {
        _ = run()
    }()

    // Bad: work is followed to a closure that does not use ctx
    work := func() { doSomething() }
    go func() {
        work()
    }()
}
```

#### Goroutines that only wait and signal

A goroutine that only waits and then tells the caller is not reported. It has no use for `ctx`, as the caller selects on `ctx.Done()` itself:

```go
func run(ctx context.Context) error {
    var wg sync.WaitGroup
    // ... start workers with wg ...

    // Good: the goroutine only waits and signals
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

The shape is narrow. The function makes at least one of these waits:

- [`sync.WaitGroup.Wait`](https://pkg.go.dev/sync#WaitGroup.Wait)
- [`errgroup.Group.Wait`](https://pkg.go.dev/golang.org/x/sync/errgroup#Group.Wait)
- [`exec.Cmd.Wait`](https://pkg.go.dev/os/exec#Cmd.Wait), [`os.Process.Wait`](https://pkg.go.dev/os#Process.Wait)
- `Wait` of [conc](https://pkg.go.dev/github.com/sourcegraph/conc): `conc.WaitGroup`, the `pool` types and `stream.Stream`

Besides the waits, it only does these:

| It may | Note |
| --- | --- |
| Close a channel | Also in `defer` |
| Send on a channel | Only when the send cannot block. Either the send is a case of a `select` with a `default`, or all of these hold: it is the goroutine's only send outside a `select`, the channel is made with `make(chan T, n)` for a constant `n` of at least 1, and there are at most `n` sends on the channel in all, counting every goroutine and closure, none in a loop |
| Assign to a variable | As `err = cmd.Wait()` |
| Return | As `return inner.Wait()` |
| Branch | As `if err := cmd.Wait(); err != nil { ... }`. A loop is not allowed |

Anything else is reported as before: a call that is not a wait, as a log line after the wait, a loop, a server's `Serve`, and a goroutine that receives from a channel. These may leak when ctx is cancelled:

- **A receive** can wait for ever when nothing is sent. The goroutine could select on `ctx.Done()` instead.
- **A send on an unbuffered channel** blocks for ever when the caller has returned on `ctx.Done()`. So does a send on a channel that can't be followed, such as a parameter.
- **More senders than slots** block the same way. Two goroutines that send on a `make(chan error, 1)`, one goroutine that sends twice, or goroutines started in a loop all leave a send waiting after the first receive. A sender counts wherever it is: in a `go` statement, a callback, or a closure. A channel given to a call is not followed, so its sends are reported.

The rule holds for a `go` statement and for the callback of `errgroup`, `sync.WaitGroup.Go`, conc and spawners. It does not change `-goroutine-deriver`: a goroutine and a callback must still call the deriver, or a callback capture `ctx`. Set [`-report-waiters`](#-report-waiters) to report these goroutines too.

### [`errgroup.Group`](https://pkg.go.dev/golang.org/x/sync/errgroup#Group)

Detects [`errgroup.Group.Go`](https://pkg.go.dev/golang.org/x/sync/errgroup#Group.Go) and [`errgroup.Group.TryGo`](https://pkg.go.dev/golang.org/x/sync/errgroup#Group.TryGo) closures that don't use context:

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

### [conc](https://pkg.go.dev/github.com/sourcegraph/conc)

Detects [conc](https://pkg.go.dev/github.com/sourcegraph/conc) API calls where closures don't use context:

```go
func handler(ctx context.Context) {
    p := pool.New()

    // Bad: closure doesn't use ctx
    p.Go(func() {
        doSomething()
    })

    // Good: closure uses ctx
    p.Go(func() {
        doSomething(ctx)
    })
}
```

Supported APIs:
- [`conc.WaitGroup.Go`](https://pkg.go.dev/github.com/sourcegraph/conc#WaitGroup.Go)
- [`pool.Pool.Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#Pool.Go), [`pool.ResultPool[T].Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ResultPool.Go), [`pool.ContextPool.Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ContextPool.Go), [`pool.ResultContextPool[T].Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ResultContextPool.Go)
- [`pool.ErrorPool.Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ErrorPool.Go), [`pool.ResultErrorPool[T].Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ResultErrorPool.Go)
- [`stream.Stream.Go`](https://pkg.go.dev/github.com/sourcegraph/conc/stream#Stream.Go)
- [`iter.ForEach`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#ForEach), [`iter.ForEachIdx`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#ForEachIdx), [`iter.Map`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Map), [`iter.MapErr`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#MapErr)
- [`iter.Iterator.ForEach`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Iterator.ForEach), [`iter.Iterator.ForEachIdx`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Iterator.ForEachIdx)
- [`iter.Mapper.Map`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Mapper.Map), [`iter.Mapper.MapErr`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Mapper.MapErr)

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
- `gotask.CancelableTask.DoAsync`, the [`Task.DoAsync`](https://pkg.go.dev/github.com/siketyan/gotask/v2#Task.DoAsync) a [`CancelableTask`](https://pkg.go.dev/github.com/siketyan/gotask/v2#CancelableTask) embeds

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

For [`gotask.Task.DoAsync`](https://pkg.go.dev/github.com/siketyan/gotask/v2#Task.DoAsync), on a `Task` or a `CancelableTask`, the context argument must be derived: a deriver call, or a context `context.With*` makes from one. A task whose own function calls the deriver passes too:

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

A task built by a combinator is judged by what it is given:

| Combinator | Judged by |
|------------|-----------|
| [`gotask.Parallel`](https://pkg.go.dev/github.com/siketyan/gotask/v2#Parallel), [`gotask.ParallelSettled`](https://pkg.go.dev/github.com/siketyan/gotask/v2#ParallelSettled), [`gotask.Race`](https://pkg.go.dev/github.com/siketyan/gotask/v2#Race) | Passes when every task calls the deriver. Each task runs in a goroutine of its own |
| [`gotask.Then`](https://pkg.go.dev/github.com/siketyan/gotask/v2#Then), [`gotask.Catch`](https://pkg.go.dev/github.com/siketyan/gotask/v2#Catch) | Passes when the task or the function calls the deriver. Both run in one goroutine, so one call is enough, as in a `go` statement |

```go
func handler(ctx context.Context) {
    bad := gotask.NewTask(func(ctx context.Context) gotask.Result[int] {
        return gotask.ResultOk(1)
    })

    // Bad: Parallel runs bad without the deriver
    _ = gotask.DoAll(ctx, gotask.Parallel(bad))
}
```

> [!NOTE]
> A combinator given a slice of tasks whose elements can't be traced, as a slice parameter (`gotask.Parallel(tasks...)`), is not reported.

**Note**: This checker only activates when `-goroutine-deriver` is set.

## Directives

Only `//goroutinectx:name` is a directive: a line comment, a lowercase name, and no spaces. A reason goes after `//`. A reason after ` - ` is also accepted, for compatibility. Any other comment that starts with `goroutinectx:` does nothing and is reported, as is an unknown directive or checker name.

### `//goroutinectx:ignore`

Suppress warnings for a specific line:

```go
func handler(ctx context.Context) {
    //goroutinectx:ignore // intentionally not passing context
    go func() {
        backgroundTask()
    }()
}
```

The comment can be on the same line or the line above. A comment after code on its line covers that line only. For a call that spans several lines, an ignore above the call covers every argument, and an ignore above one argument covers that argument.

#### Checker-Specific Ignore

You can specify which checker(s) to ignore:

```go
func handler(ctx context.Context) {
    //goroutinectx:ignore goroutine // only ignore goroutine checker
    go func() {
        backgroundTask()
    }()

    //goroutinectx:ignore goroutine,errgroup // ignore multiple checkers
    g.Go(func() error {
        return backgroundTask()
    })
}
```

**Available checker names:**
- `goroutine` - `go func()` statements
- `goroutinederive` - goroutine derive function requirement
- `waitgroup` - [`sync.WaitGroup.Go`](https://pkg.go.dev/sync#WaitGroup.Go) calls
- `errgroup` - [`errgroup.Group.Go`](https://pkg.go.dev/golang.org/x/sync/errgroup#Group.Go) calls
- `conc` - [conc](https://pkg.go.dev/github.com/sourcegraph/conc) library checks
- `spawner` - spawner directive checks
- `spawnerlabel` - spawner label requirement
- `gotask` - [gotask](https://pkg.go.dev/github.com/siketyan/gotask/v2) library checks

#### Unused Ignore Detection

An ignore that silences no report is reported as unused.

### `//goroutinectx:spawner`

Mark a function as one that spawns goroutines with its func arguments. The analyzer will check that func arguments passed to marked functions properly use context.

The directive goes anywhere in the function's doc comment. Anywhere else, it has no effect and is reported. An exported spawner is also a spawner for calls from other packages, as long as goroutinectx analyzes the package that declares it (as `go vet` and golangci-lint do).

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

Name each function with the full import path of its package. A short name such as `apm.NewGoroutineContext` would never match, so it is rejected (standard library packages such as `context` are fine). The same holds for `-external-spawner` and `-context-carriers`.

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
goroutinectx -context-carriers='github.com/labstack/echo/v4.Context' ./...

# Multiple carriers (comma-separated)
goroutinectx -context-carriers='github.com/labstack/echo/v4.Context,github.com/urfave/cli/v2.Context' ./...
```

Name each type with the full import path of its package, as for `-goroutine-deriver`.

Example with [`echo.Context`](https://pkg.go.dev/github.com/labstack/echo/v4#Context) and [`cli.Context`](https://pkg.go.dev/github.com/urfave/cli/v2#Context).

When a function has a context carrier parameter, goroutinectx will check that it's properly propagated to goroutines and other APIs. An alias of a carrier, an interface that embeds one, and a type parameter constrained by one, count as the carrier, the same as for `context.Context`.

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

### `-report-waiters`

Also report goroutines that only wait and signal (default: false). See [Goroutines that only wait and signal](#goroutines-that-only-wait-and-signal).

```bash
goroutinectx -report-waiters ./...
```

In the golangci-lint plugin, set `report-waiters: true`.

### Checker Enable/Disable Flags

Most checkers are enabled by default. Use these flags to enable or disable specific checkers:

Available flags:
- `-goroutine` (default: true)
- `-waitgroup` (default: true)
- `-errgroup` (default: true)
- `-conc` (default: true) - Check [conc](https://pkg.go.dev/github.com/sourcegraph/conc) APIs:
  - [`conc.WaitGroup.Go`](https://pkg.go.dev/github.com/sourcegraph/conc#WaitGroup.Go)
  - [`pool.Pool.Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#Pool.Go), [`pool.ResultPool[T].Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ResultPool.Go), [`pool.ContextPool.Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ContextPool.Go), [`pool.ResultContextPool[T].Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ResultContextPool.Go)
  - [`pool.ErrorPool.Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ErrorPool.Go), [`pool.ResultErrorPool[T].Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ResultErrorPool.Go)
  - [`stream.Stream.Go`](https://pkg.go.dev/github.com/sourcegraph/conc/stream#Stream.Go)
  - [`iter.ForEach`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#ForEach), [`iter.ForEachIdx`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#ForEachIdx), [`iter.Map`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Map), [`iter.MapErr`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#MapErr)
  - [`iter.Iterator.ForEach`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Iterator.ForEach), [`iter.Iterator.ForEachIdx`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Iterator.ForEachIdx)
  - [`iter.Mapper.Map`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Mapper.Map), [`iter.Mapper.MapErr`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Mapper.MapErr)
- `-spawner` (default: true)
- `-spawnerlabel` (default: false) - Check that spawner functions are properly labeled
- `-gotask` (default: true, requires `-goroutine-deriver`)

### File Filtering

| Flag | Default | Description |
|------|---------|-------------|
| `-test` | `true` | Analyze test files (`*_test.go`) — built-in driver flag |

Generated files (containing `// Code generated ... DO NOT EDIT.`) are always excluded and cannot be opted in. This includes code below a `//line` directive in a generated file.

```bash
# Exclude test files from analysis
goroutinectx -test=false ./...
```

### `-spawnerlabel`

When enabled, checks that functions calling spawn methods with func arguments have the `//goroutinectx:spawner` directive:

```go
// Bad: calls errgroup.Group.Go() with func argument but missing directive
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

## Documentation

- [Architecture](./docs/ARCHITECTURE.md) - Technical specification and design decisions
- [Tutorial](./docs/TUTORIAL.md) - Step-by-step learning guide
- [AGENTS.md](./AGENTS.md) - Repository instructions for contributors and coding agents

## Related Tools

- [contextcheck](https://github.com/kkHAIKE/contextcheck) - Detects [`context.Background`](https://pkg.go.dev/context#Background)/[`context.TODO`](https://pkg.go.dev/context#TODO) usage and missing context parameters

`goroutinectx` is complementary to `contextcheck`:
- `contextcheck` warns about creating new contexts when one should be propagated
- `goroutinectx` warns about not using an available context in specific APIs

## AI usage

goroutinectx is developed with AI coding agents (Claude Code). The fixtures under `testdata/src/` define what it reports.

## License

MIT
