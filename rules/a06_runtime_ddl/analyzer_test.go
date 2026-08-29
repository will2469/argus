package a06_runtime_ddl

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	testdata, err := filepath.Abs("../../testdata")
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, testdata, Analyzer, "a06")
}

func TestDetectRuntimeDDL_Compliant(t *testing.T) {
	dmls := []string{
		"SELECT id, name FROM users WHERE id = $1",
		"INSERT INTO orders (id, total) VALUES ($1, $2)",
		"UPDATE accounts SET balance = balance - $1 WHERE id = $2",
		"DELETE FROM tokens WHERE expires_at < NOW()",
	}

	for _, q := range dmls {
		if op := DetectDDLFromAST(q); op != "" {
			t.Errorf("expected DML query to be compliant, but detected DDL op %q for query %q", op, q)
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

func TestDetectDynamicDDL(t *testing.T) {
	cases := []struct {
		sql string
		op  string
	}{
		{"CREATE TABLE foo (id int)", "CREATE TABLE"},
		{"DROP TABLE foo", "DROP"},
		{"ALTER TABLE foo ADD col int", "ALTER TABLE"},
		{"TRUNCATE TABLE foo", "TRUNCATE"},
		{"GRANT SELECT ON foo TO bar", "GRANT/REVOKE"},
	}
	for _, tc := range cases {
		if op := IsDDLKeyword(tc.sql); op != tc.op {
			t.Errorf("expected %q, got %q", tc.op, op)
		}
	}
}
