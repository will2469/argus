package a08_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/runner"
)

func TestA08_YamlConfig_DisableRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A08:
    enabled: false
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import (
	"context"
	"database/sql"
	"net/http"
)

type Pool interface {
	BeginFunc(ctx context.Context, fn func(*sql.Tx) error) error
}

func TxWithIO(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx *sql.Tx) error {
		_, _ = http.Get("https://example.com")
		return nil
	})
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "handler.go"), []byte(goSrc), 0o600); err != nil {
		t.Fatalf("failed to write handler.go: %v", err)
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
		if issue.Rule == "TRANSACTION_BLOCKING_IO" || issue.Rule == "ARGUS-A08" {
			t.Fatalf("expected 0 A08 issues when disabled via YAML, but got: %+v", issue)
		}
	}
}

func TestA08_YamlConfig_EnabledRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A08:
    enabled: true
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import (
	"context"
	"database/sql"
	"net/http"
	"time"
)

type Pool interface {
	BeginFunc(ctx context.Context, fn func(*sql.Tx) error) error
}

func TxWithIO(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx *sql.Tx) error {
		time.Sleep(100 * time.Millisecond)
		_, _ = http.Get("https://example.com")
		return nil
	})
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "handler.go"), []byte(goSrc), 0o600); err != nil {
		t.Fatalf("failed to write handler.go: %v", err)
	}

	res, err := runner.RunAuditWithConfig(runner.AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{tempDir},
		MigrationDirs: []string{"/tmp/empty_mig"},
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig failed: %v", err)
	}

	foundCount := 0
	for _, issue := range res.Issues {
		if issue.Rule == "TRANSACTION_BLOCKING_IO" || issue.Rule == "ARGUS-A08" {
			foundCount++
		}
	}
	if foundCount < 2 {
		t.Errorf("expected at least 2 A08 violations when enabled via YAML, got %d", foundCount)
	}
}
