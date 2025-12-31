package probe_test

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/mpyw/goroutinectx/internal/probe"
)

func TestFuncLitOfLiteralKey(t *testing.T) {
	t.Parallel()

	// Helper to create a FuncLit
	makeFuncLit := func() *ast.FuncLit {
		return &ast.FuncLit{
			Type: &ast.FuncType{},
			Body: &ast.BlockStmt{},
		}
	}

	t.Run("INT index valid", func(t *testing.T) {
		t.Parallel()

		fl0 := makeFuncLit()
		fl1 := makeFuncLit()
		compLit := &ast.CompositeLit{
			Elts: []ast.Expr{fl0, fl1},
		}
		lit := &ast.BasicLit{Kind: token.INT, Value: "1"}

		result := probe.FuncLitOfLiteralKey(compLit, lit)
		if result != fl1 {
			t.Errorf("expected fl1, got %v", result)
		}
	})

	t.Run("INT index zero", func(t *testing.T) {
		t.Parallel()

		fl0 := makeFuncLit()
		compLit := &ast.CompositeLit{
			Elts: []ast.Expr{fl0},
		}
		lit := &ast.BasicLit{Kind: token.INT, Value: "0"}

		result := probe.FuncLitOfLiteralKey(compLit, lit)
		if result != fl0 {
			t.Errorf("expected fl0, got %v", result)
		}
	})

	t.Run("INT index out of range negative", func(t *testing.T) {
		t.Parallel()

		compLit := &ast.CompositeLit{
			Elts: []ast.Expr{makeFuncLit()},
		}
		lit := &ast.BasicLit{Kind: token.INT, Value: "-1"}

		result := probe.FuncLitOfLiteralKey(compLit, lit)
		if result != nil {
			t.Errorf("expected nil for negative index, got %v", result)
		}
	})

	t.Run("INT index out of range positive", func(t *testing.T) {
		t.Parallel()

		compLit := &ast.CompositeLit{
			Elts: []ast.Expr{makeFuncLit()},
		}
		lit := &ast.BasicLit{Kind: token.INT, Value: "5"}

		result := probe.FuncLitOfLiteralKey(compLit, lit)
		if result != nil {
			t.Errorf("expected nil for out of range index, got %v", result)
		}
	})

	t.Run("INT index invalid format", func(t *testing.T) {
		t.Parallel()

		compLit := &ast.CompositeLit{
			Elts: []ast.Expr{makeFuncLit()},
		}
		lit := &ast.BasicLit{Kind: token.INT, Value: "abc"}

		result := probe.FuncLitOfLiteralKey(compLit, lit)
		if result != nil {
			t.Errorf("expected nil for invalid int format, got %v", result)
		}
	})

	t.Run("INT index element not FuncLit", func(t *testing.T) {
		t.Parallel()

		compLit := &ast.CompositeLit{
			Elts: []ast.Expr{&ast.Ident{Name: "notFuncLit"}},
		}
		lit := &ast.BasicLit{Kind: token.INT, Value: "0"}

		result := probe.FuncLitOfLiteralKey(compLit, lit)
		if result != nil {
			t.Errorf("expected nil when element is not FuncLit, got %v", result)
		}
	})

	t.Run("STRING key valid", func(t *testing.T) {
		t.Parallel()

		fl := makeFuncLit()
		compLit := &ast.CompositeLit{
			Elts: []ast.Expr{
				&ast.KeyValueExpr{
					Key:   &ast.BasicLit{Kind: token.STRING, Value: `"mykey"`},
					Value: fl,
				},
			},
		}
		lit := &ast.BasicLit{Kind: token.STRING, Value: `"mykey"`}

		result := probe.FuncLitOfLiteralKey(compLit, lit)
		if result != fl {
			t.Errorf("expected fl, got %v", result)
		}
	})

	t.Run("STRING key not found", func(t *testing.T) {
		t.Parallel()

		compLit := &ast.CompositeLit{
			Elts: []ast.Expr{
				&ast.KeyValueExpr{
					Key:   &ast.BasicLit{Kind: token.STRING, Value: `"otherkey"`},
					Value: makeFuncLit(),
				},
			},
		}
		lit := &ast.BasicLit{Kind: token.STRING, Value: `"mykey"`}

		result := probe.FuncLitOfLiteralKey(compLit, lit)
		if result != nil {
			t.Errorf("expected nil when key not found, got %v", result)
		}
	})

	t.Run("STRING key value not FuncLit", func(t *testing.T) {
		t.Parallel()

		compLit := &ast.CompositeLit{
			Elts: []ast.Expr{
				&ast.KeyValueExpr{
					Key:   &ast.BasicLit{Kind: token.STRING, Value: `"mykey"`},
					Value: &ast.Ident{Name: "notFuncLit"},
				},
			},
		}
		lit := &ast.BasicLit{Kind: token.STRING, Value: `"mykey"`}

		result := probe.FuncLitOfLiteralKey(compLit, lit)
		if result != nil {
			t.Errorf("expected nil when value is not FuncLit, got %v", result)
		}
	})

	t.Run("STRING key element not KeyValueExpr", func(t *testing.T) {
		t.Parallel()

		compLit := &ast.CompositeLit{
			Elts: []ast.Expr{makeFuncLit()}, // Not KeyValueExpr
		}
		lit := &ast.BasicLit{Kind: token.STRING, Value: `"mykey"`}

		result := probe.FuncLitOfLiteralKey(compLit, lit)
		if result != nil {
			t.Errorf("expected nil when element is not KeyValueExpr, got %v", result)
		}
	})

	t.Run("STRING key key not BasicLit", func(t *testing.T) {
		t.Parallel()

		compLit := &ast.CompositeLit{
			Elts: []ast.Expr{
				&ast.KeyValueExpr{
					Key:   &ast.Ident{Name: "mykey"}, // Not BasicLit
					Value: makeFuncLit(),
				},
			},
		}
		lit := &ast.BasicLit{Kind: token.STRING, Value: `"mykey"`}

		result := probe.FuncLitOfLiteralKey(compLit, lit)
		if result != nil {
			t.Errorf("expected nil when key is not BasicLit, got %v", result)
		}
	})

	t.Run("unsupported token kind", func(t *testing.T) {
		t.Parallel()

		compLit := &ast.CompositeLit{
			Elts: []ast.Expr{makeFuncLit()},
		}
		lit := &ast.BasicLit{Kind: token.FLOAT, Value: "1.5"}

		result := probe.FuncLitOfLiteralKey(compLit, lit)
		if result != nil {
			t.Errorf("expected nil for unsupported token kind, got %v", result)
		}
	})
}
