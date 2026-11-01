# goroutinectx

[![CI](https://github.com/mpyw/goroutinectx/actions/workflows/ci.yml/badge.svg)](https://github.com/mpyw/goroutinectx/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/gh/mpyw/goroutinectx/graph/badge.svg)](https://codecov.io/gh/mpyw/goroutinectx)
[![Go Reference](https://pkg.go.dev/badge/github.com/mpyw/goroutinectx.svg)](https://pkg.go.dev/github.com/mpyw/goroutinectx)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

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

`Place` takes a context `ctx`. The goroutine does not use `ctx`. It uses a new context instead. So the goroutine is not connected to the request.

<details>
<summary>What goes wrong when a goroutine does not use the context (3 cases)</summary>

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
<td><b>The work continues after the request ends.</b> When <code>ctx</code> is cancelled or its deadline passes, the work does not stop</td>
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
<td><b>The trace breaks.</b> The goroutine does not get the values in <code>ctx</code>, such as the trace ID</td>
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

</details>

This code compiles, and its tests pass. goroutinectx finds the problem by reading the source code.

goroutinectx checks each function that takes a context. In that function, it checks these:

- Each `go` statement
- Each callback that a library runs in a goroutine

A goroutine passes when it uses the context.

> [!NOTE]
> In the examples below, code without a `func` line is the body of `func handler(ctx context.Context)`. Other names in the code, such as `wg` or `doSomething`, are declared outside that function.

## Install

| Method | Command | Needs |
| --- | --- | --- |
| **[mise](https://mise.jdx.dev/)** *(recommended)* | `mise use "github:mpyw/goroutinectx@0.15.0"` | Nothing. Installs the prebuilt binary |
| `go tool` | `go get -tool github.com/mpyw/goroutinectx/cmd/goroutinectx@latest` | Go 1.27+ |
| `go install` | `go install github.com/mpyw/goroutinectx/cmd/goroutinectx@latest` | Go 1.27+ |
| Release archive | See below | Nothing |

```bash
goroutinectx ./...
```

> [!IMPORTANT]
> Building goroutinectx from source requires Go 1.27 or later.

> [!CAUTION]
> In CI, use a fixed version tag, such as `@v0.15.0`. Do not use `@latest`. A fixed tag protects you from supply chain attacks.

<details>
<summary>More ways to install and run: mise, <code>go tool</code>, <code>go run</code>, a release archive, golangci-lint, or a Go program</summary>

`mise use` works on macOS, Linux and Windows. Run it in the root directory of the project. It writes the version to the `mise.toml` of the project. Then every checkout and every CI run use the same version. To install it for all projects on your machine, add `-g`.

```toml
[tools]
"github:mpyw/goroutinectx" = "0.15.0"
```

As a tool dependency in `go.mod`:

```bash
go get -tool github.com/mpyw/goroutinectx/cmd/goroutinectx@latest
go tool goroutinectx ./...
```

To run it without installing:

```bash
go run github.com/mpyw/goroutinectx/cmd/goroutinectx@latest ./...
```

To install from a release archive on [GitHub Releases](https://github.com/mpyw/goroutinectx/releases), download it and check it against the published checksums:

```bash
VERSION=0.15.0
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

To run it in golangci-lint, use it as a [module plugin](https://golangci-lint.run/plugins/module-plugins/). golangci-lint does not include goroutinectx. So you build a custom golangci-lint binary that includes it. Write `.custom-gcl.yml`:

```yaml
version: v2.15.0  # the golangci-lint release to build
plugins:
  - module: github.com/mpyw/goroutinectx
    import: github.com/mpyw/goroutinectx/plugin
    version: v0.15.0
```

Turn it on in `.golangci.yml`. The keys under `settings` are the names of the [flags](#flags). A key that is not set keeps its default value. An unknown key stops the run:

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
          # One group per item. A goroutine must
          # call all the functions of one group.
          # Join the functions of a group with +.
          goroutine-deriver:
            - github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+github.com/newrelic/go-agent/v3/newrelic.NewContext
            - github.com/my-example-app/telemetry/apm.NewGoroutineContext
          # One name per item, not comma-separated.
          context-carriers:
            - github.com/labstack/echo/v4.Context
          external-spawner:
            - github.com/example/workerpool.Pool.Submit
          report-waiters: false
          report-callee-without-context: false
          spawnerlabel: true
```

Then build and run it:

```bash
golangci-lint custom  # writes ./custom-gcl
./custom-gcl run ./...
```

In your own Go program, run `goroutinectx.Analyzer` with [`singlechecker`](https://pkg.go.dev/golang.org/x/tools/go/analysis/singlechecker). To run it together with other analyzers, use [`multichecker`](https://pkg.go.dev/golang.org/x/tools/go/analysis/multichecker).

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

`goroutinectx.Analyzer` reads its settings from flags. `goroutinectx.New` makes an analyzer that does not use flags. It takes the same settings in a `goroutinectx.Config`:

```go
a, err := goroutinectx.New(goroutinectx.Config{
	// One group per item, as in .golangci.yml.
	GoroutineDeriver: []string{
		"github.com/my-example-app/telemetry/apm.NewGoroutineContext",
	},
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
| [`spawnerlabel`](#spawnerlabel) | 🔴 Off | A function that runs its func parameter in a goroutine, without `//goroutinectx:spawner`. Also the directive where it is not needed |
| [`goroutinederive`](#goroutinederive) | With [`-goroutine-deriver`](#-goroutine-deriver) | A goroutine that does not call the deriver |
| [`gotask`](#gotask) | With [`-goroutine-deriver`](#-goroutine-deriver) | A task of [gotask](https://pkg.go.dev/github.com/siketyan/gotask/v2) that does not call the deriver |

Each checker except `goroutinederive` has a flag with the same name:

```bash
# every checker except errgroup, plus spawnerlabel
goroutinectx -errgroup=false -spawnerlabel ./...
```

[`//goroutinectx:ignore`](#goroutinectxignore) takes the names of checkers. A report does not show the checker name. Each checker reports this message:

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

goroutinectx checks test files too. It never checks generated files. To skip test files, use `-test=false`:

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
<td>❌️ <b>Reported.</b> A test file is checked like other files. With <code>-test=false</code>, it is not checked</td>
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
<td>✅️ <b>Passes.</b> A generated file is never checked. No flag changes this</td>
</tr>
</tbody>
</table>

<details>
<summary>All cases (3)</summary>

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
<td>❌️ <b>Reported.</b> A test file is checked like other files. With <code>-test=false</code>, it is not checked</td>
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
<td>✅️ <b>Passes.</b> A generated file is never checked. No flag changes this</td>
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
<td>✅️ <b>Passes.</b> Code after a <code>//line</code> directive is still part of the generated file</td>
</tr>
</tbody>
</table>

</details>

## `goroutine`

This checker checks each `go` statement. [What counts as a context](#what-counts-as-a-context) explains how it decides.

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
go backgroundTask()
```

</td>
<td>⚠️ <b>Not reported.</b> goroutinectx checks a named function by its arguments only. It does not read the body. It reports the call only when the arguments are a <a href="#fresh-contexts">fresh context</a> and nothing else. Use <a href="#-report-callee-without-context"><code>-report-callee-without-context</code></a> to report it</td>
</tr>
</tbody>
</table>

<details>
<summary>All cases (4)</summary>

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
<td>⚠️ <b>Not reported.</b> goroutinectx checks a named function by its arguments only. It does not read the body. It reports the call only when the arguments are a <a href="#fresh-contexts">fresh context</a> and nothing else. Use <a href="#-report-callee-without-context"><code>-report-callee-without-context</code></a> to report it</td>
</tr>
</tbody>
</table>

</details>

### Goroutines that only wait and signal

Some goroutines only wait for work to finish and then tell the caller. goroutinectx does not report them. Such a goroutine does not need `ctx`, because the caller itself selects on `ctx.Done()`:

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

This rule is strict. The goroutine must call at least one of these wait methods:

- [`sync.WaitGroup.Wait`](https://pkg.go.dev/sync#WaitGroup.Wait)
- [`errgroup.Group.Wait`](https://pkg.go.dev/golang.org/x/sync/errgroup#Group.Wait)
- [`exec.Cmd.Wait`](https://pkg.go.dev/os/exec#Cmd.Wait), [`os.Process.Wait`](https://pkg.go.dev/os#Process.Wait)
- `Wait` of [conc](https://pkg.go.dev/github.com/sourcegraph/conc): `conc.WaitGroup`, the `pool` types and `stream.Stream`

Other than the waits, it can only do these things:

- Close a channel
- Send on a channel when the send cannot block. The rules for this are below
- Assign to a variable
- Return
- Branch, but not loop
- Call a function literal in place
- Defer a function literal in place
- Call a closure in a variable once

The body of that function follows the same rules.

For example:

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
<td>✅️ <b>Passes.</b> It closes a channel. This also works in <code>defer</code></td>
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
</tbody>
</table>

<details>
<summary>All cases (11)</summary>

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
<td>✅️ <b>Passes.</b> It closes a channel. This also works in <code>defer</code></td>
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
errCh := make(chan error, 1)
if fast {
	go func() {
		errCh <- g1.Wait()
	}()
} else {
	go func() {
		errCh <- g2.Wait()
	}()
}
```

</td>
<td>✅️ <b>Passes.</b> Only one branch runs. So the two senders count as one sender for the one slot</td>
</tr>
<tr>
<td>

```go
errCh := make(chan error, 1)
send := func() {
	errCh <- cmd.Wait()
}
if fast {
	go send()
} else {
	go send()
}
```

</td>
<td>✅️ <b>Passes.</b> The same rule applies when each branch starts the same sender closure</td>
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
<td>✅️ <b>Passes.</b> The same rule applies to a function literal deferred in place</td>
</tr>
<tr>
<td>

```go
errCh := make(chan error, 1)
send := func() {
	errCh <- cmd.Wait()
}
go func() {
	send()
}()
```

</td>
<td>✅️ <b>Passes.</b> The same rule applies to a closure in a variable that the goroutine calls once</td>
</tr>
</tbody>
</table>

</details>

A send outside a `select` cannot block when all of these are true:

- It is the goroutine's only send outside a `select`.
- The channel is made with `make(chan T, n)` on every path. `n` is a constant of 1 or more. A `var ch chan T` is nil on a path that does not set it. A send on a nil channel blocks forever.
- The channel has at most `n` sends in total, and no send is in a loop. Sends in all goroutines and all closures count.

A closure that sends, or that starts a sender, must run only once. This table shows when a closure runs once:

| Closure | Runs once |
| --- | --- |
| Started by a `go` statement | Yes |
| Called directly, or deferred | Yes |
| Passed to `Go` of `errgroup`, `sync.WaitGroup` or conc | Yes |
| In a variable that another closure captures | Yes, when that closure runs once and loads the variable once. An example is `func() { go send() }()` |
| Joined with other closures by a Phi, where each loads it once | Yes. They count as one closure, because only one of them runs |
| Started, called or loaded at several places | Yes, when at most one of those places can run. An example is the two branches of an `if`. This is also true when the places are in closures called in place |
| Passed to any other call, such as a spawner | No. The call may run it more than once |

Anything else is reported as usual. Such a goroutine may leak when `ctx` is cancelled:

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
<td>❌️ <b>Reported.</b> It makes a call that is not a wait, such as a log call</td>
</tr>
<tr>
<td>

```go
go func() {
	_ = srv.Serve(l)
}()
```

</td>
<td>❌️ <b>Reported.</b> <code>Serve</code> of a server is not a wait</td>
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
<td>❌️ <b>Reported.</b> A send on an unbuffered channel blocks forever after the caller returns on <code>ctx.Done()</code></td>
</tr>
</tbody>
</table>

<details>
<summary>All cases (16)</summary>

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
<td>❌️ <b>Reported.</b> It makes a call that is not a wait, such as a log call</td>
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
<td>❌️ <b>Reported.</b> It has a loop</td>
</tr>
<tr>
<td>

```go
done := make(chan struct{})
go func() {
	if c {
		goto next
	}
wait:
	wg.Wait()
next:
	n++
	goto wait
}()
```

</td>
<td>❌️ <b>Reported.</b> It has a loop made with <code>goto</code></td>
</tr>
<tr>
<td>

```go
go func() {
	_ = srv.Serve(l)
}()
```

</td>
<td>❌️ <b>Reported.</b> <code>Serve</code> of a server is not a wait</td>
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
<td>❌️ <b>Reported.</b> A receive can wait forever when nothing is sent. The goroutine can select on <code>ctx.Done()</code> instead</td>
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
<td>❌️ <b>Reported.</b> A send on an unbuffered channel blocks forever after the caller returns on <code>ctx.Done()</code></td>
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
<td>❌️ <b>Reported.</b> The same applies to a channel that goroutinectx cannot follow, such as a parameter</td>
</tr>
<tr>
<td>

```go
var errCh chan error
if c {
	errCh = make(chan error, 1)
}
go func() {
	errCh <- g.Wait()
}()
```

</td>
<td>❌️ <b>Reported.</b> When <code>c</code> is false, <code>errCh</code> is nil. A send on a nil channel blocks forever</td>
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
<td>❌️ <b>Reported.</b> Two reports. With more senders than slots, a send blocks in the same way. The second send waits after the first receive</td>
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
<td>❌️ <b>Reported.</b> The same applies to one goroutine that sends twice</td>
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
<td>❌️ <b>Reported.</b> The same applies to goroutines started in a loop</td>
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
<td>❌️ <b>Reported.</b> goroutinectx does not follow a channel passed to a call. So its sends are reported</td>
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
<td>❌️ <b>Reported.</b> The sends in a literal called in place count together with the sends of the goroutine. So there are two sends for one slot</td>
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
<td>❌️ <b>Reported.</b> A closure called twice does not count as called in place</td>
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
<td>❌️ <b>Reported.</b> A closure passed to a call does not count as called in place</td>
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
<td>❌️ <b>Reported.</b> The outer goroutine only starts a closure. It does not wait. The inner goroutine passes</td>
</tr>
</tbody>
</table>

</details>

The rule applies to `go` statements. It also applies to callbacks of `errgroup`, `sync.WaitGroup.Go`, conc and spawners:

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
<td>✅️ <b>Passes.</b> An <code>errgroup</code> callback that only waits and signals</td>
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
<td>❌️ <b>Reported.</b> A spawner may run its callback more than once. So a send in the callback may block</td>
</tr>
</tbody>
</table>

This rule does not change [`-goroutine-deriver`](#-goroutine-deriver). A goroutine must still call the deriver. A callback must still call the deriver or use `ctx`. With [`-report-waiters`](#-report-waiters), goroutinectx reports these goroutines too.

## `errgroup`

This checker checks the callbacks of [`errgroup.Group.Go`](https://pkg.go.dev/golang.org/x/sync/errgroup#Group.Go) and [`errgroup.Group.TryGo`](https://pkg.go.dev/golang.org/x/sync/errgroup#Group.TryGo). It checks a callback in the same way as a goroutine.

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
<td>❌️ <b>Reported.</b> The callback does not use <code>ctx</code>. <code>TryGo</code> works the same way</td>
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

This checker checks the callbacks of [`sync.WaitGroup.Go`](https://pkg.go.dev/sync#WaitGroup.Go). Go 1.25 added this method.

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

This checker checks the callbacks of [conc](https://pkg.go.dev/github.com/sourcegraph/conc).

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

It checks these functions:

- [`conc.WaitGroup.Go`](https://pkg.go.dev/github.com/sourcegraph/conc#WaitGroup.Go)
- [`pool.Pool.Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#Pool.Go), [`pool.ResultPool[T].Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ResultPool.Go), [`pool.ContextPool.Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ContextPool.Go), [`pool.ResultContextPool[T].Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ResultContextPool.Go)
- [`pool.ErrorPool.Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ErrorPool.Go), [`pool.ResultErrorPool[T].Go`](https://pkg.go.dev/github.com/sourcegraph/conc/pool#ResultErrorPool.Go)
- [`stream.Stream.Go`](https://pkg.go.dev/github.com/sourcegraph/conc/stream#Stream.Go)
- [`iter.ForEach`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#ForEach), [`iter.ForEachIdx`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#ForEachIdx), [`iter.Map`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Map), [`iter.MapErr`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#MapErr)
- [`iter.Iterator.ForEach`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Iterator.ForEach), [`iter.Iterator.ForEachIdx`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Iterator.ForEachIdx)
- [`iter.Mapper.Map`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Mapper.Map), [`iter.Mapper.MapErr`](https://pkg.go.dev/github.com/sourcegraph/conc/iter#Mapper.MapErr)

## `spawner`

This checker checks calls of spawners. A spawner is a function that runs its func arguments in goroutines. Mark your own spawner with [`//goroutinectx:spawner`](#goroutinectxspawner). Mark a spawner of another module with [`-external-spawner`](#-external-spawner).

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

This checker is off by default. Turn it on with `-spawnerlabel`. It checks that every spawner has `//goroutinectx:spawner`. It also checks that no other function has it.

A function needs the directive when it runs one of its own func parameters in a goroutine. These count as running it in a goroutine:

- a `go` statement,
- a callback of `errgroup`, `sync.WaitGroup.Go` or conc,
- a func argument of a marked spawner, or of an [`-external-spawner`](#-external-spawner) function.

The value run counts when it comes from a func parameter. It can be the parameter itself, or a closure that calls or captures the parameter. A literal, a named function or a local variable of the function's own does not count.

A function with the directive is reported when it needs none. That is so when it runs no func parameter in a goroutine, and each one is only called in place or not used. A func parameter that goes where the checker cannot follow, as to a channel, keeps the directive.

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
<td>❌️ <b>Reported.</b> It passes its own func parameter to <code>g.Go</code>, but it does not have the directive</td>
</tr>
<tr>
<td>

```go
func runWork() {
	g := new(errgroup.Group)
	g.Go(func() error {
		return doWork()
	})
	_ = g.Wait()
}
```

</td>
<td>✅️ <b>Passes.</b> It passes its own literal. No func argument of a caller runs in a goroutine</td>
</tr>
</tbody>
</table>

<details>
<summary>All cases (12)</summary>

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
<td>❌️ <b>Reported.</b> It passes its own func parameter to <code>g.Go</code>, but it does not have the directive</td>
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
func runWork() {
	g := new(errgroup.Group)
	g.Go(func() error {
		return doWork()
	})
	_ = g.Wait()
}
```

</td>
<td>✅️ <b>Passes.</b> It passes its own literal. No func argument of a caller runs in a goroutine</td>
</tr>
<tr>
<td>

```go
func runAsync(cb func()) {
	go cb()
}
```

</td>
<td>❌️ <b>Reported.</b> A <code>go</code> statement runs its func parameter</td>
</tr>
<tr>
<td>

```go
func runAsync(cb func()) {
	go func() {
		cb()
	}()
}
```

</td>
<td>❌️ <b>Reported.</b> The goroutine runs a closure that calls the parameter</td>
</tr>
<tr>
<td>

```go
func runLater(cb func()) {
	spawn(func() {
		cb()
	})
}
```

</td>
<td>❌️ <b>Reported.</b> It passes a closure that calls the parameter. Capturing the parameter is enough</td>
</tr>
<tr>
<td>

```go
func withCallback(cb func()) {
	spawn(func() {})
	cb()
}
```

</td>
<td>✅️ <b>Passes.</b> It has a func parameter, but runs it in place. The spawned literal does not use it</td>
</tr>
<tr>
<td>

```go
func runWork() {
	spawn(work)
}
```

</td>
<td>✅️ <b>Passes.</b> A named function of its own is not a func argument of a caller</td>
</tr>
<tr>
<td>

```go
func runWork() {
	f := func() {}
	spawn(f)
}
```

</td>
<td>✅️ <b>Passes.</b> The variable holds its own literal</td>
</tr>
<tr>
<td>

```go
//goroutinectx:spawner
func withCallback(cb func()) {
	spawn(func() {})
	cb()
}
```

</td>
<td>❌️ <b>Reported.</b> The directive is not needed. It only calls <code>cb</code> in place</td>
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
<td>❌️ <b>Reported.</b> The directive is not needed. The function has no func parameter</td>
</tr>
<tr>
<td>

```go
//goroutinectx:spawner
func (p *Pool) Submit(f func()) {
	p.queue <- f
}
```

</td>
<td>✅️ <b>Passes.</b> The parameter escapes to a channel. The checker cannot see where it runs, so the directive stays</td>
</tr>
</tbody>
</table>

</details>

## `goroutinederive`

This checker runs only with [`-goroutine-deriver`](#-goroutine-deriver). A goroutine must call the deriver function. The examples use `-goroutine-deriver=github.com/my-example-app/telemetry/apm.NewGoroutineContext`:

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
<td>✅️ <b>Passes.</b> A function that takes its own context parameter is not checked for the deriver. A parameter type that implements <code>context.Context</code> also counts as a context</td>
</tr>
</tbody>
</table>

<details>
<summary>All cases (6)</summary>

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
<td>✅️ <b>Passes.</b> A function that takes its own context parameter is not checked for the deriver. A parameter type that implements <code>context.Context</code> also counts as a context</td>
</tr>
<tr>
<td>

```go
go func(j *Job) {
	doSomething(j.ctx)
}(j)
```

</td>
<td>❌️ <b>Reported.</b> A struct that holds a context does not count as a context. The function must still call the deriver</td>
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
<td>❌️ <b>Reported.</b> A callback that does neither is reported once, by the <code>errgroup</code> checker</td>
</tr>
</tbody>
</table>

</details>

## `gotask`

This checker checks the tasks of [gotask](https://pkg.go.dev/github.com/siketyan/gotask/v2). It runs only with [`-goroutine-deriver`](#-goroutine-deriver). A task runs in a goroutine. The caller cannot wrap the context of the task. So the task function must call the deriver in its body. The examples use `-goroutine-deriver=github.com/my-example-app/telemetry/apm.NewGoroutineContext`:

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
</tbody>
</table>

<details>
<summary>All cases (4)</summary>

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
<td>❌️ <b>Reported.</b> For a slice expanded with <code>...</code>, goroutinectx checks each element when it can trace the elements</td>
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
<td>⚠️ <b>Not reported.</b> goroutinectx cannot trace the elements of a slice parameter. So the call passes, in the same way as a func value that cannot be followed</td>
</tr>
</tbody>
</table>

</details>

It checks these functions:

- [`gotask.DoAll`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoAll), [`gotask.DoAllFns`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoAllFns)
- [`gotask.DoAllSettled`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoAllSettled), [`gotask.DoAllFnsSettled`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoAllFnsSettled)
- [`gotask.DoRace`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoRace), [`gotask.DoRaceFns`](https://pkg.go.dev/github.com/siketyan/gotask/v2#DoRaceFns)
- [`gotask.Task.DoAsync`](https://pkg.go.dev/github.com/siketyan/gotask/v2#Task.DoAsync)
- `gotask.CancelableTask.DoAsync`, the [`Task.DoAsync`](https://pkg.go.dev/github.com/siketyan/gotask/v2#Task.DoAsync) that a [`CancelableTask`](https://pkg.go.dev/github.com/siketyan/gotask/v2#CancelableTask) embeds

For `DoAsync` of a `Task` or a `CancelableTask`, the context argument must be derived:

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

<details>
<summary>All cases (5)</summary>

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
<td>✅️ <b>Passes.</b> A <code>With*</code> function of <code>context</code> or <code>golang.org/x/net/context</code> made the argument from a derived context</td>
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
<tr>
<td>

```go
func handler(
	ctx context.Context,
	task gotask.Task[error],
) {
	task.DoAsync(ctx, errChan)
}
```

</td>
<td>⚠️ <b>Not reported.</b> goroutinectx cannot trace the function of a task parameter. So it passes, in the same way as a factory that cannot be followed. A <code>*gotask.Task</code> parameter also passes</td>
</tr>
</tbody>
</table>

</details>

goroutinectx checks a task made by a combinator by the tasks and functions passed to it:

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
<td>❌️ <b>Reported.</b> <a href="https://pkg.go.dev/github.com/siketyan/gotask/v2#Parallel"><code>Parallel</code></a> runs each task in its own goroutine. Every task must call the deriver. <a href="https://pkg.go.dev/github.com/siketyan/gotask/v2#ParallelSettled"><code>ParallelSettled</code></a> and <a href="https://pkg.go.dev/github.com/siketyan/gotask/v2#Race"><code>Race</code></a> work the same way</td>
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
<td>✅️ <b>Passes.</b> <a href="https://pkg.go.dev/github.com/siketyan/gotask/v2#Then"><code>Then</code></a> runs the task and the function in one goroutine. One call of the deriver is enough, as in a <code>go</code> statement. <a href="https://pkg.go.dev/github.com/siketyan/gotask/v2#Catch"><code>Catch</code></a> works the same way</td>
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
<td>⚠️ <b>Not reported.</b> goroutinectx cannot trace the elements of a slice parameter</td>
</tr>
</tbody>
</table>

## What counts as a context

### Functions that are checked

goroutinectx checks a function when one of its parameters is a context. It never reports a goroutine in a function that it does not check.

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
<td>⚠️ <b>Not reported.</b> The function is not checked. The interface declares the methods itself. Only an embedded <code>context.Context</code> counts. Name the interface in <a href="#-context-carriers"><code>-context-carriers</code></a> to check the function</td>
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
<td>⚠️ <b>Not reported.</b> The function is not checked. The struct implements <code>context.Context</code>, but is not one. Name the struct in <a href="#-context-carriers"><code>-context-carriers</code></a> to check the function</td>
</tr>
</tbody>
</table>

<details>
<summary>All cases (7)</summary>

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
<td>❌️ <b>Reported.</b> An alias is the same type as <code>context.Context</code>. <a href="https://pkg.go.dev/golang.org/x/net/context#Context"><code>golang.org/x/net/context.Context</code></a> is also the same type</td>
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
<td>❌️ <b>Reported.</b> An unnamed interface that embeds it also counts, even with its own methods</td>
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
<td>❌️ <b>Reported.</b> The constraint of the type parameter is a context. A constraint such as <code>[C MyCtx]</code> also counts</td>
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
<td>⚠️ <b>Not reported.</b> The function is not checked. The interface declares the methods itself. Only an embedded <code>context.Context</code> counts. Name the interface in <a href="#-context-carriers"><code>-context-carriers</code></a> to check the function</td>
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
<td>⚠️ <b>Not reported.</b> The function is not checked. The struct implements <code>context.Context</code>, but is not one. Name the struct in <a href="#-context-carriers"><code>-context-carriers</code></a> to check the function</td>
</tr>
</tbody>
</table>

</details>

A context type of a web framework can also count. See [`-context-carriers`](#-context-carriers).

### Fresh contexts

A context made with [`context.Background`](https://pkg.go.dev/context#Background) or [`context.TODO`](https://pkg.go.dev/context#TODO) is a fresh context. A fresh context never counts. It does not matter how the goroutine gets it.

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
w := &Server{ctx: ctx}
go w.run(context.Background())
```

</td>
<td>⚠️ <b>Not reported.</b> The receiver can hold a context in a context field or an interface field. The method can get <code>ctx</code> from that field</td>
</tr>
</tbody>
</table>

<details>
<summary>All cases (6)</summary>

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
<td>❌️ <b>Reported.</b> A named function that gets only a fresh context is also reported</td>
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
<td>❌️ <b>Reported.</b> A fresh context in a variable of another context type, such as <code>MyCtx</code>, also does not count</td>
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
<td>❌️ <b>Reported.</b> A fresh context in an interface with the methods of <code>context.Context</code> also does not count</td>
</tr>
<tr>
<td>

```go
go Worker{}.run(context.Background())
```

</td>
<td>❌️ <b>Reported.</b> The receiver cannot hold a context. It is an empty struct or a struct of basic types. A method value, such as <code>run := Worker{}.run</code>, is checked in the same way. A method expression, such as <code>go Worker.run(Worker{}, context.Background())</code>, is also checked in the same way</td>
</tr>
<tr>
<td>

```go
w := &Server{ctx: ctx}
go w.run(context.Background())
```

</td>
<td>⚠️ <b>Not reported.</b> The receiver can hold a context in a context field or an interface field. The method can get <code>ctx</code> from that field</td>
</tr>
</tbody>
</table>

</details>

### Values that hold a context

A goroutine can get the context through another value. One example is a field of a captured value:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
ctxs := []context.Context{
	context.Background(),
}
waitAll(ctxs)
go func() {
	waitAll(ctxs)
}()
```

</td>
<td>❌️ <b>Reported.</b> The slice holds only a fresh context. Here <code>waitAll(ctxs)</code> gets only the slice. Such a call has no context to put in the slice. So it does not change the result. This is also true in a second goroutine or in another closure</td>
</tr>
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
s := &Server{ctx: ctx}
go func() {
	s.run()
	handle(s)
}()
```

</td>
<td>✅️ <b>Passes.</b> A call counts when it gets a value that holds a context. The value can be the receiver or an argument. goroutinectx does not read the called function, the same as for <code>go s.run()</code></td>
</tr>
</tbody>
</table>

<details>
<summary>All cases (9)</summary>

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
<td>✅️ <b>Passes.</b> It reads an element of a slice. An element of a map or a channel also counts. Examples are <code>m[k]</code>, <code><-ch</code> and the values of a <code>range</code>. The slice, the map or the channel also counts when it is passed to a call, such as <code>slices.Values(ctxs)</code></td>
</tr>
<tr>
<td>

```go
ctxs := []context.Context{
	context.Background(),
}
waitAll(ctxs)
go func() {
	waitAll(ctxs)
}()
```

</td>
<td>❌️ <b>Reported.</b> The slice holds only a fresh context. Here <code>waitAll(ctxs)</code> gets only the slice. Such a call has no context to put in the slice. So it does not change the result. This is also true in a second goroutine or in another closure</td>
</tr>
<tr>
<td>

```go
ctxs := []context.Context{
	context.Background(),
}
fill(ctxs, ctx)
go func() {
	waitAll(ctxs)
}()
```

</td>
<td>✅️ <b>Passes.</b> A call that also gets <code>ctx</code> may put <code>ctx</code> in the slice. A closure that captures <code>ctx</code> may also do this. A method of an interface may also do this</td>
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
<td>⚠️ <b>Not reported.</b> goroutinectx does not trace what was put in a map or a channel. So this counts, even though it holds only <code>context.Background()</code></td>
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
<td>✅️ <b>Passes.</b> A type assertion to a context type gives a context</td>
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
<td>✅️ <b>Passes.</b> A call counts when it gets a value that holds a context. The value can be the receiver or an argument. goroutinectx does not read the called function, the same as for <code>go s.run()</code></td>
</tr>
</tbody>
</table>

</details>

A context field counts when your code could have put the context in it:

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
<td>❌️ <b>Reported.</b> <code>*testing.T</code> is in the standard library, and its context field is unexported. The same is true for <code>*http.Request</code></td>
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
<td>✅️ <b>Passes.</b> A standard library function made it from <code>ctx</code>, and its type has a context field. <code>http.NewRequestWithContext(ctx, ...)</code> and <code>req.WithContext(ctx)</code> also pass</td>
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
<td>❌️ <b>Reported.</b> A function of another package may take a value out of <code>ctx</code>. Its result does not hold <code>ctx</code></td>
</tr>
</tbody>
</table>

<details>
<summary>All cases (5)</summary>

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
<td>❌️ <b>Reported.</b> <code>*testing.T</code> is in the standard library, and its context field is unexported. The same is true for <code>*http.Request</code></td>
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
<td>❌️ <b>Reported.</b> A function of another package may take a value out of <code>ctx</code>. Its result does not hold <code>ctx</code></td>
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
<td>✅️ <b>Passes.</b> A standard library function made it from <code>ctx</code>, and its type has a context field. <code>http.NewRequestWithContext(ctx, ...)</code> and <code>req.WithContext(ctx)</code> also pass</td>
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
<td>❌️ <b>Reported.</b> The call did not get <code>ctx</code>. A call that gets only <code>context.Background()</code> also does not count</td>
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

</details>

A field counts when it is exported or declared in your module. Some drivers do not tell goroutinectx the module. Then a field of any package outside the standard library counts.

### Interfaces

For a captured value of an interface type, goroutinectx checks the values that it can hold:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
var r Runner = noopRunner{}
go func() {
	r.Run()
}()
```

</td>
<td>❌️ <b>Reported.</b> No value in the interface holds a context</td>
</tr>
<tr>
<td>

```go
var r Runner = ctxRunner{ctx}
go func() {
	r.Run()
}()
```

</td>
<td>✅️ <b>Passes.</b> A value in the interface holds a context. <code>handle(r)</code> also counts</td>
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
<td>⚠️ <b>Not reported.</b> goroutinectx cannot trace the value of a parameter. <code>Runner</code> is declared in your module, so it may hold a context. A field or a call result works the same way</td>
</tr>
</tbody>
</table>

<details>
<summary>All cases (4)</summary>

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
<td>✅️ <b>Passes.</b> A value in the interface holds a context. <code>handle(r)</code> also counts</td>
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
<td>❌️ <b>Reported.</b> No value in the interface holds a context</td>
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
<td>⚠️ <b>Not reported.</b> goroutinectx cannot trace the value of a parameter. <code>Runner</code> is declared in your module, so it may hold a context. A field or a call result works the same way</td>
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
<td>❌️ <b>Reported.</b> An interface of the standard library does not count. Examples are <code>error</code>, <code>net.Conn</code> and <code>io.Reader</code></td>
</tr>
</tbody>
</table>

</details>

### Arguments of the goroutine

The goroutine function can take the context as a parameter. Such a parameter counts in the same way as a captured value:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
j := Job{ctx: context.Background()}
go func(j *Job) {
	doSomething(j.ctx)
}(&j)
```

</td>
<td>❌️ <b>Reported.</b> A struct does not count when it holds only fresh contexts. The same is true for a pointer to it or a slice of it. goroutinectx does not follow some writes to a struct. An example is a method that gets the address of the struct and a context. Such a struct counts. A method that gets no context, such as <code>j.reset()</code>, does not change this</td>
</tr>
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
<td>⚠️ <b>Not reported.</b> goroutinectx cannot trace the value of a parameter. It is the same as when the goroutine captures <code>g</code>. A closure inside the goroutine that runs <code>f</code> works the same way. Mark <code>handler</code> with <a href="#goroutinectxspawner"><code>//goroutinectx:spawner</code></a> to check the func argument of each call</td>
</tr>
<tr>
<td>

```go
go func(rs []Record) {
	store(rs)
}(list(ctx))
```

</td>
<td>❌️ <b>Reported.</b> <code>list(ctx)</code> gets <code>ctx</code>, but it returns records. The records are data. They do not hold <code>ctx</code></td>
</tr>
</tbody>
</table>

<details>
<summary>All cases (12)</summary>

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
<td>✅️ <b>Passes.</b> A struct counts when it holds a context that your code could have put there</td>
</tr>
<tr>
<td>

```go
j := Job{ctx: context.Background()}
go func(j *Job) {
	doSomething(j.ctx)
}(&j)
```

</td>
<td>❌️ <b>Reported.</b> A struct does not count when it holds only fresh contexts. The same is true for a pointer to it or a slice of it. goroutinectx does not follow some writes to a struct. An example is a method that gets the address of the struct and a context. Such a struct counts. A method that gets no context, such as <code>j.reset()</code>, does not change this</td>
</tr>
<tr>
<td>

```go
go func(r Runner) {
	r.Run()
}(ctxRunner{ctx})
```

</td>
<td>✅️ <b>Passes.</b> A parameter of another type counts when it gets a value made from <code>ctx</code>. Examples are an interface, a func, a slice and <code>any</code></td>
</tr>
<tr>
<td>

```go
go func(f func()) {
	f()
}(func() { <-ctx.Done() })
```

</td>
<td>✅️ <b>Passes.</b> The same is true for a func</td>
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
<td>⚠️ <b>Not reported.</b> goroutinectx cannot trace the value of a parameter. It is the same as when the goroutine captures <code>g</code>. A closure inside the goroutine that runs <code>f</code> works the same way. Mark <code>handler</code> with <a href="#goroutinectxspawner"><code>//goroutinectx:spawner</code></a> to check the func argument of each call</td>
</tr>
<tr>
<td>

```go
func handler(
	ctx context.Context,
	jobs []func(),
) {
	go func(fs []func()) {
		fs[0]()
	}(jobs)
}
```

</td>
<td>⚠️ <b>Not reported.</b> The same is true for a slice, a map or a channel of funcs</td>
</tr>
<tr>
<td>

```go
func handler(
	ctx context.Context,
	ctxs []context.Context,
) {
	go func(cs []context.Context) {
		<-cs[0].Done()
	}(ctxs)
}
```

</td>
<td>⚠️ <b>Not reported.</b> The same is true for a slice, an array, a map, a channel or a pointer of contexts. It is also true for these of structs with a context field. It is the same as when the goroutine captures <code>ctxs</code>. A literal that holds only fresh contexts is reported. A local variable that holds only fresh contexts is also reported</td>
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
<td>❌️ <b>Reported.</b> <code>list(ctx)</code> gets <code>ctx</code>, but it returns records. The records are data. They do not hold <code>ctx</code></td>
</tr>
<tr>
<td>

```go
go func(r Runner) {
	r.Run()
}(newRunner(ctx))
```

</td>
<td>✅️ <b>Passes.</b> A call result counts when it is a context, holds a context, or is an interface or a func</td>
</tr>
</tbody>
</table>

</details>

### Signals of the context

A goroutine also passes when it uses a signal of the context instead of the context. It can capture the signal or take it as an argument. These are signals:

- The channel of `ctx.Done()`
- The cancel func of a context made from `ctx`. These functions make one: `context.WithCancel`, `WithCancelCause`, `WithTimeout`, `WithDeadline`, `WithTimeoutCause` and `WithDeadlineCause`. `WithCancel`, `WithTimeout` and `WithDeadline` of `golang.org/x/net/context` also make one.

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
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
<td>❌️ <b>Reported.</b> A channel that goroutinectx cannot follow does not count. An example is a parameter</td>
</tr>
</tbody>
</table>

<details>
<summary>All cases (5)</summary>

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
<td>✅️ <b>Passes.</b> A named function passes when it gets the signal, even with a fresh context</td>
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
<td>❌️ <b>Reported.</b> A channel that goroutinectx cannot follow does not count. An example is a parameter</td>
</tr>
</tbody>
</table>

</details>

### Func values

A goroutine can call a func value from a captured variable. goroutinectx checks it as if the goroutine ran the func directly. A func that goroutinectx cannot trace passes.

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
<td>❌️ <b>Reported.</b> goroutinectx follows <code>work</code> to a closure. The closure does not use <code>ctx</code></td>
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
<td>✅️ <b>Passes.</b> The factory gets <code>ctx</code>, the same as for <code>go run()</code></td>
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
<td>⚠️ <b>Not reported.</b> goroutinectx cannot trace a parameter, the same as for <code>go f()</code>. It also cannot trace a field of a parameter. Mark <code>handler</code> with <a href="#goroutinectxspawner"><code>//goroutinectx:spawner</code></a> to check the func argument of each call</td>
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
<td>⚠️ <b>Not reported.</b> goroutinectx cannot trace a slice element read in a loop</td>
</tr>
</tbody>
</table>

<details>
<summary>All cases (10)</summary>

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
<td>❌️ <b>Reported.</b> goroutinectx follows <code>work</code> to a closure. The closure does not use <code>ctx</code></td>
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
<td>⚠️ <b>Not reported.</b> goroutinectx cannot trace a parameter, the same as for <code>go f()</code>. It also cannot trace a field of a parameter. Mark <code>handler</code> with <a href="#goroutinectxspawner"><code>//goroutinectx:spawner</code></a> to check the func argument of each call</td>
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
<td>⚠️ <b>Not reported.</b> goroutinectx cannot trace a slice element read in a loop</td>
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
<td>⚠️ <b>Not reported.</b> It also cannot trace a value from a channel or a map</td>
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
<td>⚠️ <b>Not reported.</b> It also cannot trace a result of a call that returns several values</td>
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
<td>✅️ <b>Passes.</b> The factory gets <code>ctx</code>, the same as for <code>go run()</code></td>
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
<td>✅️ <b>Passes.</b> A factory also passes when it gets a struct that holds <code>ctx</code> as an interface</td>
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
<td>✅️ <b>Passes.</b> A factory also passes when it gets a map or a channel of funcs that use <code>ctx</code></td>
</tr>
<tr>
<td>

```go
run := worker
go func() {
	run(context.Background())
}()
```

</td>
<td>❌️ <b>Reported.</b> A named function that gets only a fresh context does not get <code>ctx</code>. This is the same as for <code>go worker(context.Background())</code>. A method value is also reported when its receiver cannot hold a context. An example is <code>run := Worker{}.run</code></td>
</tr>
<tr>
<td>

```go
func handler(
	ctx context.Context,
	jobs chan func(),
) {
	go runFrom(jobs)()
}
```

</td>
<td>⚠️ <b>Not reported.</b> goroutinectx cannot trace what is sent on a channel parameter. It is the same as when the goroutine receives from <code>jobs</code> itself. A map parameter works the same way</td>
</tr>
</tbody>
</table>

</details>

### Nested closures

A goroutine passes when its function uses the context. The use can be in its own body or in any closure inside it. Each `go` statement is checked separately.

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

## Directives

A directive is a line comment `//goroutinectx:name`. The name is lowercase and has no spaces. Write a reason after `//`. A reason after ` - ` is also accepted for compatibility.

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
// goroutinectx:ignore
go func() {
	backgroundTask()
}()
```

</td>
<td>❌️ <b>Reported.</b> Two reports. A comment with a space has no effect, and it is reported as malformed. <code>/* goroutinectx:ignore */</code> and <code>//goroutinectx:Ignore</code> are also reported as malformed</td>
</tr>
<tr>
<td>

```go
//goroutinectx:ignore // runs after the request
go func() {
	backgroundTask()
}()
```

</td>
<td>✅️ <b>Passes.</b> The directive silences the report. The reason comes after <code>//</code></td>
</tr>
</tbody>
</table>

<details>
<summary>All cases (4)</summary>

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
<td>✅️ <b>Passes.</b> The directive silences the report. The reason comes after <code>//</code></td>
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
<td>❌️ <b>Reported.</b> Two reports. A comment with a space has no effect, and it is reported as malformed. <code>/* goroutinectx:ignore */</code> and <code>//goroutinectx:Ignore</code> are also reported as malformed</td>
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
<td>❌️ <b>Reported.</b> Two reports. An unknown directive has no effect, and it is reported</td>
</tr>
</tbody>
</table>

</details>

### `//goroutinectx:ignore`

It silences the reports on one line. Put it on the line above or at the end of the line.

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
_ = ctx //goroutinectx:ignore
go func() { backgroundTask() }()
```

</td>
<td>❌️ <b>Reported.</b> Two reports. A directive after code does not cover the line below. It silences nothing. So it is reported as unused</td>
</tr>
<tr>
<td>

```go
go func() { backgroundTask() }() //goroutinectx:ignore
```

</td>
<td>✅️ <b>Passes.</b> A directive after code covers its own line</td>
</tr>
</tbody>
</table>

<details>
<summary>All cases (4)</summary>

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
<td>❌️ <b>Reported.</b> Two reports. A directive after code does not cover the line below. It silences nothing. So it is reported as unused</td>
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
<td>✅️ <b>Passes.</b> A directive above a call covers all of its arguments. This example runs with <code>-goroutine-deriver</code></td>
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
<td>❌️ <b>Reported.</b> A directive above one argument covers only that argument. The second one is reported. This example runs with <code>-goroutine-deriver</code></td>
</tr>
</tbody>
</table>

</details>

It takes the names of the [checkers](#checkers) to silence. Separate the names with commas. Without a name, it silences all checkers.

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
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
//goroutinectx:ignore goroutine,goroutinederive
go func() {
	backgroundTask()
}()
```

</td>
<td>⚠️ <b>Reported.</b> Without <code>-goroutine-deriver</code>, <code>goroutinederive</code> does not run. The directive is reported as unused for that checker</td>
</tr>
</tbody>
</table>

<details>
<summary>All cases (6)</summary>

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
<td>✅️ <b>Passes.</b> With <code>-goroutine-deriver</code>, the goroutine gets two reports. One is from <code>goroutine</code>, and one is from <code>goroutinederive</code>. The directive silences both</td>
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
<td>⚠️ <b>Reported.</b> Without <code>-goroutine-deriver</code>, <code>goroutinederive</code> does not run. The directive is reported as unused for that checker</td>
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
<td>❌️ <b>Reported.</b> Two reports. An unknown checker name is reported, and the directive has no effect</td>
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

</details>

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
<td>❌️ <b>Reported.</b> In any other place, it has no effect, and it is reported</td>
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
<td>❌️ <b>Reported.</b> An exported spawner also works for calls from other packages. For this, goroutinectx must analyze the package that declares the spawner. <code>go vet</code> and golangci-lint do this</td>
</tr>
</tbody>
</table>

## Flags

A flag that names a function or a type needs the full import path of the package. A short name such as `apm.NewGoroutineContext` never matches. So goroutinectx rejects it:

```console
$ goroutinectx -goroutine-deriver=apm.NewGoroutineContext ./...
invalid value "apm.NewGoroutineContext" for flag -goroutine-deriver: -goroutine-deriver: "apm.NewGoroutineContext": "apm" is not an import path; write the full path of the package, as github.com/org/apm.NewGoroutineContext
```

The usage text follows that line, and the run stops.

For a standard library package, the path is enough:

```bash
goroutinectx -goroutine-deriver=context.WithoutCancel ./...
```

| Flag | Default | Effect |
| --- | --- | --- |
| [`-goroutine-deriver`](#-goroutine-deriver) | None | A goroutine must call this function |
| [`-context-carriers`](#-context-carriers) | None | These types count as a context |
| [`-external-spawner`](#-external-spawner) | None | These functions are spawners |
| [`-report-waiters`](#-report-waiters) | `false` | A goroutine that only waits and signals is reported too |
| [`-report-callee-without-context`](#-report-callee-without-context) | `false` | A `go` statement that calls a named function given no context is reported too |
| `-goroutine`, `-errgroup`, `-waitgroup`, `-conc`, `-spawner`, `-gotask` | `true` | Turns the [checker](#checkers) on or off |
| `-spawnerlabel` | `false` | Turns the [checker](#spawnerlabel) on or off |
| `-test` | `true` | Checks test files too. This flag belongs to the driver |

### `-goroutine-deriver`

A goroutine must call this function to derive its context. Use it with an APM library such as New Relic. It turns on [`goroutinederive`](#goroutinederive) and [`gotask`](#gotask).

```bash
goroutinectx \
  -goroutine-deriver='github.com/my-example-app/telemetry/apm.NewGoroutineContext' \
  ./...
```

With this flag, a goroutine that uses `ctx` must also call the deriver:

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
</tbody>
</table>

The setting can name several functions. In `.golangci.yml`, it is a list with one group per item. On the command line, commas join the groups:

| Meaning | Command line | `.golangci.yml` |
| --- | --- | --- |
| A function | `pkg/path.Func` | `- pkg/path.Func` |
| A method | `pkg/path.Type.Method` | `- pkg/path.Type.Method` |
| AND. The goroutine must call every function of the group | `A+B` | `- A+B` |
| OR. The goroutine must satisfy at least one group | `A,B` | `- A`<br>`- B` |

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

Add another group after a comma, as in `A+B,C`. Then a goroutine that calls only `C` also passes:

```bash
goroutinectx \
  -goroutine-deriver='github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+github.com/newrelic/go-agent/v3/newrelic.NewContext,github.com/my-example-app/telemetry/apm.NewGoroutineContext' \
  ./...
```

In `.golangci.yml`, the same two groups are two items:

```yaml
goroutine-deriver:
  - github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine+github.com/newrelic/go-agent/v3/newrelic.NewContext
  - github.com/my-example-app/telemetry/apm.NewGoroutineContext
```

> [!TIP]
> With the [New Relic Go Agent](https://pkg.go.dev/github.com/newrelic/go-agent/v3/newrelic), a child goroutine needs [`Transaction.NewGoroutine`](https://pkg.go.dev/github.com/newrelic/go-agent/v3/newrelic#Transaction.NewGoroutine) and [`NewContext`](https://pkg.go.dev/github.com/newrelic/go-agent/v3/newrelic#NewContext), as shown above. A common way is a wrapper function. It takes a [`context.Context`](https://pkg.go.dev/context#Context) and returns a new one:
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

These types count as a context. Use it with a web framework that has its own context type, such as [`echo.Context`](https://pkg.go.dev/github.com/labstack/echo/v4#Context) or [`cli.Context`](https://pkg.go.dev/github.com/urfave/cli/v2#Context). Separate several types with commas.

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
<td>❌️ <b>Reported.</b> The function takes a carrier, so it is checked. Without the flag, it is not checked</td>
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
<td>❌️ <b>Reported.</b> An interface that embeds a carrier counts as the carrier. An alias of a carrier also counts. A type parameter constrained by a carrier also counts. This is the same as for <code>context.Context</code></td>
</tr>
</tbody>
</table>

### `-external-spawner`

These functions are [spawners](#spawner). The flag works like [`//goroutinectx:spawner`](#goroutinectxspawner), for code that you cannot edit. Separate several functions with commas. Write each one as `pkg/path.Func` or `pkg/path.Type.Method`.

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

goroutinectx also reports [goroutines that only wait and signal](#goroutines-that-only-wait-and-signal). In the golangci-lint plugin, set `report-waiters: true`.

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

### `-report-callee-without-context`

goroutinectx also reports a `go` statement that calls a named function or a method and gives it no context. Only the [`goroutine`](#goroutine) checker reports it. In the golangci-lint plugin, set `report-callee-without-context: true`.

```bash
goroutinectx -report-callee-without-context ./...
```

With `-report-callee-without-context`:

<table>
<thead>
<tr><th>Code</th><th>Verdict</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
go backgroundTask()
```

</td>
<td>❌️ <b>Reported.</b> Without the flag, it passes</td>
</tr>
<tr>
<td>

```go
// s holds no context.
go s.loop()
```

</td>
<td>❌️ <b>Reported.</b> The receiver gives no context either</td>
</tr>
<tr>
<td>

```go
go worker(ctx.Done())
```

</td>
<td>✅️ <b>Passes.</b> The argument is a <a href="#signals-of-the-context">signal of the context</a></td>
</tr>
<tr>
<td>

```go
// s has a ctx field.
go s.run()
```

</td>
<td>✅️ <b>Passes.</b> The receiver <a href="#values-that-hold-a-context">holds a context</a></td>
</tr>
<tr>
<td>

```go
// func worker(ctx context.Context)
go worker(context.WithoutCancel(ctx))
```

</td>
<td>✅️ <b>Passes.</b> The function takes a context. The flag leaves it to the usual rules for <a href="#fresh-contexts">fresh contexts</a></td>
</tr>
<tr>
<td>

```go
// func worker(ctx context.Context)
go worker(context.Background())
```

</td>
<td>❌️ <b>Reported.</b> The usual rule for fresh contexts reports it, with or without the flag</td>
</tr>
<tr>
<td>

```go
go backgroundTask() //goroutinectx:ignore goroutine
```

</td>
<td>✅️ <b>Passes.</b> The ignore silences the report. Without the flag, the ignore is reported as unused</td>
</tr>
<tr>
<td>

```go
// fn is a func parameter.
// r is an interface.
go fn()
go r.Run()
```

</td>
<td>⚠️ <b>Not reported.</b> The flag does not touch a func value, a func parameter or an interface method. A func literal keeps the usual rules</td>
</tr>
</tbody>
</table>

The flag is off by default. A method that runs an object's own loop, such as `go s.loop()`, is usually intended.

## Design principles

1. **Zero false positives.** A missed report is better than a false report.
2. **Type-safe analysis.** It uses [`go/types`](https://pkg.go.dev/go/types), not names.
3. **Nested functions.** It follows the context through closures.

## Documentation

- [Architecture](./docs/ARCHITECTURE.md): the technical specification and the design decisions
- [Tutorial](./docs/TUTORIAL.md): a step-by-step guide
- [AGENTS.md](./AGENTS.md): instructions for contributors and coding agents

## Related tools

[contextcheck](https://github.com/kkHAIKE/contextcheck) reports [`context.Background`](https://pkg.go.dev/context#Background) or [`context.TODO`](https://pkg.go.dev/context#TODO) where a context should be passed on. It also reports a missing context parameter. The two tools work well together:

| Tool | Reports |
| --- | --- |
| contextcheck | A new context made where one should be passed on |
| goroutinectx | A context that is at hand, but not used in a goroutine |

## AI usage

goroutinectx is developed with AI coding agents (Claude Code). The fixtures under `testdata/src/` define what it reports.

## License

[MIT](LICENSE)
