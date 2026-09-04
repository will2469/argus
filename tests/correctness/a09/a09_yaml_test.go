package a09_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/runner"
)

func TestA09_YamlConfig_DisableRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A09:
    enabled: false
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import (
	"context"
)

type DBExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

func LockOperation(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "SELECT pg_advisory_lock(1)")
	return err
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
		if issue.Rule == "UNSAFE_ADVISORY_LOCK" || issue.Rule == "ARGUS-A09" {
			t.Fatalf("expected 0 A09 issues when disabled via YAML, but got: %+v", issue)
		}
	}
}

func TestA09_YamlConfig_EnabledRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A09:
    enabled: true
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import (
	"context"
)

type DBExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

type Helper struct{}

func (Helper) WithAdvisoryLock(ctx context.Context, tx any, lockName string, failFast bool, fn func() error) error {
	return fn()
}

var argus Helper

func LockOperation1(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "SELECT pg_advisory_lock($1)", 123)
	return err
}

func LockOperation2(ctx context.Context, tx any) error {
	return argus.WithAdvisoryLock(ctx, tx, "foo", true, func() error {
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
		if issue.Rule == "UNSAFE_ADVISORY_LOCK" || issue.Rule == "ARGUS-A09" {
			foundCount++
		}
	}
	if foundCount < 2 {
		t.Errorf("expected at least 2 A09 violations when enabled via YAML, got %d", foundCount)
	}
}
