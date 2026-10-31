//declscope:namespace e2e

package main_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// goldenModules are the real modules that TestGolden analyzes, each at a
// pinned commit.
var goldenModules = []struct {
	name   string
	repo   string
	commit string
}{
	{"prometheus", "prometheus/prometheus", "9e28ec50ba751345a1f01a4fdc0d6887f533a37b"},
	{"grpc-go", "grpc/grpc-go", "bc90736a6fadc41582f9a23fb955bb66088f1bb8"},
	{"cloudflared", "cloudflare/cloudflared", "872b241f94871665c1c157adefcf90c11f0ead96"},
	{"minder", "mindersec/minder", "ff5d7dfde5d67d51857f239b82dc8a7af8cd376e"},
}

// goldenEnv fixes the build configuration, so that build constraints pick
// the same files on every machine.
var goldenEnv = []string{
	"GOOS=linux",
	"GOARCH=amd64",
	"CGO_ENABLED=0",
	"GOFLAGS=",
	"GOWORK=off",
	"GOTOOLCHAIN=local",
}

// goldenReport is one line of the analyzer's output.
var goldenReport = regexp.MustCompile(`^(.+):(\d+):(\d+): (.+)$`)

// TestGolden runs the analyzer over $GOROOT/src and the modules of
// goldenModules, and compares the sorted reports with
// testdata/golden/<name>.txt. A refactor keeps the reports the same.
//
// It runs only when GOROUTINECTX_GOLDEN is set: "1" checks, "update"
// rewrites the golden files. It needs git and the network the first time;
// the modules are kept in .cache/golden. The report over $GOROOT/src depends
// on the Go version, so that case is skipped unless the toolchain is the one
// in the header of testdata/golden/goroot.txt.
func TestGolden(t *testing.T) {
	mode := os.Getenv("GOROUTINECTX_GOLDEN")
	switch mode {
	case "":
		t.Skip("set GOROUTINECTX_GOLDEN=1 to check the golden reports, or GOROUTINECTX_GOLDEN=update to rewrite them")
	case "1", "update":
	default:
		t.Fatalf("GOROUTINECTX_GOLDEN=%q, want 1 or update", mode)
	}
	update := mode == "update"
	root := getModuleRoot()
	golden := filepath.Join(root, "cmd", "goroutinectx", "testdata", "golden")
	cache := filepath.Join(root, ".cache", "golden")

	t.Run("goroot", func(t *testing.T) {
		t.Parallel()
		goroot := goldenGoEnv(t, "GOROOT")
		version := goldenGoEnv(t, "GOVERSION")
		header := "# " + version + " linux/amd64"
		file := filepath.Join(golden, "goroot.txt")
		if !update {
			want, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if got, _, _ := strings.Cut(string(want), "\n"); got != header {
				t.Skipf("goroot.txt is for %q, this is %q: run with that toolchain, or update it", got, header)
			}
		}
		goldenCompare(t, file, header, goldenRun(t, filepath.Join(goroot, "src")), update)
	})
	for _, m := range goldenModules {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(cache, m.name)
			goldenFetch(t, dir, "https://github.com/"+m.repo, m.commit)
			header := "# github.com/" + m.repo + "@" + m.commit + " linux/amd64"
			goldenCompare(t, filepath.Join(golden, m.name+".txt"), header, goldenRun(t, dir), update)
		})
	}
}

// goldenGoEnv returns the value of a go env variable.
func goldenGoEnv(t *testing.T, name string) string {
	t.Helper()
	out, err := exec.Command("go", "env", name).Output()
	if err != nil {
		t.Fatalf("go env %s: %v", name, err)
	}
	return strings.TrimSpace(string(out))
}

// goldenFetch checks out commit of url in dir, unless it is there already.
func goldenFetch(t *testing.T, dir, url, commit string) {
	t.Helper()
	if out, err := goldenGit(dir, "rev-parse", "HEAD"); err == nil && out == commit {
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"fetch", "-q", "--depth", "1", url, commit},
		{"checkout", "-q", "--detach", "FETCH_HEAD"},
	} {
		if out, err := goldenGit(dir, args...); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
}

func goldenGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// goldenRun runs the analyzer over ./... in dir, and returns its reports as
// "path:line:col: message" lines with path relative to dir, sorted. Any
// other output fails the test.
func goldenRun(t *testing.T, dir string) []string {
	t.Helper()
	cmd := exec.Command(binaryPath, "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), goldenEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	// singlechecker exits 3 when it reports, and 1 on any other failure.
	if exit := (*exec.ExitError)(nil); err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 3) {
		t.Fatalf("goroutinectx ./... in %s: %v\n%s%s", dir, err, &stdout, &stderr)
	}
	var reports []string
	for line := range strings.Lines(stdout.String() + stderr.String()) {
		line = strings.TrimRight(line, "\n")
		m := goldenReport.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("goroutinectx printed a line that is not a report:\n%s", line)
		}
		rel, err := filepath.Rel(dir, m[1])
		if err != nil || !filepath.IsLocal(rel) {
			t.Fatalf("a report is outside %s:\n%s", dir, line)
		}
		reports = append(reports, fmt.Sprintf("%s:%s:%s: %s", filepath.ToSlash(rel), m[2], m[3], m[4]))
	}
	slices.Sort(reports)
	return reports
}

// goldenCompare compares got with the golden file, or rewrites the file
// when update is set.
func goldenCompare(t *testing.T, file, header string, got []string, update bool) {
	t.Helper()
	content := header + "\n" + strings.Join(append(got, ""), "\n")
	if update {
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == content {
		return
	}
	wantHeader, rest, _ := strings.Cut(string(data), "\n")
	if wantHeader != header {
		t.Errorf("golden header is %q, want %q", wantHeader, header)
	}
	want := strings.Split(strings.TrimSuffix(rest, "\n"), "\n")
	if rest == "" {
		want = nil
	}
	var diff strings.Builder
	for _, line := range want {
		if !slices.Contains(got, line) {
			fmt.Fprintf(&diff, "-%s\n", line)
		}
	}
	for _, line := range got {
		if !slices.Contains(want, line) {
			fmt.Fprintf(&diff, "+%s\n", line)
		}
	}
	if diff.Len() == 0 {
		diff.WriteString("(the same reports, a different number of times)\n")
	}
	t.Errorf("reports differ from %s (rerun with GOROUTINECTX_GOLDEN=update if the change is intended):\n%s",
		filepath.Base(file), diff.String())
}
