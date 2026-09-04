package a15_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/will2469/argus/runner"
)

func TestA15_YamlConfig_DisableRule(t *testing.T) {
	tempDir := t.TempDir()
	migDir := filepath.Join(tempDir, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatalf("failed to create migrations dir: %v", err)
	}

	yamlContent := `
rules:
  ARGUS-A15:
    enabled: false
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	sql := "GRANT ALL PRIVILEGES ON TABLE users TO app_user;"
	if err := os.WriteFile(filepath.Join(migDir, "001_grant.up.sql"), []byte(sql), 0o600); err != nil {
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
		if issue.Rule == "ARGUS-A15" || strings.Contains(issue.Rule, "GRANT") || strings.Contains(issue.Rule, "DDL") {
			t.Fatalf("expected 0 A15 issues when disabled via YAML, but got: %+v", issue)
		}
	}
}

func TestA15_YamlConfig_CustomAppRoles(t *testing.T) {
	tempDir := t.TempDir()
	migDir := filepath.Join(tempDir, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatalf("failed to create migrations dir: %v", err)
	}

	yamlContent := `
rules:
  ARGUS-A15:
    enabled: true
    runtime_app_roles:
      - custom_api_role
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	// Should be flagged because custom_api_role is configured as runtime app role
	sqlUnsafe := "GRANT TRUNCATE ON users TO custom_api_role;"
	if err := os.WriteFile(filepath.Join(migDir, "001_custom_unsafe.up.sql"), []byte(sqlUnsafe), 0o600); err != nil {
		t.Fatalf("failed to write migration: %v", err)
	}

	// Should NOT be flagged because app_user was overridden and not in custom roles
	sqlSafe := "GRANT TRUNCATE ON users TO app_user;"
	if err := os.WriteFile(filepath.Join(migDir, "002_other_role.up.sql"), []byte(sqlSafe), 0o600); err != nil {
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

	var customFound, otherFound bool
	for _, issue := range res.Issues {
		if strings.Contains(issue.Message, "custom_api_role") {
			customFound = true
		}
		if strings.Contains(issue.Message, "app_user") {
			otherFound = true
		}
	}

	if !customFound {
		t.Errorf("expected violation for custom_api_role, but none found")
	}
	if otherFound {
		t.Errorf("did not expect violation for app_user when excluded from custom roles")
	}
}

func TestA15_YamlConfig_ForbidPublicGrantsToggle(t *testing.T) {
	tempDir := t.TempDir()
	migDir := filepath.Join(tempDir, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatalf("failed to create migrations dir: %v", err)
	}

	yamlContent := `
rules:
  ARGUS-A15:
    enabled: true
    forbid_public_grants: false
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write .argus.yaml: %v", err)
	}

	sql := "GRANT CREATE ON SCHEMA public TO PUBLIC;"
	if err := os.WriteFile(filepath.Join(migDir, "001_public_create.up.sql"), []byte(sql), 0o600); err != nil {
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
		if strings.Contains(issue.Message, "PUBLIC") {
			t.Fatalf("expected 0 PUBLIC grant issues when forbid_public_grants: false, but got: %+v", issue)
		}
	}
}
