package a30_timestamptz

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/will2469/argus/shared/directives"
)

func TestAnalyzer(t *testing.T) {
	testdata, err := filepath.Abs("../../testdata")
	if err != nil {
		t.Fatalf("failed to resolve testdata path: %v", err)
	}

	analysistest.Run(t, testdata, Analyzer, "a30")
}

func TestCheckMigration_TimestamptzCompliant(t *testing.T) {
	sql := `
CREATE TABLE events (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    event_date DATE NOT NULL
);
`
	issues := CheckMigration("001_events.sql", sql, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for TIMESTAMPTZ, got %d: %v", len(issues), issues)
	}
}

func TestCheckMigration_BareTimestampViolation(t *testing.T) {
	sql := `
CREATE TABLE logs (
    id UUID PRIMARY KEY,
    logged_at TIMESTAMP NOT NULL
);
`
	issues := CheckMigration("002_logs.sql", sql, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for bare TIMESTAMP, got %d: %v", len(issues), issues)
	}
	if issues[0].Rule != RuleCode {
		t.Errorf("expected rule %s, got %s", RuleCode, issues[0].Rule)
	}
}

func TestCheckMigration_AlterTableBareTimestampViolation(t *testing.T) {
	sql := `
ALTER TABLE logs ADD COLUMN archived_at TIMESTAMP;
`
	issues := CheckMigration("003_alter_logs.sql", sql, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for ALTER TABLE bare TIMESTAMP, got %d: %v", len(issues), issues)
	}
}

func TestCheckMigration_Ignored(t *testing.T) {
	sql := `
CREATE TABLE legacy_ticks (
    id INT PRIMARY KEY,
    -- argus:ignore ARGUS-A30 legacy wall-clock duration ticks
    tick_time TIMESTAMP NOT NULL
);
`
	dm := directives.ParseSQLDirectives(sql, "004_ticks.sql")
	issues := CheckMigration("004_ticks.sql", sql, dm)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when ignored, got %d: %v", len(issues), issues)
	}
}

func TestScanMigrationDir_TestData(t *testing.T) {
	testDir := "../../testdata/src/a30/migrations"
	issues, err := ScanMigrationDir(testDir)
	if err != nil {
		t.Fatalf("failed to scan testdata: %v", err)
	}

	// 000001 (timestamptz), 000002 (date/time), 000004 (ignored) -> only 000003 should fail
	if len(issues) != 1 {
		t.Fatalf("expected exactly 1 issue from testdata, got %d: %v", len(issues), issues)
	}
}

func TestCheckMigration_ParseError(t *testing.T) {
	badSQL := `ALTER TABEL corrupt SYNTAX;;;`
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

