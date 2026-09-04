package a11_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/runner"
)

func TestA11_YamlConfig_DisableRule(t *testing.T) {
	tempDir := t.TempDir()
	migDir := filepath.Join(tempDir, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatalf("failed to create migrations dir: %v", err)
	}

	yamlContent := `
rules:
  ARGUS-A11:
    enabled: false
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	sql := "DROP TABLE users;"
	if err := os.WriteFile(filepath.Join(migDir, "001_drop.up.sql"), []byte(sql), 0o600); err != nil {
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
		if issue.Rule == "ARGUS-A11" || issue.Rule == "DESTRUCTIVE_MIGRATION" {
			t.Fatalf("expected 0 A11 issues when disabled via YAML, but got: %+v", issue)
		}
	}
}

func TestA11_YamlConfig_EnabledRule(t *testing.T) {
	tempDir := t.TempDir()
	migDir := filepath.Join(tempDir, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatalf("failed to create migrations dir: %v", err)
	}

	yamlContent := `
rules:
  ARGUS-A11:
    enabled: true
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	sql := "DROP TABLE users;"
	if err := os.WriteFile(filepath.Join(migDir, "001_drop.up.sql"), []byte(sql), 0o600); err != nil {
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

	foundCount := 0
	for _, issue := range res.Issues {
		if issue.Rule == "ARGUS-A11" || issue.Rule == "DESTRUCTIVE_MIGRATION" {
			foundCount++
		}
	}
	if foundCount != 1 {
		t.Errorf("expected 1 A11 violation when enabled via YAML, got %d", foundCount)
	}
}
