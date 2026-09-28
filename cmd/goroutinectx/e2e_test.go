package main_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build binary once for all tests
	tmpDir, err := os.MkdirTemp("", "goroutinectx-e2e-*")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	binaryPath = filepath.Join(tmpDir, "goroutinectx")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = filepath.Join(getModuleRoot(), "cmd", "goroutinectx")
	if out, err := cmd.CombinedOutput(); err != nil {
		panic(string(out) + ": " + err.Error())
	}

	os.Exit(m.Run())
}

func getModuleRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// Make sure it's the main module, not a testdata module
			if _, err := os.Stat(filepath.Join(dir, "analyzer.go")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("module root not found")
		}
		dir = parent
	}
}

func getE2ETestdata() string {
	return filepath.Join(getModuleRoot(), "cmd", "goroutinectx", "testdata")
}

func TestE2E_BasicGoroutine(t *testing.T) {
	testdata := filepath.Join(getE2ETestdata(), "basic")

	cmd := exec.Command(binaryPath, "./...")
	cmd.Dir = testdata
	out, err := cmd.CombinedOutput()

	// Should exit with non-zero (has diagnostics)
	if err == nil {
		t.Fatal("expected non-zero exit code for code with issues")
	}

	output := string(out)

	// Verify the expected diagnostic appears
	if !strings.Contains(output, `goroutine does not propagate context "ctx"`) {
		t.Errorf("expected goroutine propagation warning, got:\n%s", output)
	}

	// Verify it points to the bad function
	if !strings.Contains(output, "main.go:") {
		t.Errorf("expected file location in output, got:\n%s", output)
	}
}

func TestE2E_Errgroup(t *testing.T) {
	testdata := filepath.Join(getE2ETestdata(), "errgroup")

	cmd := exec.Command(binaryPath, "./...")
	cmd.Dir = testdata
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("expected non-zero exit code for code with issues")
	}

	output := string(out)

	if !strings.Contains(output, `errgroup.Group.Go() closure should use context "ctx"`) {
		t.Errorf("expected errgroup warning, got:\n%s", output)
	}
}

func TestE2E_DisableGoroutineChecker(t *testing.T) {
	testdata := filepath.Join(getE2ETestdata(), "basic")

	// Disable goroutine checker
	cmd := exec.Command(binaryPath, "-goroutine=false", "./...")
	cmd.Dir = testdata
	out, err := cmd.CombinedOutput()

	// Should exit with zero (no issues when goroutine checker is disabled)
	if err != nil {
		t.Errorf("expected zero exit code when goroutine checker disabled, got error: %v\noutput:\n%s", err, out)
	}
}

func TestE2E_DisableErrgroupChecker(t *testing.T) {
	testdata := filepath.Join(getE2ETestdata(), "errgroup")

	// Disable errgroup checker
	cmd := exec.Command(binaryPath, "-errgroup=false", "./...")
	cmd.Dir = testdata
	out, err := cmd.CombinedOutput()

	// Should exit with zero (no issues when errgroup checker is disabled)
	if err != nil {
		t.Errorf("expected zero exit code when errgroup checker disabled, got error: %v\noutput:\n%s", err, out)
	}
}

func TestE2E_HelpFlag(t *testing.T) {
	cmd := exec.Command(binaryPath, "-help")
	out, _ := cmd.CombinedOutput()

	output := string(out)

	// Should show usage info with our flags
	expectedFlags := []string{
		"-goroutine",
		"-goroutine-deriver",
		"-context-carriers",
		"-external-spawner",
		"-errgroup",
		"-waitgroup",
		"-conc",
		"-spawner",
		"-spawnerlabel",
		"-gotask",
	}

	for _, flag := range expectedFlags {
		if !strings.Contains(output, flag) {
			t.Errorf("expected flag %q in help output, got:\n%s", flag, output)
		}
	}
}

