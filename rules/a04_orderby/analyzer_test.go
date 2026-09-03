package a04_orderby

import (
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
