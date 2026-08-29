package a13_missing_down_migration

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	testdata, err := filepath.Abs("../../testdata")
	if err != nil {
		t.Fatalf("failed to resolve testdata path: %v", err)
	}

	analysistest.Run(t, testdata, Analyzer, "a13")
}

func TestScanDirectory_Compliant(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "001_users.up.sql"), []byte("CREATE TABLE users (id int);"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "001_users.down.sql"), []byte("DROP TABLE users;"), 0644)

	issues := ScanDirectory(tempDir, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for compliant pair, got %d: %v", len(issues), issues)
	}
}

func TestScanDirectory_MissingDown(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "001_users.up.sql"), []byte("CREATE TABLE users (id int);"), 0644)

	issues := ScanDirectory(tempDir, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for missing down migration, got %d", len(issues))
	}
	if issues[0].Rule != RuleCode {
		t.Errorf("expected rule %s, got %s", RuleCode, issues[0].Rule)
	}
}

func TestScanDirectory_EmptyDown(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "001_users.up.sql"), []byte("CREATE TABLE users (id int);"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "001_users.down.sql"), []byte(""), 0644)

	issues := ScanDirectory(tempDir, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for empty down migration, got %d", len(issues))
	}
}

func TestScanDirectory_Ignored(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{
			"CanonicalShortCode",
			"-- argus:ignore-a13 ADR-0042 data backfill irreversible\nSELECT 1;",
		},
		{
			"ClauseDotNotation",
			"-- argus:ignore-a13.irreversible ADR-0042 lossy data transformation\nSELECT 1;",
		},
		{
			"LegacyLongCode",
			"-- argus:ignore ARGUS-A13 ADR-0042 data backfill irreversible\nSELECT 1;",
		},
	}

	for _, tc := range cases {
		tempDir := t.TempDir()
		_ = os.WriteFile(filepath.Join(tempDir, "001_users.up.sql"), []byte("CREATE TABLE users (id int);"), 0644)
		_ = os.WriteFile(filepath.Join(tempDir, "001_users.down.sql"), []byte(tc.sql), 0644)

		issues := ScanDirectory(tempDir, nil)
		if len(issues) != 0 {
			t.Fatalf("[%s] expected 0 issues when ignored, got %d: %v", tc.name, len(issues), issues)
		}
	}
}
