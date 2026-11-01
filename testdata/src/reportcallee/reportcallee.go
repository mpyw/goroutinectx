// Package reportcallee holds go statements that call a named function or
// method, run with -report-callee-without-context and with
// -context-carriers=github.com/labstack/echo/v4.Context. Without the flag,
// none of them is reported.
package reportcallee

import (
	"context"
	"net"
	"os/exec"
	"sync"

	"github.com/labstack/echo/v4"
)

func backgroundTask() {}

func otherTask() {}

func worker(n int) { _ = n }

func workerCtx(ctx context.Context) { _ = ctx }

func workerDone(done <-chan struct{}) { <-done }

func workerCancel(cancel context.CancelFunc) { cancel() }

func workerAny(v any) { _ = v }

func workerConn(conn net.Conn) { _ = conn }

func workerRunner(r runner) { r.Run() }

func workerFunc(fn func()) { fn() }

func workerCmd(cmd *exec.Cmd) { _ = cmd.Wait() }

func workerEcho(c echo.Context) { _ = c }

// reqContext is an interface that embeds context.Context.
type reqContext interface {
	context.Context
	RequestID() string
}

func workerReq(c reqContext) { _ = c }

// loopServer holds no context.
type loopServer struct {
	n int
}

func (s *loopServer) loop() { s.n++ }

// ctxServer holds a context.
type ctxServer struct {
	ctx context.Context
}

func (s *ctxServer) run() { <-s.ctx.Done() }

func (s *ctxServer) Run() { s.run() }

// runner is an interface whose method a go statement calls.
type runner interface {
	Run()
}

// waitAll only waits and signals.
func waitAll(wg *sync.WaitGroup, done chan struct{}) {
	wg.Wait()
	close(done)
}

// waitGroup only waits and signals in its method.
type waitGroup struct {
	wg   sync.WaitGroup
	done chan struct{}
}

func (w *waitGroup) waitAll() {
	w.wg.Wait()
	close(w.done)
}

// [BAD]: A named function given nothing
func badBackgroundTask(ctx context.Context) {
	go backgroundTask() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: A named function given a value that is not a context
func badWorkerInt(ctx context.Context) {
	go worker(42) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: A method whose receiver holds no context
func badMethod(ctx context.Context, s *loopServer) {
	go s.loop() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: A method value whose receiver holds no context
func badMethodValue(ctx context.Context, s *loopServer) {
	f := s.loop
	go f() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: A method expression given a receiver that holds no context
func badMethodExpr(ctx context.Context, s *loopServer) {
	go (*loopServer).loop(s) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: A function of another package given no context
func badExternal(ctx context.Context, cmd *exec.Cmd) {
	go cmd.Wait() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: A named function given a func that captures no context
func badWorkerFuncLiteral(ctx context.Context) {
	go workerFunc(func() {}) // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Either of two named functions given no context
func badPhi(ctx context.Context, b bool) {
	f := backgroundTask
	if b {
		f = otherTask
	}
	go f() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: A carrier, but the named function is given nothing
func badEchoHandler(c echo.Context) {
	go backgroundTask() // want `goroutine does not propagate context "c"`
}

// [GOOD]: A named function given ctx
func goodWorkerCtx(ctx context.Context) {
	go workerCtx(ctx)
}

// [GOOD]: A named function given the channel of ctx.Done()
func goodWorkerDone(ctx context.Context) {
	go workerDone(ctx.Done())
}

// [GOOD]: A named function given a cancel func of ctx
func goodWorkerCancel(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go workerCancel(cancel)
	_ = ctx
}

// [GOOD]: A named function given ctx as any
func goodWorkerAny(ctx context.Context) {
	go workerAny(ctx)
}

// [GOOD]: A named function given a closure that captures ctx
func goodWorkerFunc(ctx context.Context) {
	go workerFunc(func() { _ = ctx })
}

// [GOOD]: A named function given a func it cannot follow
func goodWorkerFuncParam(ctx context.Context, fn func()) {
	go workerFunc(fn)
}

// [GOOD]: A named function given an *exec.Cmd made with ctx
func goodWorkerCmd(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "true")
	go workerCmd(cmd)
}

// [GOOD]: A method of an *exec.Cmd made with ctx
func goodCmdWait(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "true")
	go cmd.Wait()
}

// [GOOD]: A method whose receiver holds a context
func goodMethod(ctx context.Context, s *ctxServer) {
	go s.run()
}

// [GOOD]: A method value whose receiver holds a context
func goodMethodValue(ctx context.Context, s *ctxServer) {
	f := s.run
	go f()
}

// [GOOD]: A callee that takes a context, given a nil one
//
// Its signature takes a context, so the rule leaves it alone.
func goodWorkerCtxNil(ctx context.Context) {
	go workerCtx(nil)
}

// [GOOD]: A callee that takes a context, given a derived one
func goodWorkerCtxDerived(ctx context.Context) {
	go workerCtx(context.WithoutCancel(ctx))
}

// [BAD]: A named function given an interface of another module
//
// A net.Conn cannot hold a context the analyzed code put there.
func badWorkerConn(ctx context.Context, conn net.Conn) {
	go workerConn(conn) // want `goroutine does not propagate context "ctx"`
}

// [GOOD]: A named function given an interface that holds a context
func goodWorkerRunner(ctx context.Context) {
	go workerRunner(&ctxServer{ctx: ctx})
}

// [GOOD]: A callee that takes a carrier
func goodWorkerEcho(c echo.Context) {
	go workerEcho(c)
}

// [GOOD]: A callee that takes a carrier, given nil
func goodWorkerEchoNil(ctx context.Context) {
	go workerEcho(nil)
}

// [GOOD]: A callee that takes an interface that embeds context.Context
func goodWorkerReq(ctx context.Context) {
	go workerReq(nil)
}

// [GOOD]: A function literal
//
// It is judged as today, and it uses ctx.
func goodFuncLiteral(ctx context.Context) {
	go func() { _ = ctx }()
}

// [GOOD]: A func parameter
//
// It cannot be followed.
func goodFuncParam(ctx context.Context, fn func()) {
	go fn()
}

// [GOOD]: An interface method
func goodInterfaceMethod(ctx context.Context, r runner) {
	go r.Run()
}

// [GOOD]: Either of a named function and a closure that captures ctx
func goodPhiLiteral(ctx context.Context, b bool) {
	f := backgroundTask
	if b {
		f = func() { _ = ctx }
	}
	go f()
}

// [GOOD]: A named function given a value that holds a context
func goodWorkerHolder(ctx context.Context) {
	go workerAny(&ctxServer{ctx: ctx})
}

// [GOOD]: Ignored
func goodIgnored(ctx context.Context) {
	go backgroundTask() //goroutinectx:ignore goroutine // a lifecycle goroutine
}

// [GOOD]: A named function that only waits and signals
func goodWaiter(ctx context.Context, wg *sync.WaitGroup) {
	done := make(chan struct{})
	go waitAll(wg, done)
	<-done
}

// [GOOD]: A method value that only waits and signals
func goodWaiterMethodValue(ctx context.Context, w *waitGroup) {
	f := w.waitAll
	go f()
	<-w.done
}

// [GOOD]: No context in the function
func goodNoContext() {
	go backgroundTask()
}
