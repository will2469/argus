package a29_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/will2469/argus/rules/a29_unindexed_fk"
	"github.com/will2469/argus/runner"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

func TestA29_PositiveCorpus(t *testing.T) {
	migDir, err := filepath.Abs("positive/migrations")
	if err != nil {
		t.Fatalf("failed to resolve positive migrations dir: %v", err)
	}

	dm := directives.NewDirectiveMap()
	cfg := config.DefaultConfig()

	issues := a29_unindexed_fk.ScanMigrationDir(migDir, dm, cfg)
	t.Logf("=== Positive Migration Corpus Results (%d issues found) ===", len(issues))
	for _, iss := range issues {
		t.Logf("  Line %d [%s]: %s", iss.Line, filepath.Base(iss.Filename), iss.Message)
	}

	if len(issues) != 1 {
		t.Fatalf("Positive Gate FAILED: expected exactly 1 issue from positive migrations, got %d", len(issues))
	}

	base := filepath.Base(issues[0].Filename)
	if base != "001_unsafe_unindexed.up.sql" {
		t.Errorf("expected issue in 001_unsafe_unindexed.up.sql, got %s", base)
	}
	if !strings.Contains(issues[0].Message, "product_id") {
		t.Errorf("expected product_id in issue message, got: %s", issues[0].Message)
	}
}

func TestA29_NegativeCorpus(t *testing.T) {
	migDir, err := filepath.Abs("negative/migrations")
	if err != nil {
		t.Fatalf("failed to resolve negative migrations dir: %v", err)
	}

	dm := directives.NewDirectiveMap()
	cfg := config.DefaultConfig()

	issues := a29_unindexed_fk.ScanMigrationDir(migDir, dm, cfg)
	t.Logf("=== Negative Migration Corpus Results (%d issues found) ===", len(issues))
	for _, iss := range issues {
		t.Errorf("Negative Gate FAILED (False Positive): unexpected issue in %s: %s", filepath.Base(iss.Filename), iss.Message)
	}

	if len(issues) != 0 {
		t.Fatalf("Negative Gate FAILED: expected 0 false positives, got %d", len(issues))
	}
}

func TestA29_AdversarialCorpus(t *testing.T) {
	migDir, err := filepath.Abs("adversarial/migrations")
	if err != nil {
		t.Fatalf("failed to resolve adversarial migrations dir: %v", err)
	}

	dm := directives.NewDirectiveMap()
	cfg := config.DefaultConfig()

	issues := a29_unindexed_fk.ScanMigrationDir(migDir, dm, cfg)
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
		{"M5_AlterTableFK", "005_alter_table_fk.up.sql"},
		{"M6_NonLeadingIndex", "006_non_leading_index.up.sql"},
		{"M7_MultiTableChain", "007_multitable_chain.up.sql"},
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

func TestA29_StandaloneRunner_DualPathParity(t *testing.T) {
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
		if iss.Rule == "ARGUS-A29" || iss.Rule == "UNINDEXED_FOREIGN_KEY" {
			posIssues++
		}
	}
	if posIssues != 1 {
		t.Errorf("Dual-Path Parity FAILED on positive: expected 1 issue from standalone runner, got %d", posIssues)
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
		if iss.Rule == "ARGUS-A29" || iss.Rule == "UNINDEXED_FOREIGN_KEY" {
			negIssues++
		}
	}
	if negIssues != 0 {
		t.Errorf("Dual-Path Parity FAILED on negative: expected 0 issues from standalone runner, got %d", negIssues)
	}
}
