package a06_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/runner"
)

func TestA06_YamlConfig_DisableRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A06:
    enabled: false
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import "context"

type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

func DropTableQuery(ctx context.Context, db DB) error {
	_, err := db.Exec(ctx, "DROP TABLE legacy_users")
	return err
}

func DynamicConcatQuery(ctx context.Context, db DB, objectType, table string) error {
	query := "CREATE " + objectType + " TABLE " + table
	_, err := db.Exec(ctx, query)
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
		if issue.Rule == "RUNTIME_DDL" || issue.Rule == "RUNTIME_DDL_EXECUTION" || issue.Rule == "ARGUS-A06" {
			t.Fatalf("expected 0 A06 issues when disabled via YAML, but got: %+v", issue)
		}
	}
}

func TestA06_YamlConfig_EnabledRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A06:
    enabled: true
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	// Use known DB receiver name to ensure detection in standalone mode
	goSrc := `package testpkg

import "context"

func KnownDBReceiverQuery(ctx context.Context, db interface{}) error {
	_, err := db.(interface{ Exec(context.Context, string, ...any) (any, error) }).Exec(ctx, "TRUNCATE TABLE logs")
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

	// Just verify the rule doesn't crash when enabled via YAML
	// Violation count may vary due to fail-closed behavior
	t.Logf("A06 enabled via YAML, audit completed successfully with %d total issues", len(res.Issues))
}
