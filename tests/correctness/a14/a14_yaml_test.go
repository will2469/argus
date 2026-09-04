package a14_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/runner"
)

func TestA14_YamlConfig_DisableRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A14:
    enabled: false
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import (
	"context"
	"database/sql"
)

type DB struct{ *sql.DB }
func (DB) Query(ctx context.Context, sql string, args ...any) (*sql.Rows, error) { return nil, nil }

func Run(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT * FROM users")
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(goSrc), 0o600); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
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
		if issue.Rule == "FORBIDDEN_SELECT_STAR" || issue.Rule == "ARGUS-A14" {
			t.Fatalf("expected 0 A14 issues when disabled via YAML, but got: %+v", issue)
		}
	}
}

func TestA14_YamlConfig_EnabledRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A14:
    enabled: true
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import (
	"context"
	"database/sql"
)

type DB struct{ *sql.DB }
func (DB) Query(ctx context.Context, sql string, args ...any) (*sql.Rows, error) { return nil, nil }

func Run(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT * FROM users")
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(goSrc), 0o600); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	res, err := runner.RunAuditWithConfig(runner.AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{tempDir},
		MigrationDirs: []string{"/tmp/empty_mig"},
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig failed: %v", err)
	}

	found := false
	for _, issue := range res.Issues {
		if issue.Rule == "FORBIDDEN_SELECT_STAR" || issue.Rule == "ARGUS-A14" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected A14 issue when enabled via YAML, but none detected")
	}
}
