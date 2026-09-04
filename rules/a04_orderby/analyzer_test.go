package a04_orderby

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func parseSnippet(t *testing.T, src string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse snippet: %v", err)
	}
	return fset, file
}

func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, rootDir, Analyzer,
		"./tests/correctness/a04/positive",
		"./tests/correctness/a04/negative",
	)
}

func TestExtractSortClauses(t *testing.T) {
	sql := "SELECT id, name FROM users ORDER BY created_at DESC, id ASC"
	clauses, err := ExtractSortClauses(sql)
	if err != nil {
		t.Fatalf("unexpected error parsing sort clauses: %v", err)
	}
	if len(clauses) != 2 {
		t.Fatalf("expected 2 sort clauses, got %d", len(clauses))
	}
	if clauses[0].ColumnName != "created_at" || clauses[0].Direction != "DESC" {
		t.Errorf("unexpected clause 0: %+v", clauses[0])
	}
	if clauses[1].ColumnName != "id" || clauses[1].Direction != "ASC" {
		t.Errorf("unexpected clause 1: %+v", clauses[1])
	}

	complexSQL := "SELECT id FROM users ORDER BY (CASE WHEN id = 1 THEN 1 ELSE 2 END)"
	complexClauses, err := ExtractSortClauses(complexSQL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(complexClauses) != 1 || !complexClauses[0].IsComplex {
		t.Errorf("expected complex sort clause, got %+v", complexClauses)
	}
}

func TestArbitraryMapLookup_Rejected(t *testing.T) {
	src := `package main
import "fmt"
func Query(userSort string, arbitraryMap map[string]string) {
	col := arbitraryMap[userSort]
	q := fmt.Sprintf("SELECT id FROM users ORDER BY %s ASC", col)
	_ = q
}`
	fset, file := parseSnippet(t, src)
	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for arbitrary map lookup, got %d", len(issues))
	}
}

func TestSwitchUnsafeDefault_Rejected(t *testing.T) {
	src := `package main
import "fmt"
func Query(userSort string) {
	var col string
	switch userSort {
	case "name":
		col = "name"
	case "date":
		col = "created_at"
	default:
		col = userSort
	}
	q := fmt.Sprintf("SELECT id FROM users ORDER BY %s ASC", col)
	_ = q
}`
	fset, file := parseSnippet(t, src)
	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for switch with unsafe default, got %d", len(issues))
	}
}

func TestDirectionUnsafeFallback_Rejected(t *testing.T) {
	src := `package main
import "fmt"
func Query(userDir string) {
	dir := userDir
	if userDir == "DESC" {
		dir = "DESC"
	}
	q := fmt.Sprintf("SELECT id FROM users ORDER BY id %s", dir)
	_ = q
}`
	fset, file := parseSnippet(t, src)
	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for direction unsafe fallback, got %d", len(issues))
	}
}

func TestAllowedColumns_Filtering(t *testing.T) {
	src := `package main
import "fmt"
var allowlist = map[string]string{
	"name":   "name",
	"secret": "password_hash",
}
func QueryAllowed(userSort string) {
	col := "id"
	q := fmt.Sprintf("SELECT id FROM users ORDER BY %s ASC", col)
	_ = q
}
func QueryDisallowed(userSort string) {
	col := "password_hash"
	q := fmt.Sprintf("SELECT id FROM users ORDER BY %s ASC", col)
	_ = q
}`
	fset, file := parseSnippet(t, src)
	allowed := []string{"id", "name", "created_at"}
	issues := InspectFileWithConfig(nil, fset, file, nil, allowed)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for disallowed column password_hash, got %d", len(issues))
	}
}
