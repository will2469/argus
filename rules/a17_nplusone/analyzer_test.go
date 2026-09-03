package a17_nplusone

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("failed to resolve rootDir: %v", err)
	}

	analysistest.Run(t, rootDir, Analyzer,
		"./tests/correctness/a17/positive",
		"./tests/correctness/a17/negative",
	)
}

func TestTransitiveCallGraphDetection(t *testing.T) {
	src := `package testpkg

type DB struct{}
func (DB) Query(sql string) {}

func level1(db DB) { db.Query("SELECT 1") }
func level2(db DB) { level1(db) }
func level3(db DB) { level2(db) }
func level4(db DB) { level3(db) }
func unrelated() {}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	detector := NewHelperQueryDetector(nil, file)

	helpers := []string{"level1", "level2", "level3", "level4"}
	for _, h := range helpers {
		if !detector.funcHasQuery[h] {
			t.Errorf("expected %s to be marked as query helper across transitive calls", h)
		}
	}
	if detector.funcHasQuery["unrelated"] {
		t.Errorf("unrelated function must not be marked as query helper")
	}
}

func TestIsDBQueryCall_Rejection(t *testing.T) {
	makeCall := func(recvName, methodName string) *ast.CallExpr {
		return &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   &ast.Ident{Name: recvName},
				Sel: &ast.Ident{Name: methodName},
			},
			Args: []ast.Expr{
				&ast.Ident{Name: "ctx"},
				&ast.BasicLit{Kind: token.STRING, Value: `"query_str"`},
			},
		}
	}

	nonDBCalls := []string{"search", "searchengine", "httpClient", "metrics", "solr", "client", "req"}
	for _, name := range nonDBCalls {
		call := makeCall(name, "Query")
		if IsDBQueryCall(nil, call) {
			t.Errorf("call on %s.Query must NOT be identified as a DB query", name)
		}
	}

	dbCall := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: "db"},
			Sel: &ast.Ident{Name: "Query"},
		},
		Args: []ast.Expr{
			&ast.Ident{Name: "ctx"},
			&ast.BasicLit{Kind: token.STRING, Value: `"SELECT 1"`},
		},
	}
	if !IsDBQueryCall(nil, dbCall) {
		t.Errorf("call on db.Query must be identified as a DB query")
	}
}
