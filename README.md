<div align="center">
  <h1>goroutinectx</h1>

  [![CI](https://github.com/mpyw/goroutinectx/actions/workflows/ci.yml/badge.svg)](https://github.com/mpyw/goroutinectx/actions/workflows/ci.yml)
  [![Codecov](https://codecov.io/gh/mpyw/goroutinectx/graph/badge.svg)](https://codecov.io/gh/mpyw/goroutinectx)
  [![Go Reference](https://pkg.go.dev/badge/github.com/mpyw/goroutinectx.svg)](https://pkg.go.dev/github.com/mpyw/goroutinectx)
  [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
</div>

Go linter that reports a goroutine that does not get the context of the function that starts it.

## Overview

```go
package app

import "context"

func Place(ctx context.Context, o *Order) error {
	if err := save(ctx, o); err != nil {
		return err
	}
	go func() {
		notify(context.Background(), o)
	}()
	return nil
}
```

```console
$ goroutinectx ./...
order.go:9:2: goroutine does not propagate context "ctx"
```

`Place` takes a context, but the goroutine uses a new one. The goroutine is cut off from the request.

<table>
<thead>
<tr><th>Code</th><th>What goes wrong</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
go func() {
	// ctx is cancelled, but this runs on
	sendMail(context.Background(), o)
}()
```

</td>
<td><b>The work outlives the request.</b> A cancel or a deadline of <code>ctx</code> does not stop it</td>
</tr>
<tr>
<td>

```go
go func() {
	// no trace ID, no request values
	audit(o)
}()
```

</td>
<td><b>The trace breaks.</b> The values that <code>ctx</code> carries do not reach the goroutine</td>
</tr>
<tr>
<td>

```go
go func() {
	for {
		poll()
	}
}()
```

</td>
<td><b>The goroutine can leak.</b> Nothing tells it to stop</td>
</tr>
</tbody>
</table>

The code compiles, and a test passes. goroutinectx finds it from the source.

It checks a function that takes a context. In that function, it checks each `go` statement and each callback of a library that runs it in a goroutine. The goroutine passes when it uses the context.

> [!NOTE]
> In the tables below, code without a `func` runs inside `func handler(ctx context.Context)`. The other names it uses, such as `wg` or `doSomething`, are declared outside that function.

## Install

| Method | Command | Needs |
| --- | --- | --- |
| **[mise](https://mise.jdx.dev/)** *(recommended)* | `mise use "github:mpyw/goroutinectx@0.14.1"` | Nothing. Installs the prebuilt binary |
| `go tool` | `go get -tool github.com/mpyw/goroutinectx/cmd/goroutinectx@latest` | Go 1.27+ |
| `go install` | `go install github.com/mpyw/goroutinectx/cmd/goroutinectx@latest` | Go 1.27+ |
| Release archive | See below | Nothing |

```bash
goroutinectx ./...
```

> [!IMPORTANT]
> Building goroutinectx from source requires Go 1.27 or later.

> [!CAUTION]
> To prevent supply chain attacks, pin a version tag in CI, such as `@v0.14.1`, instead of `@latest`.

<details>
<summary>Pin a version, run without installing, install from an archive, or use golangci-lint or a Go program</summary>

`mise use` works on macOS, Linux and Windows. Run it in the project root. It pins the version in the project's `mise.toml`, so every checkout and CI run the same one. Add `-g` to install it for every project on your machine instead.

```toml
[tools]
"github:mpyw/goroutinectx" = "0.14.1"
```

As a tool dependency in `go.mod`:

```bash
go get -tool github.com/mpyw/goroutinectx/cmd/goroutinectx@latest
go tool goroutinectx ./...
```

Without installing anything:

```bash
go run github.com/mpyw/goroutinectx/cmd/goroutinectx@latest ./...
```

From a release archive on [GitHub Releases](https://github.com/mpyw/goroutinectx/releases), verified against the published checksums:

```bash
VERSION=0.14.1
OS=linux    # or darwin
ARCH=amd64  # or arm64
REPO=https://github.com/mpyw/goroutinectx
BASE="${REPO}/releases/download/v${VERSION}"
curl -LO "${BASE}/goroutinectx_${VERSION}_${OS}_${ARCH}.tar.gz"
curl -LO "${BASE}/checksums.txt"
# on macOS: shasum -a 256 -c checksums.txt --ignore-missing
sha256sum --ignore-missing -c checksums.txt
tar xzf "goroutinectx_${VERSION}_${OS}_${ARCH}.tar.gz"
sudo mv goroutinectx /usr/local/bin/
```

On Windows, download `goroutinectx_${VERSION}_windows_${ARCH}.zip`. Extract `goroutinectx.exe` to a directory on your `PATH`.

Inside golangci-lint, as a [module plugin](https://golangci-lint.run/plugins/module-plugins/). goroutinectx is not bundled with golangci-lint, so build a binary that holds it. Write `.custom-gcl.yml`:

```yaml
version: v2.15.0  # the golangci-lint release to build
plugins:
  - module: github.com/mpyw/goroutinectx
    import: github.com/mpyw/goroutinectx/plugin
    version: v0.14.1
```

Turn it on in `.golangci.yml`. The keys under `settings` are the names of the [flags](#flags). A key left out keeps its default. An unknown key stops the run:

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
          # One name per item, not comma-separated.
          context-carriers:
            - github.com/labstack/echo/v4.Context
          external-spawner:
            - github.com/example/workerpool.Pool.Submit
          report-waiters: false
          spawnerlabel: true
```

Then build and run it:

```bash
golangci-lint custom  # writes ./custom-gcl
./custom-gcl run ./...
```

From a Go program of your own, run `goroutinectx.Analyzer` with [`singlechecker`](https://pkg.go.dev/golang.org/x/tools/go/analysis/singlechecker). Or run it beside other analyzers with [`multichecker`](https://pkg.go.dev/golang.org/x/tools/go/analysis/multichecker).

```go
package main

import (
	"github.com/mpyw/goroutinectx"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(goroutinectx.Analyzer)
}
```

`goroutinectx.Analyzer` takes its settings from flags. `goroutinectx.New` builds an analyzer without flags. It takes the same settings as a `goroutinectx.Config`:

```go
a, err := goroutinectx.New(goroutinectx.Config{
	ContextCarriers: []string{
		"github.com/labstack/echo/v4.Context",
	},
	Checkers: map[string]bool{"spawnerlabel": true},
})
```

</details>

## Checkers

| Checker | Default | Reports |
| --- | --- | --- |
| [`goroutine`](#goroutine) | 🟢 On | A `go` statement that does not use the context |
| [`errgroup`](#errgroup) | 🟢 On | A callback of `errgroup.Group.Go` or `TryGo` that does not use the context |
| [`waitgroup`](#waitgroup) | 🟢 On | A callback of `sync.WaitGroup.Go` that does not use the context |
| [`conc`](#conc) | 🟢 On | A callback of [conc](https://pkg.go.dev/github.com/sourcegraph/conc) that does not use the context |
| [`spawner`](#spawner) | 🟢 On | A func argument of a [spawner](#goroutinectxspawner) that does not use the context |
| [`spawnerlabel`](#spawnerlabel) | ⚪ Off | A spawner without `//goroutinectx:spawner`, or the directive where it is not needed |
| [`goroutinederive`](#goroutinederive) | With [`-goroutine-deriver`](#-goroutine-deriver) | A goroutine that does not call the deriver |
| [`gotask`](#gotask) | With [`-goroutine-deriver`](#-goroutine-deriver) | A task of [gotask](https://pkg.go.dev/github.com/siketyan/gotask/v2) that does not call the deriver |

Each checker except `goroutinederive` has a flag of its name:

```bash
# every checker except errgroup, plus spawnerlabel
goroutinectx -errgroup=false -spawnerlabel ./...
```

The name of a checker is what [`//goroutinectx:ignore`](#goroutinectxignore) takes. A report does not print it. The report reads:

| Checker | Report |
| --- | --- |
| `goroutine` | `goroutine does not propagate context "ctx"` |
| `errgroup` | `errgroup.Group.Go() closure should use context "ctx"` |
| `waitgroup` | `sync.WaitGroup.Go() closure should use context "ctx"` |
| `conc` | `pool.Pool.Go() closure should use context "ctx"` |
| `spawner` | `runAsync() func argument should use context "ctx"` |
| `spawnerlabel` | `function "runTask" should have //goroutinectx:spawner directive (calls errgroup.Group.Go with func argument)` |
| `goroutinederive` | `goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context` |
| `gotask` | `gotask.DoAllFnsSettled() 2nd argument should call goroutine deriver` |

### Files

Test files are checked too. `-test=false` leaves them out:

```bash
goroutinectx -test=false ./...
```

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
// handler_test.go
package app

import "context"

func handler(ctx context.Context) {
	go func() {
		doSomething()
	}()
}
```

</td>
<td>❌️ <b>Reported.</b> A test file is checked like any other file. With <code>-test=false</code>, it is not</td>
</tr>
<tr>
<td>

```go
// Code generated by mockgen. DO NOT EDIT.

package app

import "context"

func handler(ctx context.Context) {
	go func() {
		doSomething()
	}()
}
```

</td>
<td>✅️ <b>Passes.</b> A generated file is never checked. No flag turns it on</td>
</tr>
<tr>
<td>

```go
// Code generated by goyacc. DO NOT EDIT.

package app

import "context"

//line parser.y:10
func handler(ctx context.Context) {
	go func() {
		doSomething()
	}()
}
```

</td>
<td>✅️ <b>Passes.</b> Code below a <code>//line</code> directive is still in the generated file</td>
</tr>
</tbody>
</table>

## What counts as a context

### Functions that are checked

A function is checked when a parameter is a context. A goroutine in an unchecked function is never reported.

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
func handler(ctx context.Context) {
	go func() {
		doSomething()
	}()
}
```

</td>
<td>❌️ <b>Reported.</b> The parameter is a <code>context.Context</code></td>
</tr>
<tr>
<td>

```go
type Ctx = context.Context

func handler(ctx Ctx) {
	go func() {
		doSomething()
	}()
}
```

</td>
<td>❌️ <b>Reported.</b> An alias is the same type. So is <a href="https://pkg.go.dev/golang.org/x/net/context#Context"><code>golang.org/x/net/context.Context</code></a></td>
</tr>
<tr>
<td>

```go
type MyCtx interface {
	context.Context
}

func handler(ctx MyCtx) {
	go func() {
		doSomething()
	}()
}
```

</td>
<td>❌️ <b>Reported.</b> The interface embeds <code>context.Context</code></td>
</tr>
<tr>
<td>

```go
func handler(ctx interface {
	context.Context
	Name() string
}) {
	go func() {
		doSomething()
	}()
}
```

</td>
<td>❌️ <b>Reported.</b> An unnamed interface that embeds it counts too, beside methods of its own</td>
</tr>
<tr>
<td>

```go
func handler[C context.Context](ctx C) {
	go func() {
		doSomething()
	}()
}
```

</td>
<td>❌️ <b>Reported.</b> The constraint of the type parameter is a context. A constraint such as <code>[C MyCtx]</code> counts too</td>
</tr>
<tr>
<td>

```go
type Doner interface {
	Deadline() (time.Time, bool)
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}

func handler(ctx Doner) {
	go func() {
		doSomething()
	}()
}
```

</td>
<td>⚠️ <b>Not reported.</b> The function is not checked. The interface declares the methods itself. Only an embedded <code>context.Context</code> counts</td>
</tr>
<tr>
<td>

```go
type Request struct {
	context.Context
}

func handler(r Request) {
	go func() {
		doSomething()
	}()
}
```

</td>
<td>⚠️ <b>Not reported.</b> The function is not checked. The struct implements <code>context.Context</code>, but is not one</td>
</tr>
</tbody>
</table>

A type of a web framework can count too. See [`-context-carriers`](#-context-carriers).

### Fresh contexts

A context made with [`context.Background`](https://pkg.go.dev/context#Background) or [`context.TODO`](https://pkg.go.dev/context#TODO) does not count, wherever the goroutine gets it.

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
go func() {
	doSomething(context.TODO())
}()
```

</td>
<td>❌️ <b>Reported.</b> The goroutine uses only a fresh context</td>
</tr>
<tr>
<td>

```go
go worker(context.Background())
```

</td>
<td>❌️ <b>Reported.</b> A named function given only a fresh context is reported too</td>
</tr>
<tr>
<td>

```go
var c MyCtx = context.Background()
go func() {
	doSomething(c)
}()
```

</td>
<td>❌️ <b>Reported.</b> A fresh context held as a carrier that is a context does not count either</td>
</tr>
<tr>
<td>

```go
var d Doner = context.Background()
go func() {
	<-d.Done()
}()
```

</td>
<td>❌️ <b>Reported.</b> Nor does one held as an interface with the methods of <code>context.Context</code></td>
</tr>
<tr>
<td>

```go
go Worker{}.run(context.Background())
```

</td>
<td>❌️ <b>Reported.</b> The receiver cannot hold a context. It is an empty struct, or a struct of basic types</td>
</tr>
<tr>
<td>

```go
w := &Server{ctx: ctx}
go w.run(context.Background())
```

</td>
<td>⚠️ <b>Not reported.</b> The receiver may hold a context, in a context field or an interface field. The method may get <code>ctx</code> from there</td>
</tr>
</tbody>
</table>

### Values that hold a context

A goroutine can get the context through another value. These count:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
j := &Job{ctx: ctx}
go func() {
	doSomething(j.ctx)
}()
```

</td>
<td>✅️ <b>Passes.</b> It reads a field of a value it captures</td>
</tr>
<tr>
<td>

```go
ctxs := []context.Context{ctx}
go func() {
	doSomething(ctxs[0])
}()
```

</td>
<td>✅️ <b>Passes.</b> It reads an element of a slice. An element of a map or a channel counts too, as <code>m[k]</code>, <code>&lt;-ch</code> or a <code>range</code> gives</td>
</tr>
<tr>
<td>

```go
m := map[string]context.Context{
	"a": context.Background(),
}
go func() {
	doSomething(m["a"])
}()
```

</td>
<td>⚠️ <b>Not reported.</b> A map or a channel is not traced to what was put there. So this counts, although it holds only <code>context.Background()</code></td>
</tr>
<tr>
<td>

```go
var v any = ctx
go func() {
	doSomething(v.(context.Context))
}()
```

</td>
<td>✅️ <b>Passes.</b> A type assertion to a context gives one</td>
</tr>
<tr>
<td>

```go
r := Request{ctx}
go func() {
	doSomething(r)
}()
```

</td>
<td>✅️ <b>Passes.</b> <code>Request</code> embeds a context, so it implements <code>context.Context</code></td>
</tr>
<tr>
<td>

```go
func handler(
	ctx context.Context,
	req *http.Request,
) {
	go func() {
		doSomething(req.Context())
	}()
}
```

</td>
<td>✅️ <b>Passes.</b> A method of a captured value that returns a context counts</td>
</tr>
<tr>
<td>

```go
s := &Server{ctx: ctx}
go func() {
	s.run()
	handle(s)
}()
```

</td>
<td>✅️ <b>Passes.</b> A call given a value that holds a context counts, as the receiver or as an argument. The callee is not read, the same as for <code>go s.run()</code></td>
</tr>
</tbody>
</table>

A context field counts when your code can have put the context there:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
func handler(
	ctx context.Context,
	t *testing.T,
) {
	go func() {
		t.Log("x")
	}()
}
```

</td>
<td>❌️ <b>Reported.</b> The context field of <code>*testing.T</code> is unexported, in the standard library. So is the one of <code>*http.Request</code></td>
</tr>
<tr>
<td>

```go
l := zerolog.Ctx(ctx)
go func() {
	l.Print("x")
}()
```

</td>
<td>❌️ <b>Reported.</b> A call of another package may take a value out of <code>ctx</code>. Its result does not hold <code>ctx</code></td>
</tr>
<tr>
<td>

```go
cmd := exec.CommandContext(ctx, "sync")
go func() {
	_ = cmd.Run()
}()
```

</td>
<td>✅️ <b>Passes.</b> A call of the standard library given <code>ctx</code> made it, and its type has a context field. So do <code>http.NewRequestWithContext(ctx, ...)</code> and <code>req.WithContext(ctx)</code></td>
</tr>
<tr>
<td>

```go
cmd := exec.Command("sync")
go func() {
	_ = cmd.Run()
}()
```

</td>
<td>❌️ <b>Reported.</b> The call was not given <code>ctx</code>. A call given only <code>context.Background()</code> does not count either</td>
</tr>
<tr>
<td>

```go
id := fmt.Sprint(ctx.Value(key))
go func() {
	doSomething(id)
}()
```

</td>
<td>❌️ <b>Reported.</b> A string has no context field, even when made from <code>ctx</code></td>
</tr>
</tbody>
</table>

A field counts when it is exported, or declared in your module. When the driver does not tell the module, a field of any package outside the standard library counts.

### Interfaces

A captured value of an interface type is judged by the values it can hold:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
var r Runner = ctxRunner{ctx}
go func() {
	r.Run()
}()
```

</td>
<td>✅️ <b>Passes.</b> A value it holds holds a context. <code>handle(r)</code> counts too</td>
</tr>
<tr>
<td>

```go
var r Runner = noopRunner{}
go func() {
	r.Run()
}()
```

</td>
<td>❌️ <b>Reported.</b> No value it holds holds a context</td>
</tr>
<tr>
<td>

```go
func handler(
	ctx context.Context,
	r Runner,
) {
	go func() {
		r.Run()
	}()
}
```

</td>
<td>⚠️ <b>Not reported.</b> The value of a parameter cannot be traced. <code>Runner</code> is declared in your module, so it may hold one. A field or the result of a call is the same</td>
</tr>
<tr>
<td>

```go
func handler(
	ctx context.Context,
	conn net.Conn,
) {
	go func() {
		conn.Close()
	}()
}
```

</td>
<td>❌️ <b>Reported.</b> An interface of the standard library does not count, as <code>error</code>, <code>net.Conn</code> or <code>io.Reader</code></td>
</tr>
</tbody>
</table>

### Arguments of the goroutine

A parameter of the function the goroutine runs counts as a captured value does:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
go func(ctx context.Context) {
	doSomething(ctx)
}(ctx)
```

</td>
<td>✅️ <b>Passes.</b> It is given the context</td>
</tr>
<tr>
<td>

```go
go func(r Request) {
	doSomething(r)
}(Request{ctx})
```

</td>
<td>✅️ <b>Passes.</b> A value that implements <code>context.Context</code> counts</td>
</tr>
<tr>
<td>

```go
go func(j *Job) {
	doSomething(j.ctx)
}(&Job{ctx: ctx})
```

</td>
<td>✅️ <b>Passes.</b> A struct that holds a context your code can have put there counts</td>
</tr>
<tr>
<td>

```go
go func(r Runner) {
	r.Run()
}(ctxRunner{ctx})
```

</td>
<td>✅️ <b>Passes.</b> A parameter of another type, as an interface, a func, a slice or <code>any</code>, counts when it is given a value made from <code>ctx</code></td>
</tr>
<tr>
<td>

```go
go func(f func()) {
	f()
}(func() { <-ctx.Done() })
```

</td>
<td>✅️ <b>Passes.</b> The same, for a func</td>
</tr>
<tr>
<td>

```go
func handler(
	ctx context.Context,
	g func(),
) {
	go func(f func()) {
		f()
	}(g)
}
```

</td>
<td>⚠️ <b>Not reported.</b> The value of a parameter cannot be traced, as when the goroutine captures <code>g</code></td>
</tr>
<tr>
<td>

```go
go func(v any) {
	doSomething(v)
}(context.Background())
```

</td>
<td>❌️ <b>Reported.</b> The value is made from a fresh context</td>
</tr>
<tr>
<td>

```go
go func(rs []Record) {
	store(rs)
}(list(ctx))
```

</td>
<td>❌️ <b>Reported.</b> The records a call given <code>ctx</code> lists are data, not a holder of <code>ctx</code></td>
</tr>
<tr>
<td>

```go
go func(r Runner) {
	r.Run()
}(newRunner(ctx))
```

</td>
<td>✅️ <b>Passes.</b> The result of a call counts when it is a context, holds one, or is an interface or a func</td>
</tr>
</tbody>
</table>

### Signals of the context

A goroutine that takes a signal of the context, not the context itself, passes too. It can capture the signal, or take it as an argument. The signal is one of these:

- The channel of `ctx.Done()`
- The cancel func of a context made from `ctx`. `context.WithCancel`, `WithCancelCause`, `WithTimeout`, `WithDeadline`, `WithTimeoutCause` and `WithDeadlineCause` make one. So do `WithCancel`, `WithTimeout` and `WithDeadline` of `golang.org/x/net/context`.

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
done := ctx.Done()
go func() {
	<-done
	cleanup()
}()
```

</td>
<td>✅️ <b>Passes.</b> The goroutine ends when <code>ctx</code> is done</td>
</tr>
<tr>
<td>

```go
ctx, cancel := context.WithCancel(ctx)
defer cancel()
go func() {
	waitSignal()
	cancel()
}()
```

</td>
<td>✅️ <b>Passes.</b> The goroutine cancels a context made from <code>ctx</code></td>
</tr>
<tr>
<td>

```go
go worker(
	context.Background(),
	ctx.Done(),
)
```

</td>
<td>✅️ <b>Passes.</b> A named function given the signal passes, beside a fresh context</td>
</tr>
<tr>
<td>

```go
_, stop := context.WithCancel(
	context.Background(),
)
go func() {
	stop()
}()
```

</td>
<td>❌️ <b>Reported.</b> The context it cancels is not made from <code>ctx</code></td>
</tr>
<tr>
<td>

```go
func handler(
	ctx context.Context,
	done <-chan struct{},
) {
	go func() {
		<-done
	}()
}
```

</td>
<td>❌️ <b>Reported.</b> A channel that cannot be followed, as a parameter, does not count</td>
</tr>
</tbody>
</table>

### Func values

A goroutine that calls a func value from a variable it captures is judged as if it ran the func directly. A func that cannot be traced passes.

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
work := func() { doSomething() }
go func() {
	work()
}()
```

</td>
<td>❌️ <b>Reported.</b> <code>work</code> is followed to a closure that does not use <code>ctx</code></td>
</tr>
<tr>
<td>

```go
func handler(
	ctx context.Context,
	f func(),
) {
	go func() {
		f()
	}()
}
```

</td>
<td>⚠️ <b>Not reported.</b> A parameter cannot be traced, the same as for <code>go f()</code>. Nor can a field of a parameter</td>
</tr>
<tr>
<td>

```go
for _, f := range fs {
	go func() {
		f()
	}()
}
```

</td>
<td>⚠️ <b>Not reported.</b> A slice element read in a loop cannot be traced</td>
</tr>
<tr>
<td>

```go
func handler(
	ctx context.Context,
	jobs <-chan func(),
	handlers map[string]func(),
) {
	go func() {
		for job := range jobs {
			job()
		}
		handlers["a"]()
	}()
}
```

</td>
<td>⚠️ <b>Not reported.</b> Nor can a value of a channel or a map</td>
</tr>
<tr>
<td>

```go
run, _ := split()
go func() {
	_ = run()
}()
```

</td>
<td>⚠️ <b>Not reported.</b> Nor can a result of a call that returns several values</td>
</tr>
<tr>
<td>

```go
run := serve(ctx)
go func() {
	_ = run()
}()
```

</td>
<td>✅️ <b>Passes.</b> A factory is given <code>ctx</code>, the same as for <code>go run()</code></td>
</tr>
<tr>
<td>

```go
run := serveWith(holder{ctx})
go func() {
	_ = run()
}()
```

</td>
<td>✅️ <b>Passes.</b> So does a factory given a struct that holds <code>ctx</code> as an interface</td>
</tr>
<tr>
<td>

```go
fns := map[string]func(){
	"a": func() { doSomething(ctx) },
}
run := serveAll(fns)
go func() {
	_ = run()
}()
```

</td>
<td>✅️ <b>Passes.</b> So does a factory given a map or a channel of funcs that use <code>ctx</code></td>
</tr>
</tbody>
</table>

### Nested closures

A goroutine passes when its function uses the context, in its own body or in any closure inside it. Each `go` statement is checked on its own.

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
go func() {
	defer func() {
		doSomething(ctx)
	}()
}()
```

</td>
<td>✅️ <b>Passes.</b> The deferred closure inside the goroutine uses <code>ctx</code></td>
</tr>
<tr>
<td>

```go
go func() {
	go func() {
		doSomething(ctx)
	}()
}()
```

</td>
<td>✅️ <b>Passes.</b> Both pass. The outer goroutine uses <code>ctx</code> through the inner one</td>
</tr>
<tr>
<td>

```go
go func() {
	_ = ctx
	go func() {
		doSomething()
	}()
}()
```

</td>
<td>❌️ <b>Reported.</b> The inner goroutine does not use <code>ctx</code>. The outer one passes</td>
</tr>
</tbody>
</table>

## `goroutine`

The checker for the `go` statement. [What counts as a context](#what-counts-as-a-context) shows how it judges one.

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
go func() {
	doSomething()
}()
```

</td>
<td>❌️ <b>Reported.</b> The goroutine does not use <code>ctx</code></td>
</tr>
<tr>
<td>

```go
go func() {
	doSomething(ctx)
}()
```

</td>
<td>✅️ <b>Passes.</b> The goroutine uses <code>ctx</code></td>
</tr>
<tr>
<td>

```go
go worker(ctx)
```

</td>
<td>✅️ <b>Passes.</b> The function is given <code>ctx</code></td>
</tr>
<tr>
<td>

```go
go backgroundTask()
```

</td>
<td>⚠️ <b>Not reported.</b> A named function is judged by its arguments only. Its body is not read. It is reported only when it is given a <a href="#fresh-contexts">fresh context</a> and nothing else</td>
</tr>
</tbody>
</table>

### Goroutines that only wait and signal

A goroutine that only waits and then tells the caller is not reported. It has no use for `ctx`, because the caller selects on `ctx.Done()` itself:

```go
func run(ctx context.Context) error {
	var wg sync.WaitGroup
	// ... start workers with wg ...

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

The shape is narrow. The goroutine makes at least one of these waits:

- [`sync.WaitGroup.Wait`](https://pkg.go.dev/sync#WaitGroup.Wait)
- [`errgroup.Group.Wait`](https://pkg.go.dev/golang.org/x/sync/errgroup#Group.Wait)
- [`exec.Cmd.Wait`](https://pkg.go.dev/os/exec#Cmd.Wait), [`os.Process.Wait`](https://pkg.go.dev/os#Process.Wait)
- `Wait` of [conc](https://pkg.go.dev/github.com/sourcegraph/conc): `conc.WaitGroup`, the `pool` types and `stream.Stream`

Besides the waits, it may only do these:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
done := make(chan struct{})
go func() {
	defer close(done)
	wg.Wait()
}()
```

</td>
<td>✅️ <b>Passes.</b> It closes a channel, also in <code>defer</code></td>
</tr>
<tr>
<td>

```go
errCh := make(chan error, 1)
go func() {
	errCh <- g.Wait()
}()
```

</td>
<td>✅️ <b>Passes.</b> The send cannot block. The channel has a buffer, and this is its only send</td>
</tr>
<tr>
<td>

```go
done := make(chan struct{})
go func() {
	wg.Wait()
	select {
	case done <- struct{}{}:
	default:
	}
}()
```

</td>
<td>✅️ <b>Passes.</b> A send in a <code>select</code> with a <code>default</code> cannot block</td>
</tr>
<tr>
<td>

```go
var err error
done := make(chan struct{})
go func() {
	err = cmd.Wait()
	close(done)
}()
```

</td>
<td>✅️ <b>Passes.</b> It assigns to a variable</td>
</tr>
<tr>
<td>

```go
g.Go(func() error {
	return inner.Wait()
})
```

</td>
<td>✅️ <b>Passes.</b> It returns</td>
</tr>
<tr>
<td>

```go
failed := make(chan struct{})
go func() {
	if err := cmd.Wait(); err != nil {
		close(failed)
	}
}()
```

</td>
<td>✅️ <b>Passes.</b> It branches. A loop is not allowed</td>
</tr>
<tr>
<td>

```go
done := make(chan struct{})
go func() {
	func() {
		wg.Wait()
		close(done)
	}()
}()
```

</td>
<td>✅️ <b>Passes.</b> It calls a function literal in place. The body of the literal follows the same rules</td>
</tr>
<tr>
<td>

```go
done := make(chan struct{})
go func() {
	defer func() {
		close(done)
	}()
	wg.Wait()
}()
```

</td>
<td>✅️ <b>Passes.</b> The same for a function literal deferred in place</td>
</tr>
</tbody>
</table>

A send outside a `select` cannot block when all of these hold:

- It is the goroutine's only send outside a `select`.
- The channel is made with `make(chan T, n)` for a constant `n` of at least 1.
- There are at most `n` sends on the channel in all, none in a loop. Every goroutine and every closure counts.

A closure that sends, or starts a sender, must run once. A `go` statement, a direct or deferred call, and a `Go` of `errgroup`, `sync.WaitGroup` or conc run it once. Any other call, such as a spawner, may run it more than once.

Anything else is reported as before. These may leak when `ctx` is cancelled:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
done := make(chan struct{})
go func() {
	wg.Wait()
	log.Print("done")
	close(done)
}()
```

</td>
<td>❌️ <b>Reported.</b> A call that is not a wait, as a log line</td>
</tr>
<tr>
<td>

```go
done := make(chan struct{})
go func() {
	for range 3 {
		wg.Wait()
	}
	close(done)
}()
```

</td>
<td>❌️ <b>Reported.</b> A loop</td>
</tr>
<tr>
<td>

```go
go func() {
	_ = srv.Serve(l)
}()
```

</td>
<td>❌️ <b>Reported.</b> A server's <code>Serve</code> is not a wait</td>
</tr>
<tr>
<td>

```go
go func() {
	wg.Wait()
	<-ready
}()
```

</td>
<td>❌️ <b>Reported.</b> A receive can wait for ever when nothing is sent. The goroutine could select on <code>ctx.Done()</code> instead</td>
</tr>
<tr>
<td>

```go
done := make(chan struct{})
go func() {
	wg.Wait()
	done <- struct{}{}
}()
```

</td>
<td>❌️ <b>Reported.</b> A send on an unbuffered channel blocks for ever when the caller has returned on <code>ctx.Done()</code></td>
</tr>
<tr>
<td>

```go
func handler(
	ctx context.Context,
	done chan<- struct{},
) {
	var wg sync.WaitGroup
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()
}
```

</td>
<td>❌️ <b>Reported.</b> The same for a channel that cannot be followed, as a parameter</td>
</tr>
<tr>
<td>

```go
errCh := make(chan error, 1)
go func() {
	errCh <- g1.Wait()
}()
go func() {
	errCh <- g2.Wait()
}()
```

</td>
<td>❌️ <b>Reported.</b> Two reports. More senders than slots block the same way. The second send waits after the first receive</td>
</tr>
<tr>
<td>

```go
errCh := make(chan error, 1)
go func() {
	errCh <- g.Wait()
	errCh <- g.Wait()
}()
```

</td>
<td>❌️ <b>Reported.</b> The same for one goroutine that sends twice</td>
</tr>
<tr>
<td>

```go
errCh := make(chan error, 1)
for _, g := range groups {
	go func() {
		errCh <- g.Wait()
	}()
}
```

</td>
<td>❌️ <b>Reported.</b> The same for goroutines started in a loop</td>
</tr>
<tr>
<td>

```go
errCh := make(chan error, 1)
watch(errCh)
go func() {
	errCh <- g.Wait()
}()
```

</td>
<td>❌️ <b>Reported.</b> A channel given to a call is not followed, so its sends are reported</td>
</tr>
<tr>
<td>

```go
errCh := make(chan error, 1)
go func() {
	errCh <- g.Wait()
	func() {
		errCh <- g.Wait()
	}()
}()
```

</td>
<td>❌️ <b>Reported.</b> The sends of a literal called in place count with the goroutine's own. These are two sends on one slot</td>
</tr>
<tr>
<td>

```go
done := make(chan struct{})
go func() {
	f := func() {
		wg.Wait()
		close(done)
	}
	f()
	f()
}()
```

</td>
<td>❌️ <b>Reported.</b> A closure called twice is not called in place</td>
</tr>
<tr>
<td>

```go
done := make(chan struct{})
go func() {
	later(func() {
		wg.Wait()
		close(done)
	})
}()
```

</td>
<td>❌️ <b>Reported.</b> A closure given to a call is not called in place</td>
</tr>
<tr>
<td>

```go
done := make(chan struct{})
go func() {
	go func() {
		wg.Wait()
		close(done)
	}()
}()
```

</td>
<td>❌️ <b>Reported.</b> The outer goroutine only starts a closure, and does not wait. The inner goroutine passes</td>
</tr>
</tbody>
</table>

The rule holds for a `go` statement and for a callback of `errgroup`, `sync.WaitGroup.Go`, conc and spawners:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
done := make(chan struct{})
eg.Go(func() error {
	wg.Wait()
	close(done)
	return nil
})
```

</td>
<td>✅️ <b>Passes.</b> A callback of <code>errgroup</code> that only waits and signals</td>
</tr>
<tr>
<td>

```go
errCh := make(chan error, 1)
runAsync(func() {
	errCh <- g.Wait()
})
```

</td>
<td>❌️ <b>Reported.</b> A spawner may run its callback more than once, so a send in it may block</td>
</tr>
</tbody>
</table>

It does not change [`-goroutine-deriver`](#-goroutine-deriver). A goroutine must still call the deriver, and a callback must still call it or use `ctx`. [`-report-waiters`](#-report-waiters) reports these goroutines too.

## `errgroup`

The checker for [`errgroup.Group.Go`](https://pkg.go.dev/golang.org/x/sync/errgroup#Group.Go) and [`errgroup.Group.TryGo`](https://pkg.go.dev/golang.org/x/sync/errgroup#Group.TryGo). A callback is judged as a goroutine is.

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
g := new(errgroup.Group)
g.Go(func() error {
	return doSomething()
})
```

</td>
<td>❌️ <b>Reported.</b> The callback does not use <code>ctx</code>. <code>TryGo</code> is the same</td>
</tr>
<tr>
<td>

```go
g := new(errgroup.Group)
g.Go(func() error {
	return doSomething(ctx)
})
```

</td>
<td>✅️ <b>Passes.</b> The callback uses <code>ctx</code></td>
</tr>
</tbody>
</table>

## `waitgroup`

The checker for [`sync.WaitGroup.Go`](https://pkg.go.dev/sync#WaitGroup.Go), which Go 1.25 added.

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
var wg sync.WaitGroup
wg.Go(func() {
	doSomething()
})
```

</td>
<td>❌️ <b>Reported.</b> The callback does not use <code>ctx</code></td>
</tr>
<tr>
<td>

```go
var wg sync.WaitGroup
wg.Go(func() {
	doSomething(ctx)
})
```

</td>
<td>✅️ <b>Passes.</b> The callback uses <code>ctx</code></td>
</tr>
</tbody>
</table>

## `conc`

The checker for [conc](https://pkg.go.dev/github.com/sourcegraph/conc).

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
p := pool.New()
p.Go(func() {
	doSomething()
})
```

</td>
<td>❌️ <b>Reported.</b> The callback does not use <code>ctx</code></td>
</tr>
<tr>
<td>

```go
p := pool.New()
p.Go(func() {
	doSomething(ctx)
})
```

</td>
<td>✅️ <b>Passes.</b> The callback uses <code>ctx</code></td>
</tr>
<tr>
<td>

```go
iter.ForEach(items, func(it *Item) {
	doSomething(it)
})
```

</td>
<td>❌️ <b>Reported.</b> The <code>iter</code> functions run the callback in goroutines too</td>
</tr>
</tbody>
</table>

It checks these:

- [`conc.WaitGroup.Go`](https://pkg.go.dev/github.com/sourcegraph/conc#WaitGroup.Go)
- [`pool.Pool.Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#Pool.Go), [`pool.ResultPool[T].Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ResultPool.Go), [`pool.ContextPool.Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ContextPool.Go), [`pool.ResultContextPool[T].Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ResultContextPool.Go)
- [`pool.ErrorPool.Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ErrorPool.Go), [`pool.ResultErrorPool[T].Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ResultErrorPool.Go)
- [`stream.Stream.Go`](https://pkg.go.dev/github.com/sourcegraph/conc/stream#Stream.Go)
- [`iter.ForEach`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#ForEach), [`iter.ForEachIdx`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#ForEachIdx), [`iter.Map`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Map), [`iter.MapErr`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#MapErr)
- [`iter.Iterator.ForEach`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Iterator.ForEach), [`iter.Iterator.ForEachIdx`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Iterator.ForEachIdx)
- [`iter.Mapper.Map`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Mapper.Map), [`iter.Mapper.MapErr`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Mapper.MapErr)

## `spawner`

The checker for a function marked as a spawner. A spawner runs its func arguments in goroutines. Mark one of your own with [`//goroutinectx:spawner`](#goroutinectxspawner), and one of another module with [`-external-spawner`](#-external-spawner).

```go
//goroutinectx:spawner
func runAsync(g *errgroup.Group, fn func() error) {
	g.Go(fn)
}
```

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
g := new(errgroup.Group)
runAsync(g, func() error {
	return doSomething()
})
```

</td>
<td>❌️ <b>Reported.</b> The func argument does not use <code>ctx</code></td>
</tr>
<tr>
<td>

```go
g := new(errgroup.Group)
runAsync(g, func() error {
	return doSomething(ctx)
})
```

</td>
<td>✅️ <b>Passes.</b> The func argument uses <code>ctx</code></td>
</tr>
</tbody>
</table>

## `spawnerlabel`

Off by default. Turn it on with `-spawnerlabel`. It checks that a function has `//goroutinectx:spawner` exactly when it is a spawner.

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
func runTask(task func() error) {
	g := new(errgroup.Group)
	g.Go(task)
	_ = g.Wait()
}
```

</td>
<td>❌️ <b>Reported.</b> It gives a func argument to a spawn call, without the directive</td>
</tr>
<tr>
<td>

```go
//goroutinectx:spawner
func runTask(task func() error) {
	g := new(errgroup.Group)
	g.Go(task)
	_ = g.Wait()
}
```

</td>
<td>✅️ <b>Passes.</b> It has the directive</td>
</tr>
<tr>
<td>

```go
//goroutinectx:spawner
func simpleHelper() {
	fmt.Println("hello")
}
```

</td>
<td>❌️ <b>Reported.</b> The directive is not needed. The function spawns nothing, and has no func parameter</td>
</tr>
</tbody>
</table>

## `goroutinederive`

Runs only with [`-goroutine-deriver`](#-goroutine-deriver). A goroutine must call the deriver. The examples run with `-goroutine-deriver=github.com/my-example-app/telemetry/apm.NewGoroutineContext`:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
go func() {
	doSomething(ctx)
}()
```

</td>
<td>❌️ <b>Reported.</b> The goroutine uses <code>ctx</code>, but does not call the deriver</td>
</tr>
<tr>
<td>

```go
go func() {
	ctx := apm.NewGoroutineContext(ctx)
	doSomething(ctx)
}()
```

</td>
<td>✅️ <b>Passes.</b> The goroutine calls the deriver</td>
</tr>
<tr>
<td>

```go
go func(ctx context.Context) {
	doSomething(ctx)
}(ctx)
```

</td>
<td>✅️ <b>Passes.</b> A function that takes a context of its own is not checked for the deriver. A value that implements <code>context.Context</code> counts as one</td>
</tr>
<tr>
<td>

```go
go func(j *Job) {
	doSomething(j.ctx)
}(j)
```

</td>
<td>❌️ <b>Reported.</b> A struct that only holds a context is not one. The function must still call the deriver</td>
</tr>
<tr>
<td>

```go
g := new(errgroup.Group)
g.Go(func() error {
	return doSomething(ctx)
})
```

</td>
<td>✅️ <b>Passes.</b> A callback passes when it uses <code>ctx</code> or calls the deriver</td>
</tr>
<tr>
<td>

```go
g := new(errgroup.Group)
g.Go(func() error {
	return doSomething()
})
```

</td>
<td>❌️ <b>Reported.</b> A callback that does neither is reported once, by <code>errgroup</code></td>
</tr>
</tbody>
</table>

## `gotask`

The checker for [gotask](https://pkg.go.dev/github.com/siketyan/gotask/v2). Runs only with [`-goroutine-deriver`](#-goroutine-deriver). A task runs in a goroutine, and the call site cannot wrap its context. So the task function must call the deriver in its body. The examples run with `-goroutine-deriver=github.com/my-example-app/telemetry/apm.NewGoroutineContext`:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
_ = gotask.DoAllFnsSettled(
	ctx,
	func(ctx context.Context) error {
		return doSomething(ctx)
	},
)
```

</td>
<td>❌️ <b>Reported.</b> The task function does not call the deriver. <code>ctx</code> is not derived</td>
</tr>
<tr>
<td>

```go
_ = gotask.DoAllFnsSettled(
	ctx,
	func(ctx context.Context) error {
		ctx = apm.NewGoroutineContext(ctx)
		return doSomething(ctx)
	},
)
```

</td>
<td>✅️ <b>Passes.</b> The task function calls the deriver</td>
</tr>
<tr>
<td>

```go
tasks := []func(context.Context) error{
	func(ctx context.Context) error {
		return doSomething(ctx)
	},
}
_ = gotask.DoAllFnsSettled(ctx, tasks...)
```

</td>
<td>❌️ <b>Reported.</b> A slice expanded with <code>...</code> is judged by its elements when they can be traced</td>
</tr>
<tr>
<td>

```go
func handler(
	ctx context.Context,
	tasks []func(context.Context) error,
) {
	_ = gotask.DoAllFnsSettled(ctx, tasks...)
}
```

</td>
<td>⚠️ <b>Not reported.</b> The elements of a slice parameter cannot be traced, so it passes, as a func value that cannot be followed does</td>
</tr>
</tbody>
</table>

It checks these:

- [`gotask.DoAll`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoAll), [`gotask.DoAllFns`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoAllFns)
- [`gotask.DoAllSettled`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoAllSettled), [`gotask.DoAllFnsSettled`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoAllFnsSettled)
- [`gotask.DoRace`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoRace), [`gotask.DoRaceFns`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoRaceFns)
- [`gotask.Task.DoAsync`](https://pkg.go.dev/github.com/siketyan/gotask/v2#Task.DoAsync)
- `gotask.CancelableTask.DoAsync`, the [`Task.DoAsync`](https://pkg.go.dev/github.com/siketyan/gotask/v2#Task.DoAsync) that a [`CancelableTask`](https://pkg.go.dev/github.com/siketyan/gotask/v2#CancelableTask) embeds

For `DoAsync`, on a `Task` or a `CancelableTask`, the context argument must be derived:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
task := gotask.NewTask(
	func(ctx context.Context) error {
		return nil
	},
)
task.DoAsync(ctx, errChan)
```

</td>
<td>❌️ <b>Reported.</b> <code>ctx</code> is not derived</td>
</tr>
<tr>
<td>

```go
task := gotask.NewTask(
	func(ctx context.Context) error {
		return nil
	},
)
task.DoAsync(
	apm.NewGoroutineContext(ctx),
	errChan,
)
```

</td>
<td>✅️ <b>Passes.</b> The argument is a deriver call</td>
</tr>
<tr>
<td>

```go
task := gotask.NewTask(
	func(ctx context.Context) error {
		return nil
	},
)
dctx := apm.NewGoroutineContext(ctx)
tctx, cancel := context.WithTimeout(
	dctx,
	time.Second,
)
defer cancel()
task.DoAsync(tctx, errChan)
```

</td>
<td>✅️ <b>Passes.</b> A <code>With*</code> function of <code>context</code> or <code>golang.org/x/net/context</code> made it from a derived context</td>
</tr>
<tr>
<td>

```go
task := gotask.NewTask(
	func(ctx context.Context) error {
		ctx = apm.NewGoroutineContext(ctx)
		return doSomething(ctx)
	},
)
task.DoAsync(ctx, errChan)
```

</td>
<td>✅️ <b>Passes.</b> The task's own function calls the deriver</td>
</tr>
</tbody>
</table>

A task built by a combinator is judged by what it is given:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
bad := gotask.NewTask(
	func(ctx context.Context) gotask.Result[int] {
		return gotask.ResultOk(1)
	},
)
_ = gotask.DoAll(ctx, gotask.Parallel(bad))
```

</td>
<td>❌️ <b>Reported.</b> <a href="https://pkg.go.dev/github.com/siketyan/gotask/v2#Parallel"><code>Parallel</code></a> runs each task in a goroutine of its own. Every task must call the deriver. <a href="https://pkg.go.dev/github.com/siketyan/gotask/v2#ParallelSettled"><code>ParallelSettled</code></a> and <a href="https://pkg.go.dev/github.com/siketyan/gotask/v2#Race"><code>Race</code></a> are the same</td>
</tr>
<tr>
<td>

```go
bad := gotask.NewTask(
	func(ctx context.Context) gotask.Result[int] {
		return gotask.ResultOk(1)
	},
)
next := gotask.Then(
	bad,
	func(
		ctx context.Context,
		n int,
	) gotask.Result[int] {
		_ = apm.NewGoroutineContext(ctx)
		return gotask.ResultOk(n)
	},
)
_ = gotask.DoAll(ctx, next)
```

</td>
<td>✅️ <b>Passes.</b> <a href="https://pkg.go.dev/github.com/siketyan/gotask/v2#Then"><code>Then</code></a> runs the task and the function in one goroutine. One call of the deriver is enough, as in a <code>go</code> statement. <a href="https://pkg.go.dev/github.com/siketyan/gotask/v2#Catch"><code>Catch</code></a> is the same</td>
</tr>
<tr>
<td>

```go
func handler(
	ctx context.Context,
	tasks []gotask.Task[gotask.Result[int]],
) {
	_ = gotask.DoAll(
		ctx,
		gotask.Parallel(tasks...),
	)
}
```

</td>
<td>⚠️ <b>Not reported.</b> The elements of a slice parameter cannot be traced</td>
</tr>
</tbody>
</table>

## Directives

A directive is a line comment `//goroutinectx:name`. The name is lowercase, with no spaces. A reason goes after `//`. A reason after ` - ` is also accepted, for compatibility.

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
//goroutinectx:ignore // runs after the request
go func() {
	backgroundTask()
}()
```

</td>
<td>✅️ <b>Passes.</b> The directive silences it. The reason goes after <code>//</code></td>
</tr>
<tr>
<td>

```go
//goroutinectx:ignore - runs after the request
go func() {
	backgroundTask()
}()
```

</td>
<td>✅️ <b>Passes.</b> A reason after <code> - </code> works the same</td>
</tr>
<tr>
<td>

```go
// goroutinectx:ignore
go func() {
	backgroundTask()
}()
```

</td>
<td>❌️ <b>Reported.</b> Two reports. A comment with a space does nothing, and is reported as malformed. So are <code>/* goroutinectx:ignore */</code> and <code>//goroutinectx:Ignore</code></td>
</tr>
<tr>
<td>

```go
//goroutinectx:skip
go func() {
	backgroundTask()
}()
```

</td>
<td>❌️ <b>Reported.</b> Two reports. An unknown directive does nothing, and is reported</td>
</tr>
</tbody>
</table>

### `//goroutinectx:ignore`

It silences the reports of a line. It can be on the line above, or at the end of the line.

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
go func() { backgroundTask() }() //goroutinectx:ignore
```

</td>
<td>✅️ <b>Passes.</b> A directive after code covers its own line</td>
</tr>
<tr>
<td>

```go
_ = ctx //goroutinectx:ignore
go func() { backgroundTask() }()
```

</td>
<td>❌️ <b>Reported.</b> Two reports. A directive after code does not cover the line below. It silences nothing, so it is reported as unused</td>
</tr>
<tr>
<td>

```go
//goroutinectx:ignore
_ = gotask.DoAllFnsSettled(
	ctx,
	func(ctx context.Context) error {
		return nil
	},
	func(ctx context.Context) error {
		return nil
	},
)
```

</td>
<td>✅️ <b>Passes.</b> A directive above a call covers every argument of it. Run with <code>-goroutine-deriver</code></td>
</tr>
<tr>
<td>

```go
_ = gotask.DoAllFnsSettled(
	ctx,
	//goroutinectx:ignore
	func(ctx context.Context) error {
		return nil
	},
	func(ctx context.Context) error {
		return nil
	},
)
```

</td>
<td>❌️ <b>Reported.</b> A directive above one argument covers only that argument. The second one is reported. Run with <code>-goroutine-deriver</code></td>
</tr>
</tbody>
</table>

It takes the names of the [checkers](#checkers) to silence, separated by commas. Without a name, it silences every checker.

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
//goroutinectx:ignore goroutine // reason
go func() {
	backgroundTask()
}()
```

</td>
<td>✅️ <b>Passes.</b> It names the checker that reports</td>
</tr>
<tr>
<td>

```go
//goroutinectx:ignore errgroup // reason
go func() {
	backgroundTask()
}()
```

</td>
<td>❌️ <b>Reported.</b> Two reports. It names another checker. The goroutine is reported, and the directive is reported as unused</td>
</tr>
<tr>
<td>

```go
//goroutinectx:ignore goroutine,goroutinederive
go func() {
	backgroundTask()
}()
```

</td>
<td>✅️ <b>Passes.</b> With <code>-goroutine-deriver</code>, the goroutine gets a report from <code>goroutine</code> and one from <code>goroutinederive</code>. It silences both</td>
</tr>
<tr>
<td>

```go
//goroutinectx:ignore goroutine,goroutinederive
go func() {
	backgroundTask()
}()
```

</td>
<td>⚠️ <b>Reported.</b> Without <code>-goroutine-deriver</code>, <code>goroutinederive</code> does not run. The directive is reported as unused for it</td>
</tr>
<tr>
<td>

```go
//goroutinectx:ignore gorutine
go func() {
	backgroundTask()
}()
```

</td>
<td>❌️ <b>Reported.</b> Two reports. An unknown checker name is reported, and the directive does nothing</td>
</tr>
<tr>
<td>

```go
//goroutinectx:ignore
go func() {
	backgroundTask(ctx)
}()
```

</td>
<td>❌️ <b>Reported.</b> A directive that silences no report is reported as unused</td>
</tr>
</tbody>
</table>

> [!TIP]
> Name only the checkers that report there. A name of a checker that does not run makes the directive unused.

### `//goroutinectx:spawner`

It marks a function as a [spawner](#spawner). The [`spawner`](#spawner) checker then checks the func arguments of each call.

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
// runAsync runs fn in g.
//
//goroutinectx:spawner
func runAsync(g *errgroup.Group, fn func() error) {
	g.Go(fn)
}
```

</td>
<td>✅️ <b>Passes.</b> The directive can be anywhere in the doc comment</td>
</tr>
<tr>
<td>

```go
func runAsync(g *errgroup.Group, fn func() error) {
	//goroutinectx:spawner
	g.Go(fn)
}
```

</td>
<td>❌️ <b>Reported.</b> Anywhere else, it has no effect, and is reported</td>
</tr>
<tr>
<td>

```go
// package workers
//
//goroutinectx:spawner
func Run(fn func()) { go fn() }

// package app
func handler(ctx context.Context) {
	workers.Run(func() {
		doSomething()
	})
}
```

</td>
<td>❌️ <b>Reported.</b> An exported spawner works for calls from other packages too. goroutinectx must analyze the package that declares it. <code>go vet</code> and golangci-lint do</td>
</tr>
</tbody>
</table>

## Flags

A flag that names a function or a type takes the full import path of its package. A short name such as `apm.NewGoroutineContext` would never match, so it is rejected:

```console
$ goroutinectx -goroutine-deriver=apm.NewGoroutineContext ./...
invalid value "apm.NewGoroutineContext" for flag -goroutine-deriver: -goroutine-deriver: "apm.NewGoroutineContext": "apm" is not an import path; write the full path of the package, as github.com/org/apm.NewGoroutineContext
```

The usage follows that line, and the run stops.

A package of the standard library needs no more than its path:

```bash
goroutinectx -goroutine-deriver=context.WithoutCancel ./...
```

| Flag | Default | Effect |
| --- | --- | --- |
| [`-goroutine-deriver`](#-goroutine-deriver) | None | A goroutine must call this function |
| [`-context-carriers`](#-context-carriers) | None | These types count as a context |
| [`-external-spawner`](#-external-spawner) | None | These functions are spawners |
| [`-report-waiters`](#-report-waiters) | `false` | A goroutine that only waits and signals is reported too |
| `-goroutine`, `-errgroup`, `-waitgroup`, `-conc`, `-spawner`, `-gotask` | `true` | Turns the [checker](#checkers) on or off |
| `-spawnerlabel` | `false` | Turns the [checker](#spawnerlabel) on or off |
| `-test` | `true` | Checks test files too. A flag of the driver |

### `-goroutine-deriver`

A goroutine must call this function to derive its context. It is for an APM library such as New Relic. It turns on [`goroutinederive`](#goroutinederive) and [`gotask`](#gotask).

```bash
goroutinectx \
  -goroutine-deriver='github.com/my-example-app/telemetry/apm.NewGoroutineContext' \
  ./...
```

| Spelling | Meaning |
| --- | --- |
| `pkg/path.Func` | A function |
| `pkg/path.Type.Method` | A method |
| `A+B` | AND. The goroutine must call every function of the group |
| `A,B` | OR. The goroutine must satisfy at least one group |

With `-goroutine-deriver='github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+github.com/newrelic/go-agent/v3/newrelic.NewContext'`:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
go func() {
	txn := newrelic.FromContext(ctx).
		NewGoroutine()
	ctx := newrelic.NewContext(
		context.Background(),
		txn,
	)
	doSomething(ctx)
}()
```

</td>
<td>✅️ <b>Passes.</b> It calls both functions of the group</td>
</tr>
<tr>
<td>

```go
go func() {
	ctx := newrelic.NewContext(
		ctx,
		newrelic.FromContext(ctx),
	)
	doSomething(ctx)
}()
```

</td>
<td>❌️ <b>Reported.</b> It calls only one</td>
</tr>
</tbody>
</table>

Add a group after a comma, as `A+B,C`, and a goroutine that calls `C` alone passes too:

```bash
goroutinectx \
  -goroutine-deriver='github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+github.com/newrelic/go-agent/v3/newrelic.NewContext,github.com/my-example-app/telemetry/apm.NewGoroutineContext' \
  ./...
```

> [!TIP]
> With the [New Relic Go Agent](https://pkg.go.dev/github.com/newrelic/go-agent/v3/newrelic), a child goroutine needs [`Transaction.NewGoroutine`](https://pkg.go.dev/github.com/newrelic/go-agent/v3/newrelic#Transaction.NewGoroutine) and [`NewContext`](https://pkg.go.dev/github.com/newrelic/go-agent/v3/newrelic#NewContext), as above. A wrapper function that takes a [`context.Context`](https://pkg.go.dev/context#Context) and returns one is common:
>
> ```go
> // NewGoroutineContext derives a context
> // for goroutine instrumentation.
> func NewGoroutineContext(
> 	ctx context.Context,
> ) context.Context {
> 	txn := newrelic.FromContext(ctx)
> 	if txn == nil {
> 		return ctx
> 	}
> 	return newrelic.NewContext(ctx, txn.NewGoroutine())
> }
> ```
>
> See also: [New Relic Go Agent 完全理解・実践導入ガイド - Zenn (in Japanese)](https://zenn.dev/mpyw/articles/new-relic-go-agent-struggle)

### `-context-carriers`

These types count as a context. It is for a web framework that has a context type of its own, as [`echo.Context`](https://pkg.go.dev/github.com/labstack/echo/v4#Context) or [`cli.Context`](https://pkg.go.dev/github.com/urfave/cli/v2#Context). Separate several types with commas.

```bash
goroutinectx \
  -context-carriers='github.com/labstack/echo/v4.Context,github.com/urfave/cli/v2.Context' \
  ./...
```

With `-context-carriers='github.com/labstack/echo/v4.Context'`:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
func handler(c echo.Context) {
	go func() {
		doSomething()
	}()
}
```

</td>
<td>❌️ <b>Reported.</b> The function takes a carrier, so it is checked. Without the flag, it is not</td>
</tr>
<tr>
<td>

```go
func handler(c echo.Context) {
	go func() {
		doSomething(c)
	}()
}
```

</td>
<td>✅️ <b>Passes.</b> The goroutine uses the carrier</td>
</tr>
<tr>
<td>

```go
type AppCtx interface {
	echo.Context
}

func handler(c AppCtx) {
	go func() {
		doSomething()
	}()
}
```

</td>
<td>❌️ <b>Reported.</b> An interface that embeds a carrier counts as the carrier. So do an alias and a type parameter constrained by one, the same as for <code>context.Context</code></td>
</tr>
</tbody>
</table>

### `-external-spawner`

These functions are [spawners](#spawner). It does what [`//goroutinectx:spawner`](#goroutinectxspawner) does, for code you cannot edit. Separate several functions with commas. The spelling is `pkg/path.Func` or `pkg/path.Type.Method`.

```bash
goroutinectx \
  -external-spawner='github.com/example/workerpool.Pool.Submit,github.com/example/workerpool.Run' \
  ./...
```

With `-external-spawner='github.com/example/workerpool.Pool.Submit'`:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
p := &workerpool.Pool{}
p.Submit(func() {
	doSomething()
})
```

</td>
<td>❌️ <b>Reported.</b> The func argument does not use <code>ctx</code></td>
</tr>
<tr>
<td>

```go
p := &workerpool.Pool{}
p.Submit(func() {
	doSomething(ctx)
})
```

</td>
<td>✅️ <b>Passes.</b> The func argument uses <code>ctx</code></td>
</tr>
</tbody>
</table>

### `-report-waiters`

Also reports [goroutines that only wait and signal](#goroutines-that-only-wait-and-signal). In the golangci-lint plugin, set `report-waiters: true`.

```bash
goroutinectx -report-waiters ./...
```

With `-report-waiters`:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
done := make(chan struct{})
go func() {
	wg.Wait()
	close(done)
}()
```

</td>
<td>❌️ <b>Reported.</b> Without the flag, it passes</td>
</tr>
</tbody>
</table>

## Design principles

1. **Zero false positives.** It prefers a missed report to a false one.
2. **Type-safe analysis.** It uses [`go/types`](https://pkg.go.dev/go/types), not names.
3. **Nested functions.** It follows the context through closures.

## Documentation

- [Architecture](./docs/ARCHITECTURE.md): the technical specification and the design decisions
- [Tutorial](./docs/TUTORIAL.md): a guide, step by step
- [AGENTS.md](./AGENTS.md): instructions for contributors and coding agents

## Related tools

[contextcheck](https://github.com/kkHAIKE/contextcheck) reports [`context.Background`](https://pkg.go.dev/context#Background) or [`context.TODO`](https://pkg.go.dev/context#TODO) where a context should be passed on. It also reports a missing context parameter. The two complement each other:

| Tool | Reports |
| --- | --- |
| contextcheck | A new context made where one should be passed on |
| goroutinectx | A context that is at hand, but not used in a goroutine |

## AI usage

goroutinectx is developed with AI coding agents (Claude Code). The fixtures under `testdata/src/` define what it reports.

## License

[MIT](LICENSE)
