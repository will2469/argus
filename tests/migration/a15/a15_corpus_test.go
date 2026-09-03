package a15_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/will2469/argus/rules/a15_ddl_grant"
	"github.com/will2469/argus/runner"
)

func TestA15_PositiveCorpus(t *testing.T) {
	migDir, err := filepath.Abs("positive/migrations")
	if err != nil {
		t.Fatalf("failed to resolve positive migrations dir: %v", err)
	}

	reg := a15_ddl_grant.NewRoleRegistry([]string{"app_user", "public"})
	issues := a15_ddl_grant.ScanDirectory(migDir, reg)

	t.Logf("=== Positive Migration Corpus Results (%d issues found) ===", len(issues))
	for _, iss := range issues {
		t.Logf("  Line %d [%s]: %s", iss.Line, filepath.Base(iss.Filename), iss.Message)
	}

	if len(issues) != 3 {
		t.Fatalf("Positive Gate FAILED: expected exactly 3 issues from positive migrations, got %d", len(issues))
	}

	detectedFiles := make(map[string]bool)
	for _, iss := range issues {
		detectedFiles[filepath.Base(iss.Filename)] = true
	}

	expectedFiles := []string{
		"001_unsafe_all.up.sql",
		"002_unsafe_create.up.sql",
		"003_unsafe_owner.up.sql",
	}
	for _, f := range expectedFiles {
		if !detectedFiles[f] {
			t.Errorf("Positive Gate FAILED: expected violation in %s was not flagged", f)
		}
	}
}

func TestA15_NegativeCorpus(t *testing.T) {
	migDir, err := filepath.Abs("negative/migrations")
	if err != nil {
		t.Fatalf("failed to resolve negative migrations dir: %v", err)
	}

	reg := a15_ddl_grant.NewRoleRegistry([]string{"app_user", "public"})
	issues := a15_ddl_grant.ScanDirectory(migDir, reg)

	t.Logf("=== Negative Migration Corpus Results (%d issues found) ===", len(issues))
	for _, iss := range issues {
		t.Errorf("Negative Gate FAILED (False Positive): unexpected issue in %s: %s", filepath.Base(iss.Filename), iss.Message)
	}

	if len(issues) != 0 {
		t.Fatalf("Negative Gate FAILED: expected 0 false positives, got %d", len(issues))
	}
}

func TestA15_AdversarialCorpus(t *testing.T) {
	migDir, err := filepath.Abs("adversarial/migrations")
	if err != nil {
		t.Fatalf("failed to resolve adversarial migrations dir: %v", err)
	}

	reg := a15_ddl_grant.NewRoleRegistry([]string{"app_user", "web_app", "public"})
	issues := a15_ddl_grant.ScanDirectory(migDir, reg)

	t.Logf("=== Adversarial Migration Stress-Test Results (%d issues found) ===", len(issues))
	detectedFiles := make(map[string]bool)
	for _, iss := range issues {
		base := filepath.Base(iss.Filename)
		detectedFiles[base] = true
		t.Logf("  Detected in %s: %s", base, iss.Message)
	}

	assertions := []struct {
		vector   string
		filename string
	}{
		{"M_CaseInsensitive", "001_case_insensitivity.up.sql"},
		{"M_QuotedIdent", "002_quoted_identifiers.up.sql"},
		{"M_MultistmtChain", "003_multistmt_chain.up.sql"},
		{"M_SchemaPrefix", "004_schema_prefix.up.sql"},
		{"M_WebAppRole", "005_web_app_role.up.sql"},
	}

	for _, a := range assertions {
		if !detectedFiles[a.filename] {
			t.Errorf("Adversarial Evasion Alert: %s in %s was NOT detected", a.vector, a.filename)
		} else {
			t.Logf("  [RESILIENT] %s -> CAUGHT", a.vector)
		}
	}
}

func TestA15_StandaloneRunner_DualPathParity(t *testing.T) {
	posMigDir, _ := filepath.Abs("positive/migrations")
	negMigDir, _ := filepath.Abs("negative/migrations")

	auditCfgPos := runner.AuditConfig{
		RootDir:       posMigDir,
		ScanDirs:      []string{"/tmp/empty_go_dir"},
		MigrationDirs: []string{posMigDir},
	}
	resPos, err := runner.RunAuditWithConfig(auditCfgPos)
	if err != nil {
		t.Fatalf("standalone runner audit failed: %v", err)
	}

	posIssues := 0
	for _, iss := range resPos.Issues {
		if iss.Rule == "ARGUS-A15" || strings.Contains(iss.Rule, "GRANT") || strings.Contains(iss.Rule, "DDL") {
			posIssues++
		}
	}
	if posIssues != 3 {
		t.Errorf("Dual-Path Parity FAILED on positive: expected 3 issues from standalone runner, got %d", posIssues)
	}

	auditCfgNeg := runner.AuditConfig{
		RootDir:       negMigDir,
		ScanDirs:      []string{"/tmp/empty_go_dir"},
		MigrationDirs: []string{negMigDir},
	}
	resNeg, err := runner.RunAuditWithConfig(auditCfgNeg)
	if err != nil {
		t.Fatalf("standalone runner audit failed: %v", err)
	}

	negIssues := 0
	for _, iss := range resNeg.Issues {
		if iss.Rule == "ARGUS-A15" || strings.Contains(iss.Rule, "GRANT") || strings.Contains(iss.Rule, "DDL") {
			negIssues++
		}
	}
	if negIssues != 0 {
		t.Errorf("Dual-Path Parity FAILED on negative: expected 0 issues from standalone runner, got %d", negIssues)
	}
}
