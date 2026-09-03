package a01_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/rules/a01_sql_concat"
	"github.com/will2469/argus/runner"
	"github.com/will2469/argus/shared/directives"
)

func parseAndInspect(t *testing.T, relPath string) ([]a01_sql_concat.Issue, *token.FileSet) {
	t.Helper()
	absPath, err := filepath.Abs(relPath)
	if err != nil {
		t.Fatalf("failed to get abs path for %s: %v", relPath, err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, absPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", absPath, err)
	}

	dm := directives.ParseGoDirectives(file, fset)
	issues := a01_sql_concat.InspectFile(nil, fset, file, dm)
	return issues, fset
}

func TestA01_PositiveCorpus(t *testing.T) {
	issues, fset := parseAndInspect(t, "positive/positive.go")

	t.Logf("=== Positive Corpus Results (%d issues found) ===", len(issues))
	expectedLines := map[int]string{
		17: "P1_Obvious",
		24: "P2_Indirect",
		34: "P3_Helper",
		41: "P4_Nested",
		50: "P5_Alias",
	}

	foundLines := make(map[int]bool)
	for _, iss := range issues {
		pos := fset.Position(iss.Pos)
		foundLines[pos.Line] = true
		pattern := expectedLines[pos.Line]
		t.Logf("  [%s] Line %d: %s", pattern, pos.Line, iss.Message)
	}

	// 100% Positive Detection Gate
	for line, name := range expectedLines {
		if !foundLines[line] {
			t.Errorf("Positive Gate FAILED: expected violation for %s at line %d was NOT detected", name, line)
		}
	}
	if len(issues) != len(expectedLines) {
		t.Errorf("expected exactly %d issues, got %d", len(expectedLines), len(issues))
	}
}

func TestA01_NegativeCorpus(t *testing.T) {
	issues, fset := parseAndInspect(t, "negative/negative.go")

	t.Logf("=== Negative Corpus Results (%d issues found) ===", len(issues))
	for _, iss := range issues {
		pos := fset.Position(iss.Pos)
		t.Errorf("Negative Gate FAILED (False Positive): unexpected issue at Line %d: %s", pos.Line, iss.Message)
	}

	// 0% False Positive Invariant
	if len(issues) != 0 {
		t.Fatalf("Negative Gate FAILED: expected 0 false positives, got %d", len(issues))
	}
}

func TestA01_AdversarialCorpus(t *testing.T) {
	issues, fset := parseAndInspect(t, "adversarial/adversarial.go")

	t.Logf("=== Adversarial Corpus Stress-Test Results (%d issues found) ===", len(issues))
	detectedLines := make(map[int]bool)
	for _, iss := range issues {
		pos := fset.Position(iss.Pos)
		detectedLines[pos.Line] = true
		t.Logf("  Detected at Line %d: %s", pos.Line, iss.Message)
	}

	// Matrix of expectations:
	// A1 (Line 21): Branch - must be CAUGHT
	// A2 Clean (Line 27): Reassignment to clean - must SURVIVE (NOT caught)
	// A2 Dirty (Line 36): Reassignment to dirty - must be CAUGHT
	// A3 (Line 43): Alias via pointer - must be CAUGHT
	// A4 (Line 56): Wrapper struct - must be CAUGHT
	// A5 (Line 63): Nested closure - must be CAUGHT
	// A6 (Line 74): Generic struct - must be CAUGHT
	// A7 (Line 80): Interface assertion - must be CAUGHT

	assertions := []struct {
		vector     string
		line       int
		mustDetect bool
	}{
		{"A1_Branch", 21, true},
		{"A2_Reassignment_CleanOverride", 27, false},
		{"A2_Reassignment_DirtyOverride", 36, true},
		{"A3_Alias_Pointer", 43, true},
		{"A4_Wrapper_Repo", 56, true},
		{"A5_NestedFunction_Closure", 63, true},
		{"A6_Generic_Repo", 74, true},
		{"A7_Interface_Assertion", 80, true},
	}

	for _, a := range assertions {
		detected := detectedLines[a.line]
		if a.mustDetect && !detected {
			t.Errorf("Adversarial Evasion Alert: %s at line %d was NOT detected", a.vector, a.line)
		} else if !a.mustDetect && detected {
			t.Errorf("Adversarial False Alarm: %s at line %d was falsely flagged", a.vector, a.line)
		} else {
			status := "CAUGHT"
			if !a.mustDetect {
				status = "SURVIVED (CLEAN)"
			}
			t.Logf("  [RESILIENT] %s -> %s", a.vector, status)
		}
	}
}

func TestA01_StandaloneRunner_DualPathParity(t *testing.T) {
	positiveAbs, _ := filepath.Abs("positive")
	negativeAbs, _ := filepath.Abs("negative")

	// 1. Audit positive directory with standalone runner
	auditCfg := runner.AuditConfig{
		RootDir:       positiveAbs,
		ScanDirs:      []string{positiveAbs},
		MigrationDirs: []string{"/tmp/empty_mig"},
	}
	res, err := runner.RunAuditWithConfig(auditCfg)
	if err != nil {
		t.Fatalf("standalone runner audit failed: %v", err)
	}

	a01Issues := 0
	for _, iss := range res.Issues {
		if iss.Rule == "UNSAFE_SQL_CONCATENATION" {
			a01Issues++
		}
	}
	if a01Issues != 5 {
		t.Errorf("Dual-Path Parity FAILED on positive corpus: expected 5 issues from standalone runner, got %d", a01Issues)
	}

	// 2. Audit negative directory with standalone runner
	auditCfgNeg := runner.AuditConfig{
		RootDir:       negativeAbs,
		ScanDirs:      []string{negativeAbs},
		MigrationDirs: []string{"/tmp/empty_mig"},
	}
	resNeg, err := runner.RunAuditWithConfig(auditCfgNeg)
	if err != nil {
		t.Fatalf("standalone runner negative audit failed: %v", err)
	}

	negIssues := 0
	for _, iss := range resNeg.Issues {
		if iss.Rule == "UNSAFE_SQL_CONCATENATION" {
			negIssues++
		}
	}
	if negIssues != 0 {
		t.Errorf("Dual-Path Parity FAILED on negative corpus: expected 0 issues from standalone runner, got %d", negIssues)
	}
}
