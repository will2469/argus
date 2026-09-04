package a13_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/will2469/argus/rules/a13_missing_down_migration"
	"github.com/will2469/argus/runner"
)

func TestA13_PositiveCorpus(t *testing.T) {
	migDir, err := filepath.Abs("positive/migrations")
	if err != nil {
		t.Fatalf("failed to resolve positive migrations dir: %v", err)
	}

	issues := a13_missing_down_migration.ScanDirectory(migDir, nil)
	t.Logf("=== Positive Migration Corpus Results (%d issues found) ===", len(issues))
	for _, iss := range issues {
		t.Logf("  [%s]: %s", filepath.Base(iss.Filename), iss.Message)
	}

	if len(issues) != 4 {
		t.Fatalf("Positive Gate FAILED: expected exactly 4 issues from positive migrations, got %d", len(issues))
	}

	foundMissing := false
	foundEmpty := false
	foundDummy := false
	foundMismatch := false
	for _, iss := range issues {
		base := filepath.Base(iss.Filename)
		if base == "002_orders.up.sql" && strings.Contains(iss.Message, "Missing required rollback file") {
			foundMissing = true
		}
		if base == "003_empty.down.sql" && strings.Contains(iss.Message, "is empty") {
			foundEmpty = true
		}
		if base == "004_dummy_select.down.sql" && strings.Contains(iss.Message, "missing DROP TABLE for table \"products\"") {
			foundDummy = true
		}
		if base == "005_target_mismatch.down.sql" && strings.Contains(iss.Message, "missing DROP TABLE for table \"logs\"") {
			foundMismatch = true
		}
	}

	if !foundMissing {
		t.Errorf("Positive Gate FAILED: missing down file for 002_orders.up.sql was not flagged")
	}
	if !foundEmpty {
		t.Errorf("Positive Gate FAILED: empty down file for 003_empty.down.sql was not flagged")
	}
	if !foundDummy {
		t.Errorf("Positive Gate FAILED: asymmetric dummy rollback for 004_dummy_select.down.sql was not flagged")
	}
	if !foundMismatch {
		t.Errorf("Positive Gate FAILED: asymmetric target mismatch for 005_target_mismatch.down.sql was not flagged")
	}
}

func TestA13_NegativeCorpus(t *testing.T) {
	migDir, err := filepath.Abs("negative/migrations")
	if err != nil {
		t.Fatalf("failed to resolve negative migrations dir: %v", err)
	}

	issues := a13_missing_down_migration.ScanDirectory(migDir, nil)
	t.Logf("=== Negative Migration Corpus Results (%d issues found) ===", len(issues))
	for _, iss := range issues {
		t.Errorf("Negative Gate FAILED (False Positive): unexpected issue in %s: %s", filepath.Base(iss.Filename), iss.Message)
	}

	if len(issues) != 0 {
		t.Fatalf("Negative Gate FAILED: expected 0 false positives, got %d", len(issues))
	}
}

func TestA13_AdversarialCorpus(t *testing.T) {
	migDir, err := filepath.Abs("adversarial/migrations")
	if err != nil {
		t.Fatalf("failed to resolve adversarial migrations dir: %v", err)
	}

	issues := a13_missing_down_migration.ScanDirectory(migDir, nil)
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
		{"M_WhitespaceOnly", "001_whitespace_down.down.sql"},
		{"M_CommentOnly", "002_comment_only.down.sql"},
		{"M_InvalidSyntax", "003_invalid_syntax.down.sql"},
		{"M_NestedMissingDown", "004_sub.up.sql"},
	}

	for _, a := range assertions {
		if !detectedFiles[a.filename] {
			t.Errorf("Adversarial Evasion Alert: %s in %s was NOT detected", a.vector, a.filename)
		} else {
			t.Logf("  [RESILIENT] %s -> CAUGHT", a.vector)
		}
	}
}

func TestA13_StandaloneRunner_DualPathParity(t *testing.T) {
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
		if iss.Rule == "ARGUS-A13" || iss.Rule == "MISSING_DOWN_MIGRATION" {
			posIssues++
		}
	}
	if posIssues != 4 {
		t.Errorf("Dual-Path Parity FAILED on positive: expected 4 issues from standalone runner, got %d", posIssues)
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
		if iss.Rule == "ARGUS-A13" || iss.Rule == "MISSING_DOWN_MIGRATION" {
			negIssues++
		}
	}
	if negIssues != 0 {
		t.Errorf("Dual-Path Parity FAILED on negative: expected 0 issues from standalone runner, got %d", negIssues)
	}
}
