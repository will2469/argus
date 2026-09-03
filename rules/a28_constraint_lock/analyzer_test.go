package a28_constraint_lock

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/will2469/argus/shared/directives"
)

func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("failed to resolve rootDir: %v", err)
	}

	analysistest.Run(t, rootDir, Analyzer,
		"./tests/migration/a28/positive",
		"./tests/migration/a28/negative",
	)
}

func TestCheckMigration_NotValidCompliant(t *testing.T) {
	sql := `
ALTER TABLE orders
ADD CONSTRAINT fk_user
FOREIGN KEY (user_id) REFERENCES users(id) NOT VALID;
`
	issues := CheckMigration("001_fk_not_valid.sql", sql, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for NOT VALID, got %d: %v", len(issues), issues)
	}
}

func TestCheckMigration_NewTableSameFile(t *testing.T) {
	sql := `
CREATE TABLE orders (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL
);

ALTER TABLE orders
ADD CONSTRAINT fk_user
FOREIGN KEY (user_id) REFERENCES users(id);
`
	issues := CheckMigration("002_new_table.sql", sql, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for new table in same file, got %d: %v", len(issues), issues)
	}
}

func TestCheckMigration_DirectFKViolation(t *testing.T) {
	sql := `
ALTER TABLE orders
ADD CONSTRAINT fk_user
FOREIGN KEY (user_id) REFERENCES users(id);
`
	issues := CheckMigration("003_direct_fk.sql", sql, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for direct FK without NOT VALID, got %d", len(issues))
	}
	if issues[0].Rule != RuleCode {
		t.Errorf("expected rule %s, got %s", RuleCode, issues[0].Rule)
	}
}

func TestCheckMigration_DirectCheckViolation(t *testing.T) {
	sql := `
ALTER TABLE accounts
ADD CONSTRAINT chk_balance
CHECK (balance >= 0);
`
	issues := CheckMigration("004_direct_chk.sql", sql, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for direct CHECK without NOT VALID, got %d", len(issues))
	}
}

func TestCheckMigration_Ignored(t *testing.T) {
	sql := `
-- argus:ignore ARGUS-A28 intentional legacy constraint
ALTER TABLE users
ADD CONSTRAINT fk_legacy
FOREIGN KEY (legacy_id) REFERENCES legacy_users(id);
`
	dm := directives.ParseSQLDirectives(sql, "005_legacy.sql")
	issues := CheckMigration("005_legacy.sql", sql, dm)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when ignored, got %d: %v", len(issues), issues)
	}
}

func TestScanMigrationDir_TestData(t *testing.T) {
	testDir := "../../tests/migration/a28/positive/migrations"
	issues, err := ScanMigrationDir(testDir)
	if err != nil {
		t.Fatalf("failed to scan testdata: %v", err)
	}

	if len(issues) != 2 {
		t.Fatalf("expected exactly 2 issues from testdata, got %d: %v", len(issues), issues)
	}
}
