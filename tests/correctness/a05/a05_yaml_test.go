package a05_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/runner"
)

func TestA05_YamlConfig_CustomAuditTables(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A05:
    enabled: true
    audit_tables:
      - "custom_audit"
      - "secure.trail"
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import (
	"context"
)

type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

func MutateCustom(ctx context.Context, db DB) {
	db.Exec(ctx, "DELETE FROM custom_audit WHERE id = 1")
	db.Exec(ctx, "UPDATE secure.trail SET status = 'void'")
	db.Exec(ctx, "DELETE FROM audit_logs WHERE id = 1") // default table unconfigured, should be ignored
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "audit.go"), []byte(goSrc), 0o600); err != nil {
		t.Fatalf("failed to write audit.go: %v", err)
	}

	res, err := runner.RunAuditWithConfig(runner.AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{tempDir},
		MigrationDirs: []string{"/tmp/empty_mig"},
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig failed: %v", err)
	}

	var a05Issues []runner.Issue
	for _, issue := range res.Issues {
		if issue.Rule == "FORBIDDEN_AUDIT_MUTATION" || issue.Rule == "ARGUS-A05" {
			a05Issues = append(a05Issues, issue)
		}
	}

	if len(a05Issues) != 2 {
		t.Fatalf("expected exactly 2 A05 issues for custom tables, got %d: %+v", len(a05Issues), a05Issues)
	}
}

func TestA05_YamlConfig_DisableRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A05:
    enabled: false
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import (
	"context"
)

type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

func DropAudit(ctx context.Context, db DB) {
	db.Exec(ctx, "DROP TABLE audit_logs")
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "audit.go"), []byte(goSrc), 0o600); err != nil {
		t.Fatalf("failed to write audit.go: %v", err)
	}

	res, err := runner.RunAuditWithConfig(runner.AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{tempDir},
		MigrationDirs: []string{"/tmp/empty_mig"},
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig failed: %v", err)
	}

	for _, issue := range res.Issues {
		if issue.Rule == "FORBIDDEN_AUDIT_MUTATION" || issue.Rule == "ARGUS-A05" {
			t.Fatalf("expected 0 A05 issues when disabled, got issue: %+v", issue)
		}
	}
}

func TestA05_YamlConfig_CheckDownMigrations(t *testing.T) {
	tempDir := t.TempDir()
	migDir := filepath.Join(tempDir, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatalf("failed to create migDir: %v", err)
	}

	downSQL := "-- 001_rollback.down.sql\nDROP TABLE audit_logs;\n"
	if err := os.WriteFile(filepath.Join(migDir, "001_rollback.down.sql"), []byte(downSQL), 0o600); err != nil {
		t.Fatalf("failed to write down migration: %v", err)
	}

	// 1. When check_down_migrations is false (default)
	resDefault, err := runner.RunAuditWithConfig(runner.AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{},
		MigrationDirs: []string{migDir},
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig default failed: %v", err)
	}

	for _, issue := range resDefault.Issues {
		if issue.Rule == "FORBIDDEN_AUDIT_MUTATION" || issue.Rule == "ARGUS-A05" {
			t.Fatalf("expected 0 A05 issues on .down.sql when check_down_migrations is false, got: %+v", issue)
		}
	}

	// 2. When check_down_migrations is true
	yamlContent := `
rules:
  ARGUS-A05:
    enabled: true
    check_down_migrations: true
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	resEnabled, err := runner.RunAuditWithConfig(runner.AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{},
		MigrationDirs: []string{migDir},
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig enabled failed: %v", err)
	}

	var downIssues []runner.Issue
	for _, issue := range resEnabled.Issues {
		if issue.Rule == "FORBIDDEN_AUDIT_MUTATION" || issue.Rule == "ARGUS-A05" {
			downIssues = append(downIssues, issue)
		}
	}

	if len(downIssues) != 1 {
		t.Fatalf("expected 1 A05 issue on .down.sql when check_down_migrations is true, got %d: %+v", len(downIssues), downIssues)
	}
}
