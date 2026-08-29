package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuditConfig_CustomDirsAndMigrationGrant(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "argus-runner-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Create custom Go dir
	goDir := filepath.Join(tempDir, "custom-go")
	if err := os.MkdirAll(goDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	goFile := filepath.Join(goDir, "repo.go")
	goSrc := `package repo
import "context"
type DB interface {
	Query(ctx context.Context, sql string, args ...any)
}
func GetUser(ctx context.Context, db DB, id int) {
	db.Query(ctx, "SELECT id, username FROM users WHERE id = $1", id)
}
`
	if err := os.WriteFile(goFile, []byte(goSrc), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// 2. Create custom migration dir with forbidden DDL grant and ignored DDL grant
	migDir := filepath.Join(tempDir, "custom-mig")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	// Violation migration
	migFile1 := filepath.Join(migDir, "001_bad.up.sql")
	migSrc1 := `CREATE TABLE test (id INT);
GRANT ALL PRIVILEGES ON TABLE test TO app_user;
`
	if err := os.WriteFile(migFile1, []byte(migSrc1), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Ignored migration
	migFile2 := filepath.Join(migDir, "002_ignored.up.sql")
	migSrc2 := `CREATE TABLE test2 (id INT);
-- argus:ignore ARGUS-A15 special testing grant
GRANT ALL PRIVILEGES ON TABLE test2 TO app_user;
`
	if err := os.WriteFile(migFile2, []byte(migSrc2), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Run audit with custom config
	cfg := AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{goDir},
		MigrationDirs: []string{migDir},
	}

	result, err := RunAuditWithConfig(cfg)
	if err != nil {
		t.Fatalf("RunAuditWithConfig failed: %v", err)
	}

	if result.ScannedFiles != 3 {
		t.Errorf("Expected 3 scanned files, got %d", result.ScannedFiles)
	}
	if result.VerifiedQuerySites != 1 {
		t.Errorf("Expected 1 verified query site, got %d", result.VerifiedQuerySites)
	}
	if result.VerifiedParameterizedSites != 1 {
		t.Errorf("Expected 1 parameterized placeholder ($1), got %d", result.VerifiedParameterizedSites)
	}

	// Should catch the unignored grant from 001_bad.up.sql
	foundA15 := false
	for _, issue := range result.Issues {
		if issue.Rule == "ARGUS-A15" {
			foundA15 = true
			break
		}
	}
	if !foundA15 {
		t.Errorf("Expected to find ARGUS-A15 violation in bad migration, got issues: %v", result.Issues)
	}
}
