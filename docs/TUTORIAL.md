# goroutinectx Tutorial: Understanding the Goroutine Context Analyzer

This guide explains how the goroutinectx analyzer works, starting from the basics and building up to the full picture. It's designed for developers who want to understand the analyzer's behavior.

## Table of Contents

1. [What Problem Are We Solving?](#what-problem-are-we-solving)
2. [How the Analyzer Works](#how-the-analyzer-works)
3. [Understanding Context Scope](#understanding-context-scope)
4. [Higher-Order Function Support](#higher-order-function-support)
5. [The Deriver Matching System](#the-deriver-matching-system)
6. [Why Nested Closures Count](#why-nested-closures-count)
7. [Suppressing Warnings](#suppressing-warnings)

---

## What Problem Are We Solving?

When spawning goroutines, you should propagate context:

```go
// BAD - context is available but not used
func handleRequest(ctx context.Context) {
    go func() {
        doWork()  // ctx is available but not passed!
    }()
}

// GOOD - context is propagated
func handleRequest(ctx context.Context) {
    go func() {
        doWork(ctx)  // Context properly passed
    }()
}
```

The analyzer detects the "BAD" case and reports it.

### Why Is This Important?

Context carries:
- **Cancellation signals** - stopping work when requests are cancelled
- **Deadlines** - preventing goroutines from running forever
- **Request-scoped values** - trace IDs, user info, etc.

If a goroutine doesn't have context, it can:
- Continue running after the request is cancelled (resource leak)
- Miss deadlines and run forever
- Lose trace correlation in logs

---

## How the Analyzer Works

### High-Level Flow

```
┌──────────────────────────────────────────────────────────────┐
│ 1. Find functions with context.Context parameter             │
│    func handleRequest(ctx context.Context, ...) { ... }      │
│                           ↓                                  │
├──────────────────────────────────────────────────────────────┤
│ 2. For each go statement or g.Go() call in that function:    │
│    go func() { ... }                                         │
│    g.Go(func() error { ... })                                │
│                           ↓                                  │
├──────────────────────────────────────────────────────────────┤
│ 3. Check if the closure uses context                         │
│    - Direct reference: ctx, doWork(ctx)                      │
│    - Assignment: _ = ctx                                     │
│                           ↓                                  │
├──────────────────────────────────────────────────────────────┤
│ 4. Report if context is not used                             │
│    goroutine does not propagate context "ctx"                │
└──────────────────────────────────────────────────────────────┘
```

### What "Uses Context" Means

The analyzer checks if ANY context variable is referenced:

```go
func handler(ctx context.Context) {
    // GOOD: ctx is passed to a function
    go func() {
        doWork(ctx)
    }()

    // GOOD: ctx is assigned (even to _)
    go func() {
        _ = ctx
        // ... rest of code
    }()

    // GOOD: ctx is used in an expression
    go func() {
        select {
        case <-ctx.Done():
            return
        default:
        }
    }()

    // BAD: ctx is not referenced at all
    go func() {
        doOtherWork()  // No mention of ctx!
    }()
}
```

### Goroutines That Only Wait

One kind of goroutine has no use for ctx: it only waits and then tells the caller. The caller selects on `ctx.Done()` itself, so the analyzer does not report it:

```go
func handler(ctx context.Context) {
    var wg sync.WaitGroup
    done := make(chan struct{})

    // GOOD: it only waits, then closes a channel
    go func() {
        wg.Wait()
        close(done)
    }()

    select {
    case <-done:
    case <-ctx.Done():
    }
}
```

The rule has limits. These goroutines are still reported:

| Goroutine | Why it is reported |
|-----------|--------------------|
| It waits by a receive from a channel | It could select on `ctx.Done()` there. Only a wait that takes no context counts, such as `wg.Wait()` or `cmd.Wait()` |
| It sends on an unbuffered channel after the wait | The send blocks for ever when the caller has left on `ctx.Done()` |
| It sends on a buffered channel with more sends than slots | The same. An example is two goroutines that send on `make(chan error, 1)` |

> [!TIP]
> Set `-report-waiters` to report the goroutines that only wait and signal, such as the `GOOD` one above.

---

## Understanding Context Scope

### Multiple Context Parameters

Functions can have multiple context parameters. The analyzer tracks ALL of them:

```go
func handler(ctx1 context.Context, ctx2 context.Context) {
    // GOOD: uses ctx1
    go func() {
        doWork(ctx1)
    }()

    // GOOD: uses ctx2
    go func() {
        doWork(ctx2)
    }()

    // BAD: uses neither
    go func() {
        doOtherWork()
    }()
}
```

### Context Carriers

Some frameworks have their own context types (like `echo.Context`). Use `-context-carriers` to treat them as context:

```bash
goroutinectx -context-carriers=github.com/labstack/echo/v4.Context ./...
```

```go
func handler(c echo.Context) {
    // With the flag, analyzer checks c is used
    go func() {
        doWork(c)  // Good!
    }()
}
```

### Variable Shadowing

The analyzer uses type identity, not name matching:

```go
func handler(ctx context.Context) {
    // ctx is shadowed with a different type
    for _, ctx := range []string{"a", "b"} {
        go func() {
            fmt.Println(ctx) // Uses the string, NOT the context
        }() // BAD: the context ctx is not used
    }
}
```

---

## Higher-Order Function Support

### Variable Functions

The analyzer can trace function literals stored in variables:

```go
func handler(ctx context.Context) {
    fn := func() error {
        return doWork(ctx)  // Uses ctx
    }
    g.Go(fn)  // GOOD: fn is traced back to the literal
}
```

### Factory Functions

Functions that return closures are also traced:

```go
func handler(ctx context.Context) {
    g.Go(makeWorker(ctx))  // GOOD: ctx is passed to factory
}

func makeWorker(ctx context.Context) func() error {
    return func() error {
        return doWork(ctx)
    }
}
```

A factory that is not given ctx is followed too. The analyzer checks the closure it returns:

```go
func handler(ctx context.Context) {
    g.Go(getWorker())  // BAD: the closure it returns does not use ctx
}

func getWorker() func() error {
    return func() error {
        return doWork()
    }
}
```

### Limitations

A func received from a channel can't be traced. The analyzer does not report it:

```go
func handler(ctx context.Context) {
    tasks := make(chan func() error, 1)
    tasks <- func() error { return doWork() }
    g.Go(<-tasks)  // LIMITATION: not reported
}
```

---

## The Deriver Matching System

When using APM libraries, goroutines often need to call a specific function to derive a new context. The `-goroutine-deriver` flag enables this check.

### Single Deriver

```bash
goroutinectx -goroutine-deriver=github.com/my-app/apm.NewGoroutineContext ./...
```

> [!IMPORTANT]
> Name each function with the full import path of its package. A short name such as `apm.NewGoroutineContext` is rejected.

```go
func handler(ctx context.Context) {
    // BAD: uses ctx but doesn't derive
    go func() {
        doWork(ctx)
    }()

    // GOOD: derives context
    go func() {
        ctx := apm.NewGoroutineContext(ctx)
        doWork(ctx)
    }()
}
```

### OR Logic (Comma)

Multiple alternatives - at least one must be called:

```bash
-goroutine-deriver=github.com/my-app/apm.FuncA,github.com/my-app/apm.FuncB
```

```go
// GOOD: uses FuncA
go func() {
    ctx := apm.FuncA(ctx)
    doWork(ctx)
}()

// GOOD: uses FuncB
go func() {
    ctx := apm.FuncB(ctx)
    doWork(ctx)
}()
```

### AND Logic (Plus)

All specified functions must be called:

```bash
-goroutine-deriver=github.com/my-app/apm.FuncA+github.com/my-app/apm.FuncB
```

```go
// BAD: only FuncA
go func() {
    ctx := apm.FuncA(ctx)
    doWork(ctx)
}()

// GOOD: both called
go func() {
    ctx := apm.FuncA(ctx)
    ctx = apm.FuncB(ctx)
    doWork(ctx)
}()
```

### Mixed (New Relic Example)

```bash
-goroutine-deriver='github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+github.com/newrelic/go-agent/v3/newrelic.NewContext,github.com/my-app/apm.NewGoroutineContext'
```

This means: `(NewGoroutine AND NewContext) OR NewGoroutineContext`

In the golangci-lint plugin, `goroutine-deriver` is a list. Each item is one group:

```yaml
goroutine-deriver:
  - github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+github.com/newrelic/go-agent/v3/newrelic.NewContext
  - github.com/my-app/apm.NewGoroutineContext
```

---

## Why Nested Closures Count

### The Rule

A goroutine passes when its function captures the context, in its own body or in any closure inside it:

```go
func handler(ctx context.Context) {
    // GOOD: ctx is used in a deferred closure inside the goroutine
    go func() {
        defer func() {
            doWork(ctx)
        }()
    }()

    // GOOD: the outer goroutine captures ctx through the inner one
    go func() {
        go func() {
            doWork(ctx)
        }()
    }()

    // BAD: the inner goroutine does not use ctx
    go func() {
        _ = ctx
        go func() {
            doWork()
        }()
    }()
}
```

### Why This Design?

1. **How it is checked**: the check reads the closure's SSA free variables. A closure gets a free variable for each one its inner closures capture, so a context used anywhere inside it is captured by it too

2. **Zero false positives**: a goroutine that captures the context does carry it, so reporting it would be wrong

3. **Every level is still checked**: each `go` statement is checked on its own, so an inner goroutine that drops the context is reported

### Deferred Deriver Calls Are Different

With `-goroutine-deriver`, the deriver must be called when the goroutine starts. A call only in a deferred function is reported, since the goroutine has done its work by then:

```go
func handler(ctx context.Context) {
    // BAD: the deriver runs only when the goroutine ends
    go func() {
        defer func() {
            _ = apm.NewGoroutineContext(ctx)
        }()
        doWork(ctx)
    }()
}
```

---

## Suppressing Warnings

### The `//goroutinectx:ignore` Directive

For intentional fire-and-forget patterns:

```go
func handler(ctx context.Context) {
    //goroutinectx:ignore // intentionally fire-and-forget
    go func() {
        backgroundTask()  // No warning
    }()

    go func() { //goroutinectx:ignore
        anotherTask()  // Also works on same line
    }()
}
```

#### Checker-Specific Ignore

You can target specific checkers instead of suppressing all warnings:

```go
func handler(ctx context.Context) {
    //goroutinectx:ignore goroutine // only suppress goroutine checker
    go func() {
        backgroundTask()
    }()
}
```

Separate several names with commas. With `-goroutine-deriver` set, this goroutine gets a report from `goroutine` and one from `goroutinederive`. The ignore silences both:

```go
func handler(ctx context.Context) {
    //goroutinectx:ignore goroutine,goroutinederive // suppress both checkers
    go func() {
        backgroundTask()
    }()
}
```

> [!NOTE]
> Name only the checkers that report there. Without `-goroutine-deriver`, the `goroutinederive` checker does not run, so the ignore above is reported as unused for `goroutinederive`.

**Available checker names:**
- `goroutine` - `go func()` statements
- `goroutinederive` - goroutine derive function requirement
- `waitgroup` - `sync.WaitGroup.Go()` calls
- `errgroup` - `errgroup.Group.Go()` calls
- `conc` - conc library checks
- `spawner` - spawner directive checks
- `spawnerlabel` - spawner label requirement
- `gotask` - gotask library checks

#### Unused Ignore Detection

The analyzer reports unused `//goroutinectx:ignore` directives:

```go
func handler(ctx context.Context) {
    //goroutinectx:ignore  // WARNING: unused ignore directive
    go func() {
        doSomething(ctx)  // Context IS used - ignore wasn't needed
    }()

    //goroutinectx:ignore errgroup  // WARNING: unused ignore for checker "errgroup"
    go func() {
        backgroundTask()  // goroutine checker would fire, not errgroup
    }()
}
```

This helps keep your codebase clean from stale ignore comments.

### The `//goroutinectx:spawner` Directive

For wrapper functions that spawn goroutines:

```go
//goroutinectx:spawner
func runAsync(g *errgroup.Group, fn func() error) {
    g.Go(fn)
}

func handler(ctx context.Context) {
    g := new(errgroup.Group)

    // BAD: func doesn't use ctx
    runAsync(g, func() error {
        return doSomething()
    })

    // GOOD: func uses ctx
    runAsync(g, func() error {
        return doSomething(ctx)
    })
}
```

---

## Summary

1. **goroutinectx** checks that context is used in goroutines and related APIs

2. **Context scope** tracks all context parameters in the enclosing function

3. **Higher-order support** traces function literals through variables and factory calls

4. **Deriver matching** enforces APM-specific context derivation with OR/AND logic

5. **Nested closures** count - a context used in a closure inside the goroutine is captured by it too

6. **Directives** allow suppressing warnings and marking goroutine-spawning functions
