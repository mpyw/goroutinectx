package carrier

import (
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []Carrier
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
			input: "pkg1.Type1,pkg2.Type2",
			want:  []Carrier{{PkgPath: "pkg1", TypeName: "Type1"}, {PkgPath: "pkg2", TypeName: "Type2"}},
		},
		{
			name:  "with spaces",
			input: " pkg1.Type1 , pkg2.Type2 ",
			want:  []Carrier{{PkgPath: "pkg1", TypeName: "Type1"}, {PkgPath: "pkg2", TypeName: "Type2"}},
		},
		{
			name:  "invalid format - no dot",
			input: "invalid",
			want:  []Carrier{},
		},
		{
			name:  "empty parts are skipped",
			input: "pkg.Type,,other.Type",
			want:  []Carrier{{PkgPath: "pkg", TypeName: "Type"}, {PkgPath: "other", TypeName: "Type"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Parse(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("Parse(%q) returned %d carriers, want %d", tt.input, len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Parse(%q)[%d] = %+v, want %+v", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
