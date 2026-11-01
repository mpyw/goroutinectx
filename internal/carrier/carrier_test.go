package carrier

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []Carrier
		wantErr bool
	}{
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "single carrier",
			input: "github.com/example/pkg.Type",
			want:  []Carrier{{PkgPath: "github.com/example/pkg", TypeName: "Type"}},
		},
		{
			name:  "multiple carriers",
			input: "github.com/a/pkg1.Type1,github.com/b/pkg2.Type2",
			want:  []Carrier{{PkgPath: "github.com/a/pkg1", TypeName: "Type1"}, {PkgPath: "github.com/b/pkg2", TypeName: "Type2"}},
		},
		{
			name:  "with spaces",
			input: " github.com/a/pkg1.Type1 , github.com/b/pkg2.Type2 ",
			want:  []Carrier{{PkgPath: "github.com/a/pkg1", TypeName: "Type1"}, {PkgPath: "github.com/b/pkg2", TypeName: "Type2"}},
		},
		{
			name:  "empty parts are skipped",
			input: "github.com/a/pkg.Type,,github.com/b/other.Type",
			want:  []Carrier{{PkgPath: "github.com/a/pkg", TypeName: "Type"}, {PkgPath: "github.com/b/other", TypeName: "Type"}},
		},
		{
			name:  "standard library package",
			input: "context.Context",
			want:  []Carrier{{PkgPath: "context", TypeName: "Context"}},
		},
		{
			name:    "no dot",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "no type name",
			input:   "github.com/example/pkg.",
			wantErr: true,
		},
		{
			name:    "no import path",
			input:   "echo.Context",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Split as the flag does, which drops the blank parts.
			var parts []string
			for part := range strings.SplitSeq(tt.input, ",") {
				if strings.TrimSpace(part) != "" {
					parts = append(parts, part)
				}
			}
			got, err := Parse(parts)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Parse(%q) returned %d carriers, want %d", tt.input, len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Parse(%q)[%d] = %+v, want %+v", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestParseRejectsItems checks that a blank item and an item that holds a
// comma are errors, as a list given as a list can hold them.
func TestParseRejectsItems(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{"an empty item", []string{"github.com/a/pkg.Type", ""}, "-context-carriers: item 2 is empty"},
		{"a blank item", []string{"  "}, "-context-carriers: item 1 is empty"},
		{
			"an item with a comma", []string{"github.com/a/pkg.Type,github.com/b/pkg.Type"},
			`-context-carriers: "github.com/a/pkg.Type,github.com/b/pkg.Type": each item is one type, and commas separate types only on the command line`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := Parse(tt.parts); err == nil || err.Error() != tt.want {
				t.Errorf("got error %v, want %q", err, tt.want)
			}
		})
	}
}
