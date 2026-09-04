package a02_unclosed_rows

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, rootDir, Analyzer,
		"./tests/correctness/a02/positive",
		"./tests/correctness/a02/negative",
	)
}

func TestInspectFile_ConditionalDeferTrap(t *testing.T) {
	src := `package testpkg
import "context"

type Rows interface{ Close() }
type DB interface{ Query(ctx context.Context, q string) (Rows, error) }

func TestConditional(ctx context.Context, db DB, cond bool) {
	rows, err := db.Query(ctx, "SELECT 1")
	if err != nil {
		return
	}
	if cond {
		defer rows.Close()
	}
	_ = rows
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 1 {
		t.Fatalf("expected conditional defer to be flagged, got %d issues: %+v", len(issues), issues)
	}
}

func TestInspectFile_AliasReassignmentLeak(t *testing.T) {
	src := `package testpkg
import "context"

type Rows interface{ Close() }
type DB interface{ 
	Query(ctx context.Context, q string) (Rows, error)
	Other() Rows
}

func TestReassign(ctx context.Context, db DB) {
	rows, _ := db.Query(ctx, "SELECT 1")
	other := rows

	rows = db.Other()
	defer rows.Close() // only closes db.Other(), not original query!

	_ = other
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 1 {
		t.Fatalf("expected leaked original query to be flagged, got %d issues: %+v", len(issues), issues)
	}
}

func TestInspectFile_CleanAlias(t *testing.T) {
	src := `package testpkg
import "context"

type Rows interface{ Close() }
type DB interface{ Query(ctx context.Context, q string) (Rows, error) }

func TestCleanAlias(ctx context.Context, db DB) {
	rows, err := db.Query(ctx, "SELECT 1")
	if err != nil {
		return
	}
	cursor := rows
	defer cursor.Close()
	_ = cursor
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 0 {
		t.Fatalf("expected clean alias defer to have 0 issues, got %d: %+v", len(issues), issues)
	}
}

func TestInspectFile_TypeDiscriminationNonDB(t *testing.T) {
	src := `package testpkg

type SearchEngine struct{}
func (SearchEngine) Query(q string) (string, error) { return q, nil }

func TestSearch(search SearchEngine) {
	res, _ := search.Query("term")
	_ = res
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 0 {
		t.Fatalf("expected non-DB query to produce 0 issues, got %d: %+v", len(issues), issues)
	}
}
