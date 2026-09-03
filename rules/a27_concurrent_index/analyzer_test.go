package a27_concurrent_index

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/will2469/argus/shared/directives"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("failed to resolve rootDir: %v", err)
	}

	analysistest.Run(t, rootDir, Analyzer,
		"./tests/migration/a27/positive",
		"./tests/migration/a27/negative",
	)
}

func TestCheckMigration_ConcurrentCompliant(t *testing.T) {
	sql := `
CREATE INDEX CONCURRENTLY idx_users_email ON users (email);
`
	issues := CheckMigration("001_idx.sql", sql, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d: %v", len(issues), issues)
	}
}

func TestCheckMigration_NewTableSameFile(t *testing.T) {
	sql := `
CREATE TABLE categories (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL
);

CREATE INDEX idx_categories_name ON categories (name);
`
	issues := CheckMigration("002_cat.sql", sql, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for new table in same file, got %d: %v", len(issues), issues)
	}
}

func TestCheckMigration_ExistingTableNonConcurrent(t *testing.T) {
	sql := `
CREATE INDEX idx_users_phone ON users (phone);
`
	issues := CheckMigration("003_users_phone.sql", sql, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Rule != RuleCode {
		t.Errorf("expected rule %s, got %s", RuleCode, issues[0].Rule)
	}
}

func TestCheckMigration_Ignored(t *testing.T) {
	sql := `
-- argus:ignore ARGUS-A27 intentional offline maintenance migration
CREATE INDEX idx_users_legacy ON users (legacy_id);
`
	dm := directives.ParseSQLDirectives(sql, "004_legacy.sql")
	issues := CheckMigration("004_legacy.sql", sql, dm)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when ignored, got %d: %v", len(issues), issues)
	}
}

func TestScanMigrationDir_Corpus(t *testing.T) {
	migDir := "../../tests/migration/a27/positive/migrations"
	issues, err := ScanMigrationDir(migDir)
	if err != nil {
		t.Fatalf("failed to scan migrations: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("expected exactly 1 issue from corpus, got %d: %v", len(issues), issues)
	}

	if !strings.Contains(issues[0].Filename, "001_unsafe_plain.up.sql") {
		t.Errorf("expected violation in 001_unsafe_plain.up.sql, got %s", issues[0].Filename)
	}
}

func TestCheckMigration_ParseError(t *testing.T) {
	badSQL := `CREAT TABLE malformed (;;;`
	issues := CheckMigration("005_bad.sql", badSQL, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for malformed SQL, got %d", len(issues))
	}
	if issues[0].Rule != "ARGUS-E001" {
		t.Errorf("expected rule ARGUS-E001, got %s", issues[0].Rule)
	}
	if !strings.Contains(issues[0].Message, "unable to analyze migration") {
		t.Errorf("unexpected error message: %s", issues[0].Message)
	}
}
