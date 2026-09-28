//declscope:namespace e2e

package main_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// assignModule is a module whose diagnostics depend on finding the assignment
// of a variable in its file: func literals assigned to a variable (plain and
// in a branch), struct fields, slice indexes, factories, and gotask tasks.
// The gotask stub is copied from testdata/src.
var assignModule = map[string]string{
	"go.mod": `module example.com/e2e

go 1.23

require github.com/siketyan/gotask/v2 v2.0.0

replace github.com/siketyan/gotask/v2 => ./gotask
`,
	"gotask/go.mod": `module github.com/siketyan/gotask/v2

go 1.23
`,
	"apm/apm.go": `package apm

import "context"

func NewGoroutineContext(ctx context.Context) context.Context { return ctx }
`,
	"goroutines/a.go": `package goroutines

import "context"

func other(ctx context.Context) {
	fn := func() { _ = ctx }
	go fn()
}
`,
	"goroutines/b.go": `package goroutines

import "context"

type holder struct {
	run func()
}

func assigned(ctx context.Context) {
	fn := func() {}
	go fn()
}

func reassignedInBranch(ctx context.Context, b bool) {
	fn := func() { _ = ctx }
	if b {
		fn = func() {}
	}
	go fn()
}

func reassignedAfter(ctx context.Context) {
	fn := func() {}
	fn = func() { _ = ctx }
	go fn()
}

func field(ctx context.Context) {
	h := holder{run: func() {}}
	go h.run()
}

func fieldGood(ctx context.Context) {
	h := holder{run: func() { _ = ctx }}
	go h.run()
}

func index(ctx context.Context) {
	fns := []func(){func() { _ = ctx }, func() {}}
	go fns[1]()
}

func indexGood(ctx context.Context) {
	fns := []func(){func() { _ = ctx }, func() {}}
	go fns[0]()
}

func factory(ctx context.Context) {
	mk := func() func() { return func() {} }
	go mk()()
}

func factoryGood(ctx context.Context) {
	mk := func() func() { return func() { _ = ctx } }
	go mk()()
}
`,
	"tasks/tasks.go": `package tasks

import (
	"context"

	"example.com/e2e/apm"
	"github.com/siketyan/gotask/v2"
)

func taskVar(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error { return nil })
	_ = gotask.DoAllSettled(ctx, task)
}

func taskVarGood(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error {
		_ = apm.NewGoroutineContext(ctx)
		return nil
	})
	_ = gotask.DoAllSettled(ctx, task)
}

func fnVar(ctx context.Context) {
	fn := func(ctx context.Context) error { return nil }
	_ = gotask.DoAllFnsSettled(ctx, fn)
}

func fnVarGood(ctx context.Context) {
	fn := func(ctx context.Context) error {
		_ = apm.NewGoroutineContext(ctx)
		return nil
	}
	_ = gotask.DoAllFnsSettled(ctx, fn)
}

func derivedVar(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error { return nil })
	dctx := apm.NewGoroutineContext(ctx)
	task.DoAsync(dctx, nil)
}

func notDerivedVar(ctx context.Context) {
	task := gotask.NewTask(func(ctx context.Context) error { return nil })
	dctx := context.WithoutCancel(ctx)
	task.DoAsync(dctx, nil)
}
`,
}

func writeAssignModule(t *testing.T) string {
	t.Helper()

	// Resolve symlinks so the prefix matches the paths in the output
	// (on macOS the temp dir is under a symlink).
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range assignModule {
		writeFile(t, filepath.Join(dir, name), content)
	}

	stub, err := os.ReadFile(filepath.Join(getModuleRoot(), "testdata", "src", "github.com", "siketyan", "gotask", "v2", "gotask.go"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "gotask", "gotask.go"), string(stub))

	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runInModule(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()

	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off")
	out, err := cmd.CombinedOutput()

	code := 0
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}

	return strings.ReplaceAll(string(out), dir+string(filepath.Separator), ""), code
}

func TestE2E_AssignmentLookup(t *testing.T) {
	dir := writeAssignModule(t)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "goroutines",
			args: []string{"./goroutines/..."},
			want: `goroutines/b.go:11:2: goroutine does not propagate context "ctx"
goroutines/b.go:19:2: goroutine does not propagate context "ctx"
goroutines/b.go:30:2: goroutine does not propagate context "ctx"
goroutines/b.go:40:2: goroutine does not propagate context "ctx"
goroutines/b.go:50:2: goroutine does not propagate context "ctx"
`,
		},
		{
			name: "gotask",
			args: []string{"-goroutine-deriver=example.com/e2e/apm.NewGoroutineContext", "./tasks/..."},
			want: `tasks/tasks.go:12:6: gotask.DoAllSettled() 2nd argument should call goroutine deriver
tasks/tasks.go:25:6: gotask.DoAllFnsSettled() 2nd argument should call goroutine deriver
tasks/tasks.go:45:7: gotask.(*Task).DoAsync() 1st argument should call goroutine deriver
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, code := runInModule(t, dir, tt.args...)
			if code != 3 {
				t.Errorf("exit code = %d, want 3", code)
			}
			if got != tt.want {
				t.Errorf("output mismatch\ngot:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}
