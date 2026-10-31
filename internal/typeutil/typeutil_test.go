package typeutil

import (
	"go/token"
	"go/types"
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
			name:      "no match - version-like package",
			pkgPath:   "github.com/example/pkg/v1alpha1",
			targetPkg: "github.com/example/pkg",
			want:      false,
		},
		{
			name:      "no match - package under a version suffix",
			pkgPath:   "github.com/example/pkg/v2/internal/x",
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

func TestIsContextType(t *testing.T) {
	t.Parallel()

	ctxPkg := types.NewPackage("context", "context")
	ctx := types.NewNamed(types.NewTypeName(token.NoPos, ctxPkg, "Context", nil), types.NewInterfaceType(nil, nil), nil)
	other := types.NewNamed(types.NewTypeName(token.NoPos, types.NewPackage("other", "other"), "Context", nil), types.NewInterfaceType(nil, nil), nil)

	pkg := types.NewPackage("p", "p")
	alias := types.NewAlias(types.NewTypeName(token.NoPos, pkg, "Ctx", nil), ctx)
	ptrAlias := types.NewAlias(types.NewTypeName(token.NoPos, pkg, "PCtx", nil), types.NewPointer(alias))
	typeParam := func(constraint types.Type) types.Type {
		return types.NewTypeParam(types.NewTypeName(token.NoPos, pkg, "C", nil), constraint)
	}
	embedding := func(embeddeds ...types.Type) types.Type {
		return types.NewInterfaceType(nil, embeddeds)
	}

	tests := []struct {
		name string
		typ  types.Type
		want bool
	}{
		{name: "context.Context", typ: ctx, want: true},
		{name: "pointer", typ: types.NewPointer(ctx), want: true},
		{name: "alias", typ: alias, want: true},
		{name: "pointer to alias", typ: types.NewPointer(alias), want: true},
		{name: "alias of pointer to alias", typ: ptrAlias, want: true},
		{name: "type parameter", typ: typeParam(ctx), want: true},
		{name: "type parameter of alias", typ: typeParam(alias), want: true},
		{name: "embedding constraint", typ: typeParam(embedding(alias)), want: true},
		{name: "nested embedding constraint", typ: typeParam(embedding(embedding(ctx))), want: true},
		{name: "other Context", typ: other, want: false},
		{name: "any type parameter", typ: typeParam(embedding()), want: false},
		{name: "named interface embedding context", typ: typeParam(types.NewNamed(
			types.NewTypeName(token.NoPos, pkg, "MyCtx", nil), embedding(ctx), nil)), want: false},
		{name: "basic", typ: types.Typ[types.Int], want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := IsContextType(tt.typ); got != tt.want {
				t.Errorf("IsContextType(%v) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
}
