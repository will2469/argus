package a04_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/runner"
)

func TestA04_YamlConfig_AllowedColumns(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A04:
    enabled: true
    allowed_columns:
      - "id"
      - "created_at"
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import (
	"context"
	"fmt"
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any)
}

func GetSafe(ctx context.Context, db DB) {
	col := "id"
	db.Query(ctx, fmt.Sprintf("SELECT id FROM users ORDER BY %s ASC", col))
}

func GetDisallowed(ctx context.Context, db DB) {
	col := "secret_key"
	db.Query(ctx, fmt.Sprintf("SELECT id FROM users ORDER BY %s ASC", col))
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "user.go"), []byte(goSrc), 0o600); err != nil {
		t.Fatalf("failed to write user.go: %v", err)
	}

	res, err := runner.RunAuditWithConfig(runner.AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{tempDir},
		MigrationDirs: []string{"/tmp/empty_mig"},
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig failed: %v", err)
	}

	var a04Issues []runner.Issue
	for _, issue := range res.Issues {
		if issue.Rule == "UNSAFE_DYNAMIC_ORDERBY" || issue.Rule == "ARGUS-A04" {
			a04Issues = append(a04Issues, issue)
		}
	}

	if len(a04Issues) != 1 {
		t.Fatalf("expected exactly 1 A04 issue for disallowed column, got %d: %+v", len(a04Issues), a04Issues)
	}
}

func TestA04_YamlConfig_DisableRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A04:
    enabled: false
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import (
	"context"
	"fmt"
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any)
}

func GetUnsafe(ctx context.Context, db DB, userSort string) {
	db.Query(ctx, fmt.Sprintf("SELECT id FROM users ORDER BY %s ASC", userSort))
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "user.go"), []byte(goSrc), 0o600); err != nil {
		t.Fatalf("failed to write user.go: %v", err)
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
		if issue.Rule == "UNSAFE_DYNAMIC_ORDERBY" || issue.Rule == "ARGUS-A04" {
			t.Fatalf("expected 0 A04 issues when disabled via YAML, but got: %+v", issue)
		}
	}
}

func TestA04_YamlConfig_EnabledRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A04:
    enabled: true
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import (
	"context"
	"fmt"
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any)
}

func GetUnsafe(ctx context.Context, db DB, userSort string) {
	db.Query(ctx, fmt.Sprintf("SELECT id FROM users ORDER BY %s ASC", userSort))
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "user.go"), []byte(goSrc), 0o600); err != nil {
		t.Fatalf("failed to write user.go: %v", err)
	}

	res, err := runner.RunAuditWithConfig(runner.AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{tempDir},
		MigrationDirs: []string{"/tmp/empty_mig"},
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig failed: %v", err)
	}

	foundA04 := false
	for _, issue := range res.Issues {
		if issue.Rule == "UNSAFE_DYNAMIC_ORDERBY" || issue.Rule == "ARGUS-A04" {
			foundA04 = true
			break
		}
	}
	if !foundA04 {
		t.Errorf("expected A04 violation when enabled via YAML, but none was detected")
	}
}
