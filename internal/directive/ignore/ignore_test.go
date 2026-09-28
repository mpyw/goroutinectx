package ignore

import (
	"slices"
	"strings"
	"testing"
)

func TestParseComment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		text    string
		want    []CheckerName
		unknown string // the unknown checker the problem names
		wantOK  bool
	}{
		{name: "canonical", text: "//goroutinectx:ignore", wantOK: true},
		{name: "spaced", text: "// goroutinectx:ignore", wantOK: false},
		{name: "trailing space", text: "//goroutinectx:ignore  ", wantOK: true},
		{name: "one checker", text: "//goroutinectx:ignore goroutine", want: []CheckerName{Goroutine}, wantOK: true},
		{name: "spaced with checker", text: "// goroutinectx:ignore goroutine", wantOK: false},
		{name: "block comment", text: "/* goroutinectx:ignore */", wantOK: false},
		{name: "checker list", text: "//goroutinectx:ignore goroutine,errgroup", want: []CheckerName{Goroutine, Errgroup}, wantOK: true},
		{name: "checker list with spaces", text: "//goroutinectx:ignore goroutine, errgroup", want: []CheckerName{Goroutine, Errgroup}, wantOK: true},
		{name: "dash reason", text: "//goroutinectx:ignore - fire-and-forget", wantOK: true},
		{name: "bare dash", text: "//goroutinectx:ignore -", wantOK: true},
		{name: "checker with dash reason", text: "//goroutinectx:ignore goroutine - fire-and-forget", want: []CheckerName{Goroutine}, wantOK: true},
		{name: "checker with slash reason", text: "//goroutinectx:ignore errgroup // want `x`", want: []CheckerName{Errgroup}, wantOK: true},
		{name: "checker list with slash reason", text: "//goroutinectx:ignore goroutine,errgroup //reason", want: []CheckerName{Goroutine, Errgroup}, wantOK: true},
		{name: "bare slash reason", text: "//goroutinectx:ignore // fire-and-forget", wantOK: true},
		{name: "bare slash reason without space", text: "//goroutinectx:ignore //fire-and-forget", wantOK: true},
		{name: "bare want comment", text: "//goroutinectx:ignore // want `x`", wantOK: true},
		{name: "slash reason holding a URL", text: "//goroutinectx:ignore // see https://example.com", wantOK: true},
		{name: "conc", text: "//goroutinectx:ignore conc", want: []CheckerName{Conc}, wantOK: true},
		{name: "reason without dash", text: "//goroutinectx:ignore intentionally detached", unknown: "intentionally detached", wantOK: true},
		{name: "checker then reason without dash", text: "//goroutinectx:ignore goroutine intentionally detached", unknown: "goroutine intentionally detached", wantOK: true},
		{name: "misspelled checker", text: "//goroutinectx:ignore gorutine", unknown: "gorutine", wantOK: true},
		{name: "one misspelled in a list", text: "//goroutinectx:ignore goroutine,typo", unknown: "typo", wantOK: true},
		{name: "longer name", text: "//goroutinectx:ignored", wantOK: false},
		{name: "longer name with args", text: "//goroutinectx:ignoregoroutine", wantOK: false},
		{name: "longer tool", text: "//goroutinectxx:ignore", wantOK: false},
		{name: "other directive", text: "//goroutinectx:spawner", wantOK: false},
		{name: "other tool", text: "//nolint:ignore", wantOK: false},
		{name: "plain comment", text: "// ignore this", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, problem, ok := parseComment(tt.text)
			if ok != tt.wantOK {
				t.Fatalf("parseComment(%q) ok = %v, want %v", tt.text, ok, tt.wantOK)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("parseComment(%q) = %q, want %q", tt.text, got, tt.want)
			}
			if tt.unknown == "" && problem != "" {
				t.Errorf("parseComment(%q) problem = %q, want none", tt.text, problem)
			}
			if tt.unknown != "" && !strings.HasPrefix(problem, "unknown checker \""+tt.unknown+"\" in goroutinectx:ignore (want one of ") {
				t.Errorf("parseComment(%q) problem = %q, want one naming %q", tt.text, problem, tt.unknown)
			}
		})
	}
}
