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

func mk() func() { return nil }

func f(cond bool) {
	g := func() {}
	if cond {
		g = func() {}
	}
	g()
	h := mk()
	h()
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

// lookupFuncLitAssignments lists the func literals assigned by := or = in file, in
// source order.
func lookupFuncLitAssignments(file *ast.File) []*ast.AssignStmt {
	var out []*ast.AssignStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if a, ok := n.(*ast.AssignStmt); ok {
			if _, ok := a.Rhs[0].(*ast.FuncLit); ok {
				out = append(out, a)
			}
		}
		return true
	})
	return out
}

func TestAssignmentLookups(t *testing.T) {
	t.Parallel()
	c, file := lookupContext(t)
	g, gIdent := lookupVar(t, c, "g")
	assigns := lookupFuncLitAssignments(file)
	first, second := assigns[0].Rhs[0].(*ast.FuncLit), assigns[1].Rhs[0].(*ast.FuncLit)

	t.Run("FuncLitAssignedTo", func(t *testing.T) {
		t.Parallel()
		if got := c.FuncLitAssignedTo(g, token.NoPos); got != second {
			t.Errorf("last assignment: got %v, want the one in the branch", got)
		}
		if got := c.FuncLitAssignedTo(g, assigns[1].Pos()); got != first {
			t.Errorf("before the branch: got %v, want the first", got)
		}
		if got := c.FuncLitAssignedToIdent(gIdent); got != second {
			t.Errorf("by ident: got %v, want the one in the branch", got)
		}
	})

	t.Run("FuncLitsAssignedTo", func(t *testing.T) {
		t.Parallel()
		if got := c.FuncLitsAssignedTo(g, token.NoPos); len(got) != 2 || got[0] != first || got[1] != second {
			t.Errorf("all assignments: got %v", got)
		}
		if got := c.FuncLitsAssignedTo(g, assigns[1].Pos()); len(got) != 1 || got[0] != first {
			t.Errorf("before the branch: got %v", got)
		}
		if got := c.FuncLitsAssignedToIdent(gIdent); len(got) != 2 {
			t.Errorf("by ident: got %v", got)
		}
	})

	t.Run("FuncLitAssignmentsTo", func(t *testing.T) {
		t.Parallel()
		got := c.FuncLitAssignmentsTo(g, token.NoPos)
		if len(got) != 2 || got[0].Conditional || !got[1].Conditional {
			t.Errorf("all assignments: got %+v, want one unconditional then one conditional", got)
		}
		if got := c.FuncLitAssignmentsTo(g, assigns[1].Pos()); len(got) != 1 || got[0].Lit != first {
			t.Errorf("before the branch: got %+v", got)
		}
		if got := c.FuncLitAssignmentsOfIdent(gIdent); len(got) != 2 {
			t.Errorf("by ident: got %+v", got)
		}
	})

	t.Run("CallExprAssignedTo", func(t *testing.T) {
		t.Parallel()
		h, hIdent := lookupVar(t, c, "h")
		call := c.CallExprAssignedTo(h, token.NoPos)
		if fn, ok := call.Fun.(*ast.Ident); !ok || fn.Name != "mk" {
			t.Errorf("got %v, want the call to mk", call)
		}
		if got := c.CallExprAssignedTo(h, hIdent.Pos()); got != nil {
			t.Errorf("before the assignment: got %v, want nil", got)
		}
		if got := c.CallExprAssignedToIdent(hIdent); got != call {
			t.Errorf("by ident: got %v", got)
		}
	})

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
	if got := c.FuncLitAssignedTo(v, token.NoPos); got != nil {
		t.Errorf("FuncLitAssignedTo: got %v", got)
	}
	if got := c.FuncLitsAssignedTo(v, token.NoPos); got != nil {
		t.Errorf("FuncLitsAssignedTo: got %v", got)
	}
	if got := c.FuncLitAssignmentsTo(v, token.NoPos); got != nil {
		t.Errorf("FuncLitAssignmentsTo: got %v", got)
	}
	if got := c.CallExprAssignedTo(v, token.NoPos); got != nil {
		t.Errorf("CallExprAssignedTo: got %v", got)
	}
	if got := c.FuncLitAssignedToStructField(v, "F"); got != nil {
		t.Errorf("FuncLitAssignedToStructField: got %v", got)
	}
	if got := c.FuncLitAssignedToIndex(v, &ast.BasicLit{Kind: token.INT, Value: "0"}); got != nil {
		t.Errorf("FuncLitAssignedToIndex: got %v", got)
	}
}
