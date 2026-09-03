package a05_audit_immutability

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/will2469/argus/shared/directives"
)

func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, rootDir, Analyzer,
		"./tests/correctness/a05/positive",
		"./tests/correctness/a05/negative",
	)
}

func TestCheckSQLTampering_Compliant(t *testing.T) {
	tables := map[string]bool{"audit_logs": true}
	op, _ := CheckSQLTampering("INSERT INTO audit_logs (action) VALUES ('LOGIN')", tables)
	if op != "" {
		t.Fatalf("expected empty op for INSERT, got %s", op)
	}
	op, _ = CheckSQLTampering("SELECT * FROM audit_logs WHERE id = 1", tables)
	if op != "" {
		t.Fatalf("expected empty op for SELECT, got %s", op)
	}
}

func TestCheckSQLTampering_Violations(t *testing.T) {
	tables := map[string]bool{"audit_logs": true}

	queries := []string{
		"UPDATE audit_logs SET action = 'HACK' WHERE id = 1",
		"DELETE FROM audit_logs WHERE id = 1",
		"TRUNCATE TABLE audit_logs",
		"DROP TABLE audit_logs",
		"MERGE INTO audit_logs a USING temp_logs t ON a.id = t.id WHEN MATCHED THEN UPDATE SET action = 'HACK'",
		"WITH del AS (DELETE FROM audit_logs WHERE id = 1) SELECT 1",
	}

	for _, q := range queries {
		op, tbl := CheckSQLTampering(q, tables)
		if op == "" {
			t.Errorf("expected violation for query %q", q)
		}
		if tbl != "audit_logs" {
			t.Errorf("expected table audit_logs, got %s", tbl)
		}
	}
}

func TestCheckMigration_Ignored(t *testing.T) {
	tables := map[string]bool{"audit_logs": true}
	sql := `
-- argus:ignore ARGUS-A05 maintenance archive rotation
TRUNCATE TABLE audit_logs;
`
	dm := directives.ParseSQLDirectives(sql, "001_trunc.sql")
	issues := CheckMigration("001_trunc.sql", sql, dm, tables)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when ignored, got %d: %v", len(issues), issues)
	}
}
