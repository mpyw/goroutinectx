# goroutinectx Tutorial: Understanding the Goroutine Context Analyzer

This guide explains how the goroutinectx analyzer works, starting from the basics and building up to the full picture. It's designed for developers who want to understand the analyzer's behavior.

## Table of Contents

1. [What Problem Are We Solving?](#what-problem-are-we-solving)
2. [How the Analyzer Works](#how-the-analyzer-works)
3. [Understanding Context Scope](#understanding-context-scope)
4. [Higher-Order Function Support](#higher-order-function-support)
5. [The Deriver Matching System](#the-deriver-matching-system)
6. [Why Nested Closures Don't Count](#why-nested-closures-dont-count)

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
│    "goroutine does not use context 'ctx'"                    │
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
    ctx := "not a context"

    go func() {
        fmt.Println(ctx)  // Uses shadowed string, NOT context
    }()  // BAD: original ctx is not used
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
    g.Go(makeWorker(ctx))  // ctx is passed to factory
}

func makeWorker(ctx context.Context) func() error {
    return func() error {
        return doWork(ctx)
    }
}
```

### Limitations

Some patterns can't be traced:

```go
func handler(ctx context.Context) {
    var fn func() error
    fn = getFromSomewhere()  // Can't trace through arbitrary calls
    g.Go(fn)  // LIMITATION: Can't verify ctx usage

    tasks := make(chan func() error)
    g.Go(<-tasks)  // LIMITATION: Can't trace through channels
}
```

---

## The Deriver Matching System

When using APM libraries, goroutines often need to call a specific function to derive a new context. The `-goroutine-deriver` flag enables this check.

### Single Deriver

```bash
goroutinectx -goroutine-deriver=github.com/my-app/apm.NewGoroutineContext ./...
```

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
-goroutine-deriver=pkg.FuncA,pkg.FuncB
```

```go
// GOOD: uses FuncA
go func() {
    ctx := pkg.FuncA(ctx)
    doWork(ctx)
}()

// GOOD: uses FuncB
go func() {
    ctx := pkg.FuncB(ctx)
    doWork(ctx)
}()
```

### AND Logic (Plus)

All specified functions must be called:

```bash
-goroutine-deriver=pkg.FuncA+pkg.FuncB
```

```go
// BAD: only FuncA
go func() {
    ctx := pkg.FuncA(ctx)
    doWork(ctx)
}()

// GOOD: both called
go func() {
    ctx := pkg.FuncA(ctx)
    ctx = pkg.FuncB(ctx)
    doWork(ctx)
}()
```

### Mixed (New Relic Example)

```bash
-goroutine-deriver='newrelic.Transaction.NewGoroutine+newrelic.NewContext,apm.NewGoroutineContext'
```

This means: `(NewGoroutine AND NewContext) OR NewGoroutineContext`

---

## Why Nested Closures Don't Count

### The Rule

Context usage in nested closures does NOT satisfy the outer goroutine's requirement:

```go
func handler(ctx context.Context) {
    // BAD: ctx is only in the nested closure
    go func() {
        go func() {
            doWork(ctx)  // ctx is here...
        }()
    }()  // ...but outer goroutine still reports error!

    // GOOD: outer explicitly acknowledges ctx
    go func() {
        _ = ctx  // "I know ctx exists and I'm propagating it"
        go func() {
            doWork(ctx)
        }()
    }()
}
```

### Why This Design?

1. **Visibility**: When reading code, `_ = ctx` makes it immediately clear the goroutine is context-aware

2. **Intentionality**: It shows the programmer consciously considered context propagation

3. **Consistency**: Every level of goroutine nesting has the same rule

4. **Refactoring Safety**: If the inner goroutine is later removed, the outer still properly propagates context

Think of `_ = ctx` as a "context checkpoint" - a declaration that "yes, context flows through here."

### Similarly for Deferred Functions

```go
func handler(ctx context.Context) {
    // BAD: ctx only in defer
    go func() {
        defer func() {
            cleanup(ctx)
        }()
        doWork()  // No ctx in main body!
    }()

    // GOOD: ctx in main body
    go func() {
        doWork(ctx)
        defer func() {
            cleanup(ctx)
        }()
    }()
}
```

---

## Suppressing Warnings

### The `//goroutinectx:ignore` Directive

For intentional fire-and-forget patterns:

```go
func handler(ctx context.Context) {
    //goroutinectx:ignore - intentionally fire-and-forget
    go func() {
        backgroundTask()  // No warning
    }()

    go func() { //goroutinectx:ignore
        anotherTask()  // Also works on same line
    }()
}
```

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

5. **Nested closures** don't count - each goroutine level must explicitly reference context

6. **Directives** allow suppressing warnings and marking goroutine-spawning functions
