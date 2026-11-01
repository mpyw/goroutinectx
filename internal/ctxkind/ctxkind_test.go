package ctxkind

import (
	"go/token"
	"go/types"
	"testing"
)

func TestIsContext(t *testing.T) {
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
	named := func(name string, underlying types.Type) types.Type {
		return types.NewNamed(types.NewTypeName(token.NoPos, pkg, name, nil), underlying, nil)
	}
	myCtx := named("MyCtx", embedding(ctx))
	done := types.NewFunc(token.NoPos, pkg, "Done", types.NewSignatureType(nil, nil, nil, nil, nil, false))

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
		{name: "interface literal embedding context", typ: embedding(ctx), want: true},
		{name: "named interface embedding context", typ: myCtx, want: true},
		{name: "pointer to named interface embedding context", typ: types.NewPointer(myCtx), want: true},
		{name: "named interface embedding alias", typ: named("AliasCtx", embedding(alias)), want: true},
		{name: "nested named interface", typ: named("Outer", embedding(myCtx)), want: true},
		{name: "named interface constraint", typ: typeParam(myCtx), want: true},
		{name: "interface with a context method only", typ: named("Doner", types.NewInterfaceType([]*types.Func{done}, nil)), want: false},
		{name: "struct embedding context", typ: named("S", types.NewStruct(
			[]*types.Var{types.NewField(token.NoPos, pkg, "Context", ctx, true)}, nil)), want: false},
		{name: "named interface embedding other Context", typ: named("OtherCtx", embedding(other)), want: false},
		{name: "basic", typ: types.Typ[types.Int], want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isContext(tt.typ); got != tt.want {
				t.Errorf("isContext(%v) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
}

func TestIsCancelCall(t *testing.T) {
	t.Parallel()

	ctxPkg := types.NewPackage("context", "context")
	ctx := types.NewNamed(types.NewTypeName(token.NoPos, ctxPkg, "Context", nil), types.NewInterfaceType(nil, nil), nil)
	cancelFunc := types.NewNamed(types.NewTypeName(token.NoPos, ctxPkg, "CancelFunc", nil), types.NewSignatureType(nil, nil, nil, nil, nil, false), nil)
	param := func(typ types.Type) *types.Var { return types.NewParam(token.NoPos, ctxPkg, "", typ) }
	fn := func(pkg *types.Package, name string, results ...types.Type) *types.Func {
		vars := make([]*types.Var, len(results))
		for i, r := range results {
			vars[i] = param(r)
		}
		sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(param(ctx)), types.NewTuple(vars...), false)
		return types.NewFunc(token.NoPos, pkg, name, sig)
	}

	tests := []struct {
		name string
		fn   *types.Func
		want bool
	}{
		{name: "nil", fn: nil, want: false},
		{name: "WithCancel", fn: fn(ctxPkg, "WithCancel", ctx, cancelFunc), want: true},
		{name: "WithValue", fn: fn(ctxPkg, "WithValue", ctx), want: false},
		{name: "second result not a func", fn: fn(ctxPkg, "WithOther", ctx, types.Typ[types.Int]), want: false},
		{name: "x/net WithCancel", fn: fn(types.NewPackage("golang.org/x/net/context", "context"), "WithCancel", ctx, cancelFunc), want: true},
		{name: "other package", fn: fn(types.NewPackage("other", "other"), "WithCancel", ctx, cancelFunc), want: false},
	}
	for _, tt := range tests {
		if got := IsCancelCall(tt.fn); got != tt.want {
			t.Errorf("IsCancelCall(%s) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
