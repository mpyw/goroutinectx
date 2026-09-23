package directive

import "testing"

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		text     string
		wantOK   bool
		wantName string
		wantArgs string
	}{
		{name: "canonical", text: "//goroutinectx:ignore", wantOK: true, wantName: "ignore"},
		{name: "args", text: "//goroutinectx:ignore goroutine - reason ", wantOK: true, wantName: "ignore", wantArgs: "goroutine - reason"},
		{name: "spaced", text: "// goroutinectx:ignore", wantOK: false},
		{name: "tab after marker", text: "//\tgoroutinectx:spawner", wantOK: false},
		{name: "space after colon", text: "//goroutinectx: ignore", wantOK: false},
		{name: "block comment", text: "/*goroutinectx:ignore*/", wantOK: false},
		{name: "other tool", text: "//nolint:errcheck", wantOK: false},
		{name: "longer tool", text: "//goroutinectxx:ignore", wantOK: false},
		{name: "shorter tool", text: "//goroutine:ignore", wantOK: false},
		{name: "prose", text: "// goroutinectx is a linter", wantOK: false},
		{name: "uppercase tool", text: "//Goroutinectx:ignore", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d, ok := Parse(tt.text)
			if ok != tt.wantOK {
				t.Fatalf("Parse(%q) ok = %v, want %v", tt.text, ok, tt.wantOK)
			}
			if d.Name != tt.wantName || d.Args != tt.wantArgs {
				t.Errorf("Parse(%q) = {Name: %q, Args: %q}, want {Name: %q, Args: %q}",
					tt.text, d.Name, d.Args, tt.wantName, tt.wantArgs)
			}
		})
	}
}

func TestMalformed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		text   string
		want   string
		wantOK bool
	}{
		{name: "canonical", text: "//goroutinectx:ignore", wantOK: false},
		{name: "canonical with args", text: "//goroutinectx:spawner //vt:helper", wantOK: false},
		{name: "spaced", text: "// goroutinectx:ignore", want: "//goroutinectx:ignore", wantOK: true},
		{name: "spaced with args", text: "// goroutinectx:ignore goroutine - reason", want: "//goroutinectx:ignore", wantOK: true},
		{name: "tab after marker", text: "//\tgoroutinectx:spawner", want: "//goroutinectx:spawner", wantOK: true},
		{name: "space after colon", text: "//goroutinectx: ignore", want: "//goroutinectx:ignore", wantOK: true},
		{name: "block comment", text: "/*goroutinectx:ignore*/", want: "//goroutinectx:ignore", wantOK: true},
		{name: "spaced block comment", text: "/* goroutinectx:spawner */", want: "//goroutinectx:spawner", wantOK: true},
		{name: "no name", text: "//goroutinectx:", wantOK: true},
		{name: "no name spaced", text: "// goroutinectx:  ", wantOK: true},
		{name: "no name block", text: "/* goroutinectx: */", wantOK: true},
		{name: "uppercase name", text: "//goroutinectx:Ignore", wantOK: true},
		{name: "uppercase name spaced", text: "// goroutinectx:Spawner", wantOK: true},
		{name: "uppercase name after space", text: "//goroutinectx: Ignore", wantOK: true},
		{name: "longer tool", text: "// goroutinectxx:ignore", wantOK: false},
		{name: "other tool", text: "// nolint:errcheck", wantOK: false},
		{name: "prose", text: "// goroutinectx is a linter", wantOK: false},
		{name: "mentioned mid-comment", text: "// see //goroutinectx:ignore", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := Malformed(tt.text)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("Malformed(%q) = (%q, %v), want (%q, %v)", tt.text, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
