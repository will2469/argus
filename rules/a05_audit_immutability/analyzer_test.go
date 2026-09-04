package a05_audit_immutability

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/will2469/argus/shared/directives"
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

func TestDataArgumentsNotExtractedAsQuery(t *testing.T) {
	src := `package main
import "context"
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}
func Record(ctx context.Context, db DB) {
	// 3rd arg resembles SQL, but is a data parameter for $1
	db.Exec(ctx, "INSERT INTO audit_logs (id) VALUES ($1)", "DELETE FROM audit_logs WHERE id = '1'")
}`
	fset, file := parseSnippet(t, src)
	issues := InspectFile(nil, fset, file, nil, map[string]bool{"audit_logs": true})
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues (data arg must not be parsed as query), got %d: %+v", len(issues), issues)
	}
}

func TestSchemaQualificationMatching(t *testing.T) {
	tables := map[string]bool{
		"audit_logs":   true,
		"audit.ledger": true,
	}

	// 1. Unqualified table matches
	op, _ := CheckSQLTampering("UPDATE audit_logs SET a = 1", tables)
	if op != "UPDATE" {
		t.Errorf("expected UPDATE on audit_logs, got %s", op)
	}

	// 2. Default public schema matches unqualified definition
	op, _ = CheckSQLTampering("UPDATE public.audit_logs SET a = 1", tables)
	if op != "UPDATE" {
		t.Errorf("expected UPDATE on public.audit_logs, got %s", op)
	}

	// 3. Non-public schema collision avoided (evil_schema.audit_logs != audit_logs)
	op, _ = CheckSQLTampering("UPDATE evil_schema.audit_logs SET a = 1", tables)
	if op != "" {
		t.Errorf("expected no violation for evil_schema.audit_logs, got %s", op)
	}

	// 4. Explicit schema match
	op, _ = CheckSQLTampering("UPDATE audit.ledger SET a = 1", tables)
	if op != "UPDATE" {
		t.Errorf("expected UPDATE on audit.ledger, got %s", op)
	}

	// 5. Explicit schema mismatch avoided (other.ledger != audit.ledger)
	op, _ = CheckSQLTampering("UPDATE other.ledger SET a = 1", tables)
	if op != "" {
		t.Errorf("expected no violation for other.ledger, got %s", op)
	}
}

func TestInspectMigrationDir_DownOption(t *testing.T) {
	dir := t.TempDir()
	upSQL := "CREATE TABLE audit_logs (id int);"
	downSQL := "DROP TABLE audit_logs;"

	_ = os.WriteFile(filepath.Join(dir, "001.up.sql"), []byte(upSQL), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "001.down.sql"), []byte(downSQL), 0o600)

	tables := map[string]bool{"audit_logs": true}

	// Default: checkDown = false -> .down.sql not inspected
	issuesDefault := InspectMigrationDir(dir, nil, tables, false)
	if len(issuesDefault) != 0 {
		t.Errorf("expected 0 issues when checkDown is false, got %d", len(issuesDefault))
	}

	// Explicit: checkDown = true -> .down.sql inspected and DROP is flagged
	issuesWithDown := InspectMigrationDir(dir, nil, tables, true)
	if len(issuesWithDown) != 1 {
		t.Errorf("expected 1 issue when checkDown is true, got %d", len(issuesWithDown))
	}
}
