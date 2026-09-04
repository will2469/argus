package a01_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/runner"
)

func TestA01_YamlConfig_CustomTaintSources(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Write Go file passing parameter "nik" into query execution
	goSrc := `package testpkg

import "context"

type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

func FindByNIK(ctx context.Context, db DB, nik string) {
	q := nik
	_, _ = db.Exec(ctx, q)
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "repo.go"), []byte(goSrc), 0o600); err != nil {
		t.Fatalf("failed to write repo.go: %v", err)
	}

	// 2. Audit with default config (nik is NOT a default universal taint source)
	resDefault, err := runner.RunAuditWithConfig(runner.AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{tempDir},
		MigrationDirs: []string{"/tmp/empty_mig"},
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig (default) failed: %v", err)
	}

	for _, issue := range resDefault.Issues {
		if issue.Rule == "UNSAFE_SQL_CONCATENATION" || issue.Rule == "ARGUS-A01" {
			t.Fatalf("expected 0 A01 issues without custom_taint_sources, but got: %+v", issue)
		}
	}

	// 3. Create .argus.yaml with custom_taint_sources containing "nik"
	yamlContent := `
rules:
  ARGUS-A01:
    enabled: true
    custom_taint_sources:
      - "nik"
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	// 4. Audit again (should reload .argus.yaml and flag the "nik" concatenation)
	resCustom, err := runner.RunAuditWithConfig(runner.AuditConfig{
		RootDir:       tempDir,
		ScanDirs:      []string{tempDir},
		MigrationDirs: []string{"/tmp/empty_mig"},
	})
	if err != nil {
		t.Fatalf("RunAuditWithConfig (custom) failed: %v", err)
	}

	foundA01 := false
	for _, issue := range resCustom.Issues {
		if issue.Rule == "UNSAFE_SQL_CONCATENATION" || issue.Rule == "ARGUS-A01" {
			foundA01 = true
			break
		}
	}
	if !foundA01 {
		t.Errorf("expected A01 violation for custom taint source 'nik', but none was detected")
	}
}

func TestA01_YamlConfig_DisableRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A01:
    enabled: false
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import "context"

type DB interface {
	Query(ctx context.Context, sql string, args ...any)
}

func GetUser(ctx context.Context, db DB, id string) {
	db.Query(ctx, "SELECT * FROM users WHERE id = " + id)
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
		if issue.Rule == "UNSAFE_SQL_CONCATENATION" || issue.Rule == "ARGUS-A01" {
			t.Fatalf("expected 0 A01 issues when disabled via YAML, but got: %+v", issue)
		}
	}
}

func TestA01_YamlConfig_EnabledRule(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
rules:
  ARGUS-A01:
    enabled: true
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	goSrc := `package testpkg

import "context"

type DB interface {
	Query(ctx context.Context, sql string, args ...any)
}

func GetUser(ctx context.Context, db DB, id string) {
	db.Query(ctx, "SELECT * FROM users WHERE id = " + id)
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

	foundA01 := false
	for _, issue := range res.Issues {
		if issue.Rule == "UNSAFE_SQL_CONCATENATION" || issue.Rule == "ARGUS-A01" {
			foundA01 = true
			break
		}
	}
	if !foundA01 {
		t.Errorf("expected A01 violation when enabled via YAML, but none was detected")
	}
}
