package a12_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/rules/a12_timeout_config"
	"github.com/will2469/argus/runner"
	"github.com/will2469/argus/shared/directives"
)

func parseAndInspect(t *testing.T, relPath string) ([]a12_timeout_config.Issue, *token.FileSet) {
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
	issues := a12_timeout_config.InspectFile(nil, fset, file, dm)
	return issues, fset
}

func TestA12_PositiveCorpus(t *testing.T) {
	issues, fset := parseAndInspect(t, "positive/positive.go")

	t.Logf("=== Positive Corpus Results (%d issues found) ===", len(issues))
	expectedLines := map[int]string{
		47: "P1_Obvious",
		52: "P2_Indirect",
		57: "P3_Helper",
		70: "P4_Nested",
		75: "P5_Alias",
	}

	foundLines := make(map[int]bool)
	for _, iss := range issues {
		pos := fset.Position(iss.Pos)
		foundLines[pos.Line] = true
		pattern := expectedLines[pos.Line]
		t.Logf("  [%s] Line %d: %s", pattern, pos.Line, iss.Message)
	}

	for line, name := range expectedLines {
		if !foundLines[line] {
			t.Errorf("Positive Gate FAILED: expected violation for %s at line %d was NOT detected", name, line)
		}
	}
}

func TestA12_NegativeCorpus(t *testing.T) {
	issues, fset := parseAndInspect(t, "negative/negative.go")

	t.Logf("=== Negative Corpus Results (%d issues found) ===", len(issues))
	for _, iss := range issues {
		pos := fset.Position(iss.Pos)
		t.Errorf("Negative Gate FAILED (False Positive): unexpected issue at Line %d: %s", pos.Line, iss.Message)
	}

	if len(issues) != 0 {
		t.Fatalf("Negative Gate FAILED: expected 0 false positives, got %d", len(issues))
	}
}

func TestA12_AdversarialCorpus(t *testing.T) {
	issues, fset := parseAndInspect(t, "adversarial/adversarial.go")

	t.Logf("=== Adversarial Corpus Stress-Test Results (%d issues found) ===", len(issues))
	detectedLines := make(map[int]bool)
	for _, iss := range issues {
		pos := fset.Position(iss.Pos)
		detectedLines[pos.Line] = true
		t.Logf("  Detected at Line %d: %s", pos.Line, iss.Message)
	}

	assertions := []struct {
		vector     string
		line       int
		mustDetect bool
	}{
		{"A1_Branch", 40, true},
		{"A2_Reassignment", 49, true},
		{"A3_Alias", 56, true},
		{"A4_Wrapper", 63, true},
		{"A5_NestedFunction", 70, true},
		{"A6_Generic", 79, true},
		{"A7_BareStructLiteral", 84, true},
		{"A8_Shadowing", 106, true},
		{"A9_BranchReassignment", 126, true},
		{"A10_BranchZeroTimeout", 145, true},
		{"A11_FakeConfigStruct", 167, true},
		{"A12_BranchFunctionReassignment", 194, true},
	}

	for _, a := range assertions {
		detected := detectedLines[a.line]
		if a.mustDetect && !detected {
			t.Errorf("Adversarial Evasion Alert: %s at line %d was NOT detected", a.vector, a.line)
		} else if !a.mustDetect && detected {
			t.Errorf("Adversarial False Alarm: %s at line %d was falsely flagged", a.vector, a.line)
		} else {
			t.Logf("  [RESILIENT] %s -> CAUGHT", a.vector)
		}
	}
}

func TestA12_StandaloneRunner_DualPathParity(t *testing.T) {
	positiveAbs, _ := filepath.Abs("positive")
	negativeAbs, _ := filepath.Abs("negative")

	auditCfg := runner.AuditConfig{
		RootDir:       positiveAbs,
		ScanDirs:      []string{positiveAbs},
		MigrationDirs: []string{"/tmp/empty_mig"},
	}
	res, err := runner.RunAuditWithConfig(auditCfg)
	if err != nil {
		t.Fatalf("standalone runner audit failed: %v", err)
	}

	a12Issues := 0
	for _, iss := range res.Issues {
		if iss.Rule == "TIMEOUT_CONFIG_MISSING" {
			a12Issues++
		}
	}
	if a12Issues == 0 {
		t.Errorf("Dual-Path Parity FAILED on positive corpus: expected issues from standalone runner, got 0")
	}

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
		if iss.Rule == "TIMEOUT_CONFIG_MISSING" {
			negIssues++
		}
	}
	if negIssues != 0 {
		t.Errorf("Dual-Path Parity FAILED on negative corpus: expected 0 issues from standalone runner, got %d", negIssues)
	}
}
