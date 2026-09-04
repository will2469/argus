package a12_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/runner"
)

func TestA12_YamlConfig_DisableRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A12:
    enabled: false
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import "context"

type poolPkg struct{}
var pgxpool poolPkg
func (poolPkg) New(ctx context.Context, dsn string) (any, error) { return nil, nil }

func Init(ctx context.Context) {
	_, _ = pgxpool.New(ctx, "postgres://localhost:5432/db")
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
		if issue.Rule == "TIMEOUT_CONFIG_MISSING" || issue.Rule == "ARGUS-A12" {
			t.Fatalf("expected 0 A12 issues when disabled via YAML, but got: %+v", issue)
		}
	}
}

func TestA12_YamlConfig_EnabledRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A12:
    enabled: true
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import "context"

type pgxpoolPkg struct{}
var pgxpool pgxpoolPkg
func (pgxpoolPkg) New(ctx context.Context, dsn string) (any, error) { return nil, nil }

func Init(ctx context.Context) {
	_, _ = pgxpool.New(ctx, "postgres://localhost:5432/db")
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
		if issue.Rule == "TIMEOUT_CONFIG_MISSING" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected A12 violation when enabled via YAML, but none was reported")
	}
}
