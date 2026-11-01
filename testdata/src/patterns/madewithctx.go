// A value a call given ctx makes, as exec.CommandContext(ctx, ...),
// captured by a goroutine instead of ctx itself (#257).

package patterns

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ===== Command made with ctx =====

// [GOOD]: Command made with ctx
//
// exec.CommandContext puts ctx in the *exec.Cmd, so running it uses ctx.
func goodCommandContextGoroutine(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "sleep", "1")
	go func() { _ = cmd.Run() }()
}

// [GOOD]: Command made with ctx
//
// exec.CommandContext puts ctx in the *exec.Cmd, so running it uses ctx.
func goodCommandContextErrgroup(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "sleep", "1")
	g := new(errgroup.Group)
	g.Go(func() error { return cmd.Run() })
	_ = g.Wait()
}

// [GOOD]: Command made with ctx
//
// exec.CommandContext puts ctx in the *exec.Cmd, so running it uses ctx.
func goodCommandContextWaitGroup(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "sleep", "1")
	var wg sync.WaitGroup
	wg.Go(func() { _ = cmd.Run() })
	wg.Wait()
}

// [GOOD]: Command made with ctx
//
// exec.CommandContext puts ctx in the *exec.Cmd, so running it uses ctx.
func goodCommandContextSpawner(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "sleep", "1")
	g := new(errgroup.Group)
	runWithGroup(g, func() error { return cmd.Run() })
	_ = g.Wait()
}

// [GOOD]: Command made with ctx, given to a go statement
//
// The go statement gives the literal a *exec.Cmd made with ctx, judged as a
// captured one.
func goodCommandContextArgGoroutine(ctx context.Context) {
	go func(cmd *exec.Cmd) { _ = cmd.Run() }(exec.CommandContext(ctx, "sleep", "1"))
}

// [BAD]: Command made with ctx, only a field read
//
// The goroutine does not run the command, so it does not use ctx.
func badCommandContextFieldGoroutine(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "sleep", "1")
	go func() { fmt.Println(cmd.Path) }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Command made without ctx
//
// exec.Command takes no context, so the goroutine does not use ctx.
func badCommandGoroutine(ctx context.Context) {
	cmd := exec.Command("sleep", "1")
	go func() { _ = cmd.Run() }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Command made without ctx
//
// exec.Command takes no context, so the goroutine does not use ctx.
func badCommandErrgroup(ctx context.Context) {
	cmd := exec.Command("sleep", "1")
	g := new(errgroup.Group)
	g.Go(func() error { return cmd.Run() }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Command made with a fresh context
//
// context.Background() carries nothing of ctx.
func badCommandFreshContextGoroutine(ctx context.Context) {
	cmd := exec.CommandContext(context.Background(), "sleep", "1")
	go func() { _ = cmd.Run() }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Command made with a fresh context, given to a go statement
//
// context.Background() carries nothing of ctx.
func badCommandFreshContextArgGoroutine(ctx context.Context) {
	go func(cmd *exec.Cmd) { _ = cmd.Run() }(exec.CommandContext(context.Background(), "sleep", "1")) // want `goroutine does not propagate context "ctx"`
}

// ===== Request made with ctx =====

// [GOOD]: Request made with ctx
//
// http.NewRequestWithContext puts ctx in the *http.Request, so sending it
// uses ctx.
func goodNewRequestWithContextGoroutine(ctx context.Context, client *http.Client) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	if err != nil {
		return
	}
	go func() {
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
}

// [GOOD]: Request given ctx by WithContext
//
// req.WithContext(ctx) gives a copy of req that holds ctx.
func goodRequestWithContextGoroutine(ctx context.Context, client *http.Client, req *http.Request) {
	req = req.WithContext(ctx)
	go func() {
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
}

// requestMaker makes a request with the context it is given.
type requestMaker interface {
	Make(ctx context.Context) *http.Request
}

// [GOOD]: Request made by an interface method given ctx
//
// The interface method is given ctx and makes an *http.Request, which has
// a context field.
func goodRequestMakerGoroutine(ctx context.Context, client *http.Client, maker requestMaker) {
	req := maker.Make(ctx)
	go func() {
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
}

// [BAD]: Request given a fresh context by WithContext
//
// context.Background() carries nothing of ctx.
func badRequestWithFreshContextGoroutine(ctx context.Context, client *http.Client, req *http.Request) {
	req2 := req.WithContext(context.Background())
	go func() { // want `goroutine does not propagate context "ctx"`
		resp, err := client.Do(req2)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
}

// ===== Data made from ctx =====

// [BAD]: String made from ctx
//
// A string cannot hold a context, so the goroutine does not use ctx.
func badStringFromContextGoroutine(ctx context.Context) {
	s := fmt.Sprint(ctx.Value("key"))
	go func() { fmt.Println(s) }() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: String made from ctx
//
// A string cannot hold a context, so the goroutine does not use ctx.
func badStringFromContextErrgroup(ctx context.Context) {
	s := fmt.Sprint(ctx.Value("key"))
	g := new(errgroup.Group)
	g.Go(func() error { fmt.Println(s); return nil }) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}
