package a03_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/runner"
)

func TestA03_YamlConfig_DisableRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A03:
    enabled: false
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import "context"

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

func UnboundedQuery(db DB) error {
	_, err := db.Query(context.Background(), "SELECT id FROM users")
	return err
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "query.go"), []byte(goSrc), 0o600); err != nil {
		t.Fatalf("failed to write query.go: %v", err)
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
		if issue.Rule == "UNBOUNDED_CONTEXT" || issue.Rule == "ARGUS-A03" {
			t.Fatalf("expected 0 A03 issues when disabled via YAML, but got: %+v", issue)
		}
	}
}

func TestA03_YamlConfig_EnabledRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A03:
    enabled: true
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import "context"

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

func UnboundedQuery(db DB) error {
	_, err := db.Query(context.Background(), "SELECT id FROM users")
	return err
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "query.go"), []byte(goSrc), 0o600); err != nil {
		t.Fatalf("failed to write query.go: %v", err)
	}

	res, err := runner.RunAuditWithConfig(runner.AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{tempDir},
		MigrationDirs: []string{"/tmp/empty_mig"},
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig failed: %v", err)
	}

	foundA03 := false
	for _, issue := range res.Issues {
		if issue.Rule == "UNBOUNDED_CONTEXT" || issue.Rule == "ARGUS-A03" {
			foundA03 = true
			break
		}
	}
	if !foundA03 {
		t.Errorf("expected A03 violation to be caught when enabled via YAML, but none was found")
	}
}
