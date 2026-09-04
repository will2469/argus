package a02_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/runner"
)

func TestA02_YamlConfig_DisableRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A02:
    enabled: false
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import "context"

type Rows interface {
	Close()
}
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
}

func UnclosedQuery(ctx context.Context, db DB) error {
	rows, err := db.Query(ctx, "SELECT id FROM users")
	if err != nil {
		return err
	}
	_ = rows
	return nil
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
		if issue.Rule == "MISSING_DEFER_CLOSE" || issue.Rule == "ARGUS-A02" {
			t.Fatalf("expected 0 A02 issues when disabled via YAML, but got: %+v", issue)
		}
	}
}

func TestA02_YamlConfig_EnabledRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A02:
    enabled: true
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import "context"

type Rows interface {
	Close()
}
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
}

func UnclosedQuery(ctx context.Context, db DB) error {
	rows, err := db.Query(ctx, "SELECT id FROM users")
	if err != nil {
		return err
	}
	_ = rows
	return nil
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

	foundA02 := false
	for _, issue := range res.Issues {
		if issue.Rule == "MISSING_DEFER_CLOSE" || issue.Rule == "ARGUS-A02" {
			foundA02 = true
			break
		}
	}
	if !foundA02 {
		t.Errorf("expected A02 violation to be caught when enabled via YAML, but none was found")
	}
}
