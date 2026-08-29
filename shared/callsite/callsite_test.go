package callsite

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestIsDBQueryMethod(t *testing.T) {
	cases := []struct {
		name     string
		expected bool
	}{
		{"Query", true},
		{"QueryRow", true},
		{"Exec", true},
		{"ExecContext", true},
		{"SendBatch", true},
		{"CopyFrom", true},
		{"Begin", true},
		{"Println", false},
		{"Format", false},
	}

	for _, tc := range cases {
		if got := IsDBQueryMethod(tc.name); got != tc.expected {
			t.Errorf("IsDBQueryMethod(%q) = %v, expected %v", tc.name, got, tc.expected)
		}
	}
}

func TestExtractQueryString_DirectAndConcat(t *testing.T) {
	src := `package main
func main() {
	pool.Query(ctx, "SELECT id FROM users")
	pool.Exec(ctx, "SELECT id, name " + "FROM accounts WHERE id = $1")
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	var queries []string
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if q, found := ExtractQueryString(call); found {
				queries = append(queries, q)
			}
		}
		return true
	})

	if len(queries) != 2 {
		t.Fatalf("expected 2 queries extracted, got %d", len(queries))
	}
	if queries[0] != "SELECT id FROM users" {
		t.Errorf("query 1 mismatch: got %q", queries[0])
	}
	if queries[1] != "SELECT id, name FROM accounts WHERE id = $1" {
		t.Errorf("query 2 (concat) mismatch: got %q", queries[1])
	}
}

func TestGetCallSelector_Generics(t *testing.T) {
	src := `package main
func main() {
	repo.Query[User](ctx, "SELECT * FROM users")
	repo.QueryRow[Key, Value](ctx, "SELECT * FROM kv")
	repo.Exec(ctx, "DELETE FROM logs")
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	var methodNames []string
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			name := GetCallMethodName(call.Fun)
			if name != "" {
				methodNames = append(methodNames, name)
			}
		}
		return true
	})

	if len(methodNames) != 3 {
		t.Fatalf("expected 3 methods extracted, got %d", len(methodNames))
	}
	if methodNames[0] != "Query" {
		t.Errorf("expected generic method Query, got %s", methodNames[0])
	}
	if methodNames[1] != "QueryRow" {
		t.Errorf("expected generic multi-arg method QueryRow, got %s", methodNames[1])
	}
	if methodNames[2] != "Exec" {
		t.Errorf("expected plain method Exec, got %s", methodNames[2])
	}
}

func TestIsInsideLoop(t *testing.T) {
	forNode := &ast.ForStmt{}
	callNode := &ast.CallExpr{}
	path := []ast.Node{forNode, callNode}

	if !IsInsideLoop(path) {
		t.Errorf("expected IsInsideLoop to be true")
	}

	pathNoLoop := []ast.Node{&ast.BlockStmt{}, callNode}
	if IsInsideLoop(pathNoLoop) {
		t.Errorf("expected IsInsideLoop to be false")
	}
}
