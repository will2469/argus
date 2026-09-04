package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/shared/config"
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

func TestAuditConfig_DisabledGoRule(t *testing.T) {
	tempDir := t.TempDir()

	goDir := filepath.Join(tempDir, "service")
	if err := os.MkdirAll(goDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	// Go file violating ARGUS-A24 (multi-tenant query missing tenant_id)
	goFile := filepath.Join(goDir, "user_repo.go")
	goSrc := `package service

import "context"

type DB interface {
	Query(ctx context.Context, sql string, args ...any)
}

func GetUsers(ctx context.Context, db DB) {
	db.Query(ctx, "SELECT id, name FROM tenant_data WHERE active = true")
}
`
	if err := os.WriteFile(goFile, []byte(goSrc), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// 1. Audit with ARGUS-A24 disabled
	cfgDisabled := config.DefaultConfig()
	cfgDisabled.Rules["ARGUS-A24"] = config.RuleConfig{Enabled: false}

	resDisabled, err := RunAuditWithConfig(AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{goDir},
		MigrationDirs: []string{tempDir},
		Config:        cfgDisabled,
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig failed: %v", err)
	}

	for _, issue := range resDisabled.Issues {
		if issue.Rule == "TENANT_ISOLATION_LEAK" || issue.Rule == "ARGUS-A24" {
			t.Fatalf("expected 0 A24 issues when disabled, but found: %+v", issue)
		}
	}

	// 2. Audit with ARGUS-A24 enabled
	cfgEnabled := config.DefaultConfig()
	cfgEnabled.Rules["ARGUS-A24"] = config.RuleConfig{Enabled: true}

	resEnabled, err := RunAuditWithConfig(AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{goDir},
		MigrationDirs: []string{tempDir},
		Config:        cfgEnabled,
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig failed: %v", err)
	}

	foundA24 := false
	for _, issue := range resEnabled.Issues {
		if issue.Rule == "TENANT_ISOLATION_LEAK" || issue.Rule == "ARGUS-A24" {
			foundA24 = true
			break
		}
	}
	if !foundA24 {
		t.Errorf("expected ARGUS-A24 violation when enabled, got issues: %+v", resEnabled.Issues)
	}
}

func TestAuditConfig_DisabledMigrationRule(t *testing.T) {
	tempDir := t.TempDir()

	migDir := filepath.Join(tempDir, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	migFile := filepath.Join(migDir, "001_bad.up.sql")
	migSrc := `CREATE TABLE test (id INT);
GRANT ALL PRIVILEGES ON TABLE test TO app_user;
`
	if err := os.WriteFile(migFile, []byte(migSrc), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// 1. Audit with ARGUS-A15 disabled
	cfgDisabled := config.DefaultConfig()
	cfgDisabled.Rules["ARGUS-A15"] = config.RuleConfig{Enabled: false}

	resDisabled, err := RunAuditWithConfig(AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{tempDir},
		MigrationDirs: []string{migDir},
		Config:        cfgDisabled,
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig failed: %v", err)
	}

	for _, issue := range resDisabled.Issues {
		if issue.Rule == "ARGUS-A15" || issue.Rule == "FORBIDDEN_DDL_APP_ROLE_GRANT" {
			t.Fatalf("expected 0 A15 issues when disabled, but found: %+v", issue)
		}
	}

	// 2. Audit with ARGUS-A15 enabled
	cfgEnabled := config.DefaultConfig()
	cfgEnabled.Rules["ARGUS-A15"] = config.RuleConfig{Enabled: true}

	resEnabled, err := RunAuditWithConfig(AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{tempDir},
		MigrationDirs: []string{migDir},
		Config:        cfgEnabled,
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig failed: %v", err)
	}

	foundA15 := false
	for _, issue := range resEnabled.Issues {
		if issue.Rule == "ARGUS-A15" {
			foundA15 = true
			break
		}
	}
	if !foundA15 {
		t.Errorf("expected ARGUS-A15 violation when enabled, got issues: %+v", resEnabled.Issues)
	}
}

