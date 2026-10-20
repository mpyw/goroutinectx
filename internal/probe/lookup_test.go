package probe

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"
)

const lookupSrc = `package p

type S struct{ F func() }

func f() {
	s := S{F: func() {}}
	s.F()
	fs := []func(){func() {}}
	fs[0]()
}
`

// lookupContext type-checks lookupSrc and returns a Context over it, with the
// inspector the analyzer shares.
func lookupContext(t *testing.T) (*Context, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", lookupSrc, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Defs: map[*ast.Ident]types.Object{},
		Uses: map[*ast.Ident]types.Object{},
	}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("p", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}
	pass := &analysis.Pass{Fset: fset, Files: []*ast.File{file}, Pkg: pkg, TypesInfo: info}
	return &Context{Pass: pass, Inspector: inspector.New(pass.Files)}, file
}

// lookupVar returns the variable the named local declares, and the ident.
func lookupVar(t *testing.T, c *Context, name string) (*types.Var, *ast.Ident) {
	t.Helper()
	for id, obj := range c.Pass.TypesInfo.Defs {
		if v, ok := obj.(*types.Var); ok && id.Name == name {
			return v, id
		}
	}
	t.Fatalf("no variable %s", name)
	return nil, nil
}

func TestAssignmentLookups(t *testing.T) {
	t.Parallel()
	c, _ := lookupContext(t)

	t.Run("FuncLitAssignedToStructField", func(t *testing.T) {
		t.Parallel()
		s, _ := lookupVar(t, c, "s")
		if got := c.FuncLitAssignedToStructField(s, "F"); got == nil {
			t.Error("field F: got nil, want the func literal")
		}
		if got := c.FuncLitAssignedToStructField(s, "G"); got != nil {
			t.Errorf("field G: got %v, want nil", got)
		}
	})

	t.Run("FuncLitAssignedToIndex", func(t *testing.T) {
		t.Parallel()
		fs, _ := lookupVar(t, c, "fs")
		if got := c.FuncLitAssignedToIndex(fs, &ast.BasicLit{Kind: token.INT, Value: "0"}); got == nil {
			t.Error("index 0: got nil, want the func literal")
		}
		if got := c.FuncLitAssignedToIndex(fs, &ast.BasicLit{Kind: token.INT, Value: "1"}); got != nil {
			t.Errorf("index 1: got %v, want nil", got)
		}
	})
}

// TestAssignmentLookupsOutsideThePass checks a variable declared in no file of
// the pass, such as one from another package. There is no file to search, so
// every lookup finds nothing.
func TestAssignmentLookupsOutsideThePass(t *testing.T) {
	t.Parallel()
	c, _ := lookupContext(t)
	v := types.NewVar(token.NoPos, nil, "x", types.NewSignatureType(nil, nil, nil, nil, nil, false))
	if got := c.FuncLitAssignedToStructField(v, "F"); got != nil {
		t.Errorf("FuncLitAssignedToStructField: got %v", got)
	}
	if got := c.FuncLitAssignedToIndex(v, &ast.BasicLit{Kind: token.INT, Value: "0"}); got != nil {
		t.Errorf("FuncLitAssignedToIndex: got %v", got)
	}
}
