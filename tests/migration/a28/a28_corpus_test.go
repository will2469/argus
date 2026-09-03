package a28_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/will2469/argus/rules/a28_constraint_lock"
	"github.com/will2469/argus/runner"
)

func TestA28_PositiveCorpus(t *testing.T) {
	migDir, err := filepath.Abs("positive/migrations")
	if err != nil {
		t.Fatalf("failed to resolve positive migrations dir: %v", err)
	}

	issues, err := a28_constraint_lock.ScanMigrationDir(migDir)
	if err != nil {
		t.Fatalf("ScanMigrationDir failed: %v", err)
	}

	t.Logf("=== Positive Migration Corpus Results (%d issues found) ===", len(issues))
	for _, iss := range issues {
		t.Logf("  Line %d [%s]: %s", iss.Line, filepath.Base(iss.Filename), iss.Message)
	}

	if len(issues) != 2 {
		t.Fatalf("Positive Gate FAILED: expected exactly 2 issues from positive migrations, got %d", len(issues))
	}

	for _, iss := range issues {
		if !strings.Contains(iss.Message, "NOT VALID") {
			t.Errorf("expected NOT VALID in issue message, got: %s", iss.Message)
		}
	}
}

func TestA28_NegativeCorpus(t *testing.T) {
	migDir, err := filepath.Abs("negative/migrations")
	if err != nil {
		t.Fatalf("failed to resolve negative migrations dir: %v", err)
	}

	issues, err := a28_constraint_lock.ScanMigrationDir(migDir)
	if err != nil {
		t.Fatalf("ScanMigrationDir failed: %v", err)
	}

	t.Logf("=== Negative Migration Corpus Results (%d issues found) ===", len(issues))
	for _, iss := range issues {
		t.Errorf("Negative Gate FAILED (False Positive): unexpected issue in %s: %s", filepath.Base(iss.Filename), iss.Message)
	}

	if len(issues) != 0 {
		t.Fatalf("Negative Gate FAILED: expected 0 false positives, got %d", len(issues))
	}
}

func TestA28_AdversarialCorpus(t *testing.T) {
	migDir, err := filepath.Abs("adversarial/migrations")
	if err != nil {
		t.Fatalf("failed to resolve adversarial migrations dir: %v", err)
	}

	issues, err := a28_constraint_lock.ScanMigrationDir(migDir)
	if err != nil {
		t.Fatalf("ScanMigrationDir failed: %v", err)
	}

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
		{"M1_MultistmtChain", "001_multistmt_chain.up.sql"},
		{"M2_CaseInsensitive", "002_case_insensitivity.up.sql"},
		{"M3_QuotedIdent", "003_quoted_ident.up.sql"},
		{"M4_SchemaQualified", "004_schema_qualified.up.sql"},
		{"M5_CompositeFK", "005_composite_fk.up.sql"},
		{"M6_BoolCheck", "006_bool_check.up.sql"},
		{"M7_MultiConstraint", "007_multiconstraint.up.sql"},
	}

	for _, a := range assertions {
		if !detectedFiles[a.filename] {
			t.Errorf("Adversarial Evasion Alert: %s in %s was NOT detected", a.vector, a.filename)
		} else {
			t.Logf("  [RESILIENT] %s -> CAUGHT", a.vector)
		}
	}
	if len(issues) < len(assertions) {
		t.Errorf("expected at least %d issues, got %d", len(assertions), len(issues))
	}
}

func TestA28_StandaloneRunner_DualPathParity(t *testing.T) {
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
		if iss.Rule == "ARGUS-A28" || iss.Rule == "BLOCKING_CONSTRAINT_ADD" {
			posIssues++
		}
	}
	if posIssues != 2 {
		t.Errorf("Dual-Path Parity FAILED on positive: expected 2 issues from standalone runner, got %d", posIssues)
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
		if iss.Rule == "ARGUS-A28" || iss.Rule == "BLOCKING_CONSTRAINT_ADD" {
			negIssues++
		}
	}
	if negIssues != 0 {
		t.Errorf("Dual-Path Parity FAILED on negative: expected 0 issues from standalone runner, got %d", negIssues)
	}
}