func TestE2E_NoIssuesExitZero(t *testing.T) {
	// Create a temp directory with clean code
	tmpDir, err := os.MkdirTemp("", "goroutinectx-clean-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create go.mod
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/clean\n\ngo 1.23\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create clean code
	cleanCode := `package main

import (
	"context"
	"fmt"
)

func main() {
	ctx := context.Background()
	work(ctx)
}

func work(ctx context.Context) {
	go func() {
		_ = ctx
		fmt.Println("work with context")
	}()
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(cleanCode), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binaryPath, "./...")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()

	if err != nil {
		t.Errorf("expected zero exit code for clean code, got error: %v\noutput:\n%s", err, out)
	}
}

func TestE2E_InvalidFlag(t *testing.T) {
	cmd := exec.Command(binaryPath, "-invalid-flag-xyz", "./...")
	_, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("expected non-zero exit code for invalid flag")
	}
}

func TestE2E_Version(t *testing.T) {
	// singlechecker doesn't have a version flag, but -V=full shows analyzer info
	cmd := exec.Command(binaryPath, "-V=full")
	out, err := cmd.CombinedOutput()

	if err != nil {
		t.Errorf("unexpected error: %v\noutput:\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "goroutinectx") {
		t.Errorf("expected analyzer name in version output, got:\n%s", output)
	}
}

func TestE2E_Spawner(t *testing.T) {
	testdata := filepath.Join(getE2ETestdata(), "spawner")

	cmd := exec.Command(binaryPath, "./...")
	cmd.Dir = testdata
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("expected non-zero exit code for code with issues")
	}

	output := string(out)

	// Should detect spawner directive issues
	if !strings.Contains(output, `runTask() func argument should use context "ctx"`) &&
		!strings.Contains(output, `runMultipleTasks() func argument should use context "ctx"`) {
		t.Errorf("expected spawner warning, got:\n%s", output)
	}
}

func TestE2E_Spawnerlabel(t *testing.T) {
	testdata := filepath.Join(getE2ETestdata(), "spawnerlabel")

	cmd := exec.Command(binaryPath, "-spawnerlabel=true", "./...")
	cmd.Dir = testdata
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("expected non-zero exit code for code with issues")
	}

	output := string(out)

	// Should detect missing spawner label
	if !strings.Contains(output, "//goroutinectx:spawner directive") {
		t.Errorf("expected spawnerlabel warning, got:\n%s", output)
	}
}

func TestE2E_GoroutineDerive(t *testing.T) {
	testdata := filepath.Join(getE2ETestdata(), "goroutinederive")

	cmd := exec.Command(binaryPath,
		"-goroutine-deriver=example.com/goroutinederive/apm.NewGoroutineContext",
		"./...",
	)
	cmd.Dir = testdata
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("expected non-zero exit code for code with issues")
	}

	output := string(out)

	// Should detect missing deriver call
	if !strings.Contains(output, "apm.NewGoroutineContext") {
		t.Errorf("expected deriver warning, got:\n%s", output)
	}
}

func TestE2E_DisableSpawnerChecker(t *testing.T) {
	testdata := filepath.Join(getE2ETestdata(), "spawner")

	// Disable spawner checker
	cmd := exec.Command(binaryPath, "-spawner=false", "./...")
	cmd.Dir = testdata
	out, err := cmd.CombinedOutput()

	// Should exit with zero when spawner checker is disabled
	if err != nil {
		t.Errorf("expected zero exit code when spawner checker disabled, got error: %v\noutput:\n%s", err, out)
	}
}

// TestE2E_LineDirective checks that //line directives do not move ignore
// directives, spawner directives, or the generated-file filter.
func TestE2E_LineDirective(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantOut  []string // stderr lines, with "<repro>" for the module directory
		wantExit int
	}{
		{
			name: "ignore/with",
			src: `package with

import "context"

//line fake.tmpl:1
func F(ctx context.Context) {
	//goroutinectx:ignore
	go func() {}()
}
`,
		},
		{
			name: "ignore/without",
			src: `package without

import "context"

func F(ctx context.Context) {
	//goroutinectx:ignore
	go func() {}()
}
`,
		},
		{
			name: "ignore/noignore",
			src: `package noignore

import "context"

func F(ctx context.Context) {
	go func() {}()
}
`,
			wantOut:  []string{`<repro>/ignore/noignore/a.go:6:2: goroutine does not propagate context "ctx"`},
			wantExit: 3,
		},
		{
			name: "gen/with",
			src: `// Code generated by hand for a repro. DO NOT EDIT.

package with

import "context"

//line fake.tmpl:1
func G(ctx context.Context) {
	go func() {}()
}
`,
		},
		{
			name: "gen/without",
			src: `// Code generated by hand for a repro. DO NOT EDIT.

package without

import "context"

func G(ctx context.Context) {
	go func() {}()
}
`,
		},
		{
			name: "coll/with",
			src: `package with

import "context"

//line a.tmpl:10
//goroutinectx:spawner
var X = 1

//line b.tmpl:11
func Run(f func()) { go f() }

func Use(ctx context.Context) {
	Run(func() {})
}
`,
		},
		{
			name: "coll/without",
			src: `package without

import "context"

//goroutinectx:spawner
var X = 1

func Run(f func()) { go f() }

func Use(ctx context.Context) {
	Run(func() {})
}
`,
		},
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/repro\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		pkgDir := filepath.Join(dir, filepath.FromSlash(tt.name))
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkgDir, "a.go"), []byte(tt.src), 0644); err != nil {
			t.Fatal(err)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "./"+tt.name)
			cmd.Dir = dir
			var stdout, stderr strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()

			exitCode := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}

			var gotOut []string
			for line := range strings.Lines(stderr.String()) {
				gotOut = append(gotOut, strings.ReplaceAll(strings.TrimSuffix(line, "\n"), dir, "<repro>"))
			}

			if stdout.String() != "" {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			if strings.Join(gotOut, "\n") != strings.Join(tt.wantOut, "\n") {
				t.Errorf("stderr:\n%s\nwant:\n%s", strings.Join(gotOut, "\n"), strings.Join(tt.wantOut, "\n"))
			}
			if exitCode != tt.wantExit {
				t.Errorf("exit code = %d, want %d", exitCode, tt.wantExit)
			}
		})
	}
}

// TestE2E_IgnoreForms runs the binary over the forms of //goroutinectx:ignore
// in #65. Each package holds one go statement that does not use ctx, under
// one ignore comment. Output lines are sorted, since the report on a directive
// and the report on the statement come from different passes over the file.
func TestE2E_IgnoreForms(t *testing.T) {
	const unknown = `unknown checker %q in goroutinectx:ignore (want one of goroutine, goroutinederive, waitgroup, errgroup, conc, spawner, spawnerlabel, gotask; write a reason after //)`
	const report = `<repro>/%s/a.go:7:2: goroutine does not propagate context "ctx"`
	tests := []struct {
		name     string
		comment  string
		wantOut  []string
		wantExit int
	}{
		{name: "bare", comment: "//goroutinectx:ignore"},
		{name: "dashreason", comment: "//goroutinectx:ignore - reason"},
		{name: "slashreason", comment: "//goroutinectx:ignore // reason"},
		{name: "checkerslashreason", comment: "//goroutinectx:ignore goroutine // reason"},
		{name: "checkerdashreason", comment: "//goroutinectx:ignore goroutine - reason"},
		{
			name: "reasonwithoutdash", comment: "//goroutinectx:ignore intentionally detached",
			wantOut: []string{
				fmt.Sprintf("<repro>/reasonwithoutdash/a.go:6:2: "+unknown, "intentionally detached"),
				fmt.Sprintf(report, "reasonwithoutdash"),
			},
			wantExit: 3,
		},
		{
			name: "checkerthenwords", comment: "//goroutinectx:ignore goroutine intentionally detached",
			wantOut: []string{
				fmt.Sprintf("<repro>/checkerthenwords/a.go:6:2: "+unknown, "goroutine intentionally detached"),
				fmt.Sprintf(report, "checkerthenwords"),
			},
			wantExit: 3,
		},
		{
			name: "misspelleddirective", comment: "//goroutinectx:ignre",
			wantOut: []string{
				"<repro>/misspelleddirective/a.go:6:2: unknown directive goroutinectx:ignre",
				fmt.Sprintf(report, "misspelleddirective"),
			},
			wantExit: 3,
		},
		{
			name: "misspelled", comment: "//goroutinectx:ignore gorutine",
			wantOut: []string{
				fmt.Sprintf("<repro>/misspelled/a.go:6:2: "+unknown, "gorutine"),
				fmt.Sprintf(report, "misspelled"),
			},
			wantExit: 3,
		},
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/repro\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		src := "package " + tt.name + "\n\nimport \"context\"\n\nfunc F(ctx context.Context) {\n\t" + tt.comment + "\n\tgo func() {}()\n}\n"
		if err := os.MkdirAll(filepath.Join(dir, tt.name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, tt.name, "a.go"), []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "./"+tt.name)
			cmd.Dir = dir
			var stdout, stderr strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()

			exitCode := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}

			var gotOut []string
			for line := range strings.Lines(stderr.String()) {
				gotOut = append(gotOut, strings.ReplaceAll(strings.TrimSuffix(line, "\n"), dir, "<repro>"))
			}
			slices.Sort(gotOut)
			want := slices.Sorted(slices.Values(tt.wantOut))

			if stdout.String() != "" {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			if !slices.Equal(gotOut, want) {
				t.Errorf("stderr:\n%s\nwant:\n%s", strings.Join(gotOut, "\n"), strings.Join(want, "\n"))
			}
			if exitCode != tt.wantExit {
				t.Errorf("exit code = %d, want %d", exitCode, tt.wantExit)
			}
		})
	}
}
