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

			got, err := Parse(strings.Split(tt.input, ","))
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
