package a13_missing_down_migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("failed to resolve rootDir: %v", err)
	}

	analysistest.Run(t, rootDir, Analyzer,
		"./tests/migration/a13/positive",
		"./tests/migration/a13/negative",
	)
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

func TestScanDirectory_RecursiveSubdirectories(t *testing.T) {
	tempDir := t.TempDir()

	serviceA := filepath.Join(tempDir, "service_a")
	serviceB := filepath.Join(tempDir, "service_b")
	_ = os.MkdirAll(serviceA, 0755)
	_ = os.MkdirAll(serviceB, 0755)

	// service_a has a compliant pair
	_ = os.WriteFile(filepath.Join(serviceA, "001_users.up.sql"), []byte("CREATE TABLE users (id int);"), 0644)
	_ = os.WriteFile(filepath.Join(serviceA, "001_users.down.sql"), []byte("DROP TABLE users;"), 0644)

	// service_b has a missing down migration
	_ = os.WriteFile(filepath.Join(serviceB, "001_invoices.up.sql"), []byte("CREATE TABLE invoices (id int);"), 0644)

	issues := ScanDirectory(tempDir, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for recursive missing down migration in service_b, got %d: %v", len(issues), issues)
	}
	expectedFile := filepath.Join(serviceB, "001_invoices.up.sql")
	if issues[0].Filename != expectedFile {
		t.Errorf("expected issue on %s, got %s", expectedFile, issues[0].Filename)
	}
}

func TestScanDirectory_AsymmetricDummySelect(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "001_users.up.sql"), []byte("CREATE TABLE users (id int);"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "001_users.down.sql"), []byte("SELECT 1;"), 0644)

	issues := ScanDirectory(tempDir, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for asymmetric SELECT 1 rollback, got %d", len(issues))
	}
	if !strings.Contains(issues[0].Message, "missing DROP TABLE for table \"users\"") {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
}

func TestScanDirectory_AsymmetricTargetMismatch(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "001_users.up.sql"), []byte("CREATE TABLE users (id int);"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "001_users.down.sql"), []byte("DROP TABLE orders;"), 0644)

	issues := ScanDirectory(tempDir, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for asymmetric target mismatch, got %d", len(issues))
	}
	if !strings.Contains(issues[0].Message, "missing DROP TABLE for table \"users\"") {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
}

func TestScanDirectory_UserScenario_AddColumnVsDropTable(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "001_users.up.sql"), []byte("ALTER TABLE users ADD COLUMN email TEXT;"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "001_users.down.sql"), []byte("DROP TABLE audit_log;"), 0644)

	issues := ScanDirectory(tempDir, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for ADD COLUMN vs DROP unrelated table, got %d", len(issues))
	}
	if !strings.Contains(issues[0].Message, "missing DROP COLUMN for column \"email\" on table \"users\"") {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
}

func TestScanDirectory_BackwardAsymmetry_RogueDrop(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "001_users.up.sql"), []byte("ALTER TABLE users ADD COLUMN email TEXT;"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "001_users.down.sql"), []byte("ALTER TABLE users DROP COLUMN email; DROP TABLE audit_log;"), 0644)

	issues := ScanDirectory(tempDir, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for rogue drop in down migration, got %d: %v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, "unexpected schema mutation on table \"audit_log\"") {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
}

func TestScanDirectory_FullStackRollback_Compliant(t *testing.T) {
	tempDir := t.TempDir()

	upSQL := `
CREATE TABLE users (id int PRIMARY KEY);
ALTER TABLE users ADD COLUMN email text;
CREATE INDEX idx_users_email ON users (email);
`
	downSQL := `
DROP INDEX idx_users_email;
ALTER TABLE users DROP COLUMN email;
DROP TABLE users;
`
	_ = os.WriteFile(filepath.Join(tempDir, "001_users.up.sql"), []byte(upSQL), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "001_users.down.sql"), []byte(downSQL), 0644)

	issues := ScanDirectory(tempDir, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for fully symmetric stack rollback, got %d: %v", len(issues), issues)
	}
}

