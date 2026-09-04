package a13_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/runner"
)

func TestA13_YamlConfig_DisableRule(t *testing.T) {
	tempDir := t.TempDir()
	migDir := filepath.Join(tempDir, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatalf("failed to create migrations dir: %v", err)
	}

	yamlContent := `
rules:
  ARGUS-A13:
    enabled: false
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	sql := "CREATE TABLE users (id int);"
	if err := os.WriteFile(filepath.Join(migDir, "001_create.up.sql"), []byte(sql), 0o600); err != nil {
		t.Fatalf("failed to write migration: %v", err)
	}

	res, err := runner.RunAuditWithConfig(runner.AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{"/tmp/empty_go_dir"},
		MigrationDirs: []string{migDir},
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig failed: %v", err)
	}

	for _, issue := range res.Issues {
		if issue.Rule == "ARGUS-A13" || issue.Rule == "MISSING_DOWN_MIGRATION" {
			t.Fatalf("expected 0 A13 issues when disabled via YAML, but got: %+v", issue)
		}
	}
}

func TestA13_YamlConfig_EnabledRule(t *testing.T) {
	tempDir := t.TempDir()
	migDir := filepath.Join(tempDir, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatalf("failed to create migrations dir: %v", err)
	}

	yamlContent := `
rules:
  ARGUS-A13:
    enabled: true
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	sqlUp := "CREATE TABLE users (id int);"
	if err := os.WriteFile(filepath.Join(migDir, "001_create.up.sql"), []byte(sqlUp), 0o600); err != nil {
		t.Fatalf("failed to write migration: %v", err)
	}

	res, err := runner.RunAuditWithConfig(runner.AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{"/tmp/empty_go_dir"},
		MigrationDirs: []string{migDir},
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig failed: %v", err)
	}

	found := false
	for _, issue := range res.Issues {
		if issue.Rule == "ARGUS-A13" || issue.Rule == "MISSING_DOWN_MIGRATION" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected A13 violation when enabled via YAML, but none was reported")
	}
}

func TestA13_YamlConfig_AsymmetricRule(t *testing.T) {
	tempDir := t.TempDir()
	migDir := filepath.Join(tempDir, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatalf("failed to create migrations dir: %v", err)
	}

	sqlUp := "CREATE TABLE users (id int);"
	sqlDown := "SELECT 1;"
	if err := os.WriteFile(filepath.Join(migDir, "001_create.up.sql"), []byte(sqlUp), 0o600); err != nil {
		t.Fatalf("failed to write up migration: %v", err)
	}
	if err := os.WriteFile(filepath.Join(migDir, "001_create.down.sql"), []byte(sqlDown), 0o600); err != nil {
		t.Fatalf("failed to write down migration: %v", err)
	}

	res, err := runner.RunAuditWithConfig(runner.AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{"/tmp/empty_go_dir"},
		MigrationDirs: []string{migDir},
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig failed: %v", err)
	}

	found := false
	for _, issue := range res.Issues {
		if (issue.Rule == "ARGUS-A13" || issue.Rule == "MISSING_DOWN_MIGRATION") &&
			issue.Line == 1 {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected A13 asymmetric rollback violation, but none was reported")
	}
}
