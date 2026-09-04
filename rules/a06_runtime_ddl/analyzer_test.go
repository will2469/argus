package a06_runtime_ddl

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
		"./tests/correctness/a06/positive",
		"./tests/correctness/a06/negative",
	)
}

func TestDetectRuntimeDDL_Compliant(t *testing.T) {
	dmls := []string{
		"SELECT id, name FROM users WHERE id = $1",
		"INSERT INTO orders (id, total) VALUES ($1, $2)",
		"UPDATE accounts SET balance = balance - $1 WHERE id = $2",
		"DELETE FROM tokens WHERE expires_at < NOW()",
		"SELECT * FROM users WHERE name = 'CREATE TABLE'",
		"INSERT INTO logs (msg) VALUES ('DROP TABLE foo')",
	}

	for _, q := range dmls {
		if op := DetectDDLFromAST(q); op != "" {
			t.Errorf("expected DML query to be compliant, but detected DDL op %q for query %q", op, q)
		}
		if op := MatchDDLCommand(q); op != "" {
			t.Errorf("expected MatchDDLCommand to be empty for DML query %q, got %q", q, op)
		}
	}
}

func TestDetectRuntimeDDL_Violations(t *testing.T) {
	ddls := map[string]string{
		"CREATE TABLE temp_orders (id int)":     "CREATE TABLE",
		"DROP TABLE legacy_users":               "DROP",
		"ALTER TABLE users ADD COLUMN bio text": "ALTER TABLE",
		"TRUNCATE TABLE cached_tokens":          "TRUNCATE",
		"CREATE INDEX idx_user ON users(id)":    "CREATE INDEX",
		"GRANT ALL ON users TO app_user":        "GRANT/REVOKE",
		"CREATE SEQUENCE order_seq":             "CREATE SEQUENCE",
		"ALTER SEQUENCE order_seq RESTART":      "ALTER SEQUENCE",
		"SELECT 1; DROP TABLE users;":           "DROP",
	}

	for q, expectedOp := range ddls {
		op := DetectDDLFromAST(q)
		if op == "" {
			t.Errorf("expected DDL detection for query %q", q)
		}
		if op != expectedOp {
			t.Errorf("expected op %q, got %q for query %q", expectedOp, op, q)
		}
	}
}

func TestMatchDDLCommand(t *testing.T) {
	cases := []struct {
		sql string
		op  string
	}{
		{"CREATE TABLE foo (id int)", "CREATE TABLE"},
		{"DROP TABLE foo", "DROP"},
		{"ALTER TABLE foo ADD col int", "ALTER TABLE"},
		{"TRUNCATE TABLE foo", "TRUNCATE"},
		{"GRANT SELECT ON foo TO bar", "GRANT/REVOKE"},
		{"CREATE TABLE", "CREATE TABLE"},
		{"SELECT * FROM users WHERE note = 'CREATE TABLE'", ""},
		{"UPDATE users SET status = 'DROP TABLE'", ""},
	}
	for _, tc := range cases {
		if op := MatchDDLCommand(tc.sql); op != tc.op {
			t.Errorf("MatchDDLCommand(%q): expected %q, got %q", tc.sql, tc.op, op)
		}
	}
}

func TestDDLLatticeJoin_Branching(t *testing.T) {
	src := `package test
import "context"
type DB struct{}
func (DB) Exec(ctx context.Context, query string) (any, error) { return nil, nil }

func BranchDivergence(ctx context.Context, db DB, cond bool) {
	var query string
	if cond {
		query = "CREATE TABLE foo (id int)"
	} else {
		query = "SELECT 1"
	}
	db.Exec(ctx, query)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue from maybe-DDL lattice join, got %d: %v", len(issues), issues)
	}
	if issues[0].Message == "" {
		t.Errorf("expected non-empty error message")
	}
}

func TestDDLTracker_UnrelatedWriteString(t *testing.T) {
	src := `package test
import "context"
type DB struct{}
func (DB) Exec(ctx context.Context, query string) (any, error) { return nil, nil }

type Calculator struct{}
func (c *Calculator) WriteString(s string) {}
func (c *Calculator) String() string { return "" }

func SafeCalculatorUsage(ctx context.Context, db DB) {
	var calc Calculator
	calc.WriteString("CREATE TABLE fake (id int)")
	query := "SELECT 1"
	db.Exec(ctx, query)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for unrelated calculator WriteString, got %d: %v", len(issues), issues)
	}
}

