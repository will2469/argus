package a11_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/rules/a11_destructive_migration"
	"github.com/will2469/argus/runner"
)

func TestA11_PositiveCorpus(t *testing.T) {
	migDir, err := filepath.Abs("positive/migrations")
	if err != nil {
		t.Fatalf("failed to resolve positive migrations dir: %v", err)
	}

	issues, err := a11_destructive_migration.ScanMigrationDir(migDir)
	if err != nil {
		t.Fatalf("ScanMigrationDir failed: %v", err)
	}

	t.Logf("=== Positive Migration Corpus Results (%d issues found) ===", len(issues))
	for _, iss := range issues {
		t.Logf("  Line %d [%s]: %s", iss.Line, filepath.Base(iss.Filename), iss.Message)
	}

	if len(issues) != 6 {
		t.Fatalf("Positive Gate FAILED: expected exactly 6 issues from positive migrations, got %d", len(issues))
	}

	detectedFiles := make(map[string]bool)
	for _, iss := range issues {
		detectedFiles[filepath.Base(iss.Filename)] = true
	}

	expectedFiles := []string{
		"001_destructive.up.sql",
		"002_dummy_bypass.up.sql",
		"003_drop_schema.up.sql",
		"004_drop_constraint.up.sql",
		"005_set_not_null.up.sql",
		"006_detach_partition.up.sql",
	}

	for _, ef := range expectedFiles {
		if !detectedFiles[ef] {
			t.Errorf("expected violation in %s, but none was detected", ef)
		}
	}
}

func TestA11_NegativeCorpus(t *testing.T) {
	migDir, err := filepath.Abs("negative/migrations")
	if err != nil {
		t.Fatalf("failed to resolve negative migrations dir: %v", err)
	}

	issues, err := a11_destructive_migration.ScanMigrationDir(migDir)
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

func TestA11_AdversarialCorpus(t *testing.T) {
	migDir, err := filepath.Abs("adversarial/migrations")
	if err != nil {
		t.Fatalf("failed to resolve adversarial migrations dir: %v", err)
	}

	issues, err := a11_destructive_migration.ScanMigrationDir(migDir)
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
		{"M5_Truncate", "005_truncate.up.sql"},
		{"M6_AlterType", "006_alter_type.up.sql"},
		{"M7_NotNullNoDefault", "007_not_null_no_default.up.sql"},
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

func TestA11_StandaloneRunner_DualPathParity(t *testing.T) {
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
		if iss.Rule == "ARGUS-A11" || iss.Rule == "DESTRUCTIVE_MIGRATION" {
			posIssues++
		}
	}
	if posIssues != 6 {
		t.Errorf("Dual-Path Parity FAILED on positive: expected 6 issues from standalone runner, got %d", posIssues)
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
		if iss.Rule == "ARGUS-A11" || iss.Rule == "DESTRUCTIVE_MIGRATION" {
			negIssues++
		}
	}
	if negIssues != 0 {
		t.Errorf("Dual-Path Parity FAILED on negative: expected 0 issues from standalone runner, got %d", negIssues)
	}
}

func TestA11_EndToEndExpandContractMigrationLifecycle(t *testing.T) {
	// 1. Authorized Lifecycle (Expand -> Migrate -> Authorized Contract): Must pass with 0 issues
	t.Run("AuthorizedLifecycle_MustPass", func(t *testing.T) {
		dir := t.TempDir()
		m1 := `ALTER TABLE users ADD COLUMN email_v2 VARCHAR(255) DEFAULT '';`
		m2 := `UPDATE users SET email_v2 = email WHERE email_v2 = '';`
		m3 := `-- argus:phase=contract release=v2.1.0 issue=DB-402 approved_by=lead-dba reason="completed dual-write migration of user email"
ALTER TABLE users DROP COLUMN email;`

		if err := os.WriteFile(filepath.Join(dir, "001_expand.up.sql"), []byte(m1), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "002_migrate.up.sql"), []byte(m2), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "003_contract.up.sql"), []byte(m3), 0644); err != nil {
			t.Fatal(err)
		}

		// Direct migration scanner
		issues, err := a11_destructive_migration.ScanMigrationDir(dir)
		if err != nil {
			t.Fatalf("ScanMigrationDir failed: %v", err)
		}
		if len(issues) != 0 {
			t.Fatalf("expected 0 issues for authorized lifecycle, got %d: %+v", len(issues), issues)
		}

		// End-to-end runner audit
		auditCfg := runner.AuditConfig{
			RootDir:       dir,
			ScanDirs:      []string{t.TempDir()},
			MigrationDirs: []string{dir},
		}
		res, err := runner.RunAuditWithConfig(auditCfg)
		if err != nil {
			t.Fatalf("RunAuditWithConfig failed: %v", err)
		}
		a11Issues := 0
		for _, iss := range res.Issues {
			if iss.Rule == "ARGUS-A11" || iss.Rule == "DESTRUCTIVE_MIGRATION" {
				a11Issues++
			}
		}
		if a11Issues != 0 {
			t.Fatalf("expected 0 runner issues for authorized lifecycle, got %d", a11Issues)
		}
	})

	// 2. Unauthorized Contract Lifecycle (Dummy Release Bypass): Must be blocked with 1 issue
	t.Run("UnauthorizedDummyBypass_MustBeBlocked", func(t *testing.T) {
		dir := t.TempDir()
		m1 := `ALTER TABLE users ADD COLUMN email_v2 VARCHAR(255) DEFAULT '';`
		m2 := `-- argus:contract anything
ALTER TABLE users DROP COLUMN email;`

		if err := os.WriteFile(filepath.Join(dir, "001_expand.up.sql"), []byte(m1), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "002_contract.up.sql"), []byte(m2), 0644); err != nil {
			t.Fatal(err)
		}

		issues, err := a11_destructive_migration.ScanMigrationDir(dir)
		if err != nil {
			t.Fatalf("ScanMigrationDir failed: %v", err)
		}
		if len(issues) != 1 {
			t.Fatalf("expected 1 issue blocking unauthorized contract, got %d", len(issues))
		}
	})

	// 3. Expand Phase With Destructive DDL: Must be blocked because destructive operations are forbidden in expand phase
	t.Run("ExpandPhaseDestructive_MustBeBlocked", func(t *testing.T) {
		dir := t.TempDir()
		m1 := `-- argus:phase=expand release=v2.0.0 issue=DB-101 approved_by=lead-dba reason="expand phase"
ALTER TABLE users DROP COLUMN email;`

		if err := os.WriteFile(filepath.Join(dir, "001_expand_destructive.up.sql"), []byte(m1), 0644); err != nil {
			t.Fatal(err)
		}

		issues, err := a11_destructive_migration.ScanMigrationDir(dir)
		if err != nil {
			t.Fatalf("ScanMigrationDir failed: %v", err)
		}
		if len(issues) != 1 {
			t.Fatalf("expected 1 issue blocking destructive operation in expand phase, got %d", len(issues))
		}
	})
}

