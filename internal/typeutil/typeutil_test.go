package typeutil

import (
	"testing"
)

func TestPkgPathMatches(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			if got := PkgPathMatches(tt.pkgPath, tt.targetPkg); got != tt.want {
				t.Errorf("PkgPathMatches(%q, %q) = %v, want %v", tt.pkgPath, tt.targetPkg, got, tt.want)
			}
		})
	}
}
