package carrier

import "testing"

func TestMatchPkg(t *testing.T) {
	tests := []struct {
		name      string
		pkgPath   string
		targetPkg string
		want      bool
	}{
		{
			name:      "exact match",
			pkgPath:   "github.com/example/pkg",
			targetPkg: "github.com/example/pkg",
			want:      true,
		},
		{
			name:      "version suffix v2",
			pkgPath:   "github.com/example/pkg/v2",
			targetPkg: "github.com/example/pkg",
			want:      true,
		},
		{
			name:      "version suffix v3",
			pkgPath:   "github.com/example/pkg/v3",
			targetPkg: "github.com/example/pkg",
			want:      true,
		},
		{
			name:      "version suffix v10",
			pkgPath:   "github.com/example/pkg/v10",
			targetPkg: "github.com/example/pkg",
			want:      true,
		},
		{
			name:      "no match - different pkg",
			pkgPath:   "github.com/other/pkg",
			targetPkg: "github.com/example/pkg",
			want:      false,
		},
		{
			name:      "no match - not a version suffix",
			pkgPath:   "github.com/example/pkg/subpkg",
			targetPkg: "github.com/example/pkg",
			want:      false,
		},
		{
			name:      "no match - version suffix without number",
			pkgPath:   "github.com/example/pkg/v",
			targetPkg: "github.com/example/pkg",
			want:      false,
		},
		{
			name:      "no match - version suffix with non-digit",
			pkgPath:   "github.com/example/pkg/vX",
			targetPkg: "github.com/example/pkg",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchPkg(tt.pkgPath, tt.targetPkg); got != tt.want {
				t.Errorf("matchPkg(%q, %q) = %v, want %v", tt.pkgPath, tt.targetPkg, got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
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
