package a07_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/rules/a07_error_leak"
	"github.com/will2469/argus/runner"
	"github.com/will2469/argus/shared/directives"
)

func parseAndInspect(t *testing.T, relPath string) ([]a07_error_leak.Issue, *token.FileSet) {
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
	issues := a07_error_leak.InspectFile(nil, fset, file, dm)
	return issues, fset
}

func TestA07_PositiveCorpus(t *testing.T) {
	issues, fset := parseAndInspect(t, "positive/positive.go")

	t.Logf("=== Positive Corpus Results (%d issues found) ===", len(issues))
	// Updated expectations after fail-closed provenance fix
	// P3 and P4 use custom PgError struct (not real pgconn.PgError), so they won't be flagged
	expectedLines := map[int]string{
		29: "P1_Obvious",
		35: "P2_Indirect",
		51: "P5_Alias_Write",
		52: "P5_Alias_JsonEncode",
		63: "P6_Unmasked_404",
		68: "P7_Unmasked_400",
		73: "P8_FormattedString",
		82: "P9_UnmaskedFactory",
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
	if len(issues) != len(expectedLines) {
		t.Errorf("expected exactly %d issues, got %d", len(expectedLines), len(issues))
	}
}

func TestA07_NegativeCorpus(t *testing.T) {
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

func TestA07_AdversarialCorpus(t *testing.T) {
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
		note       string
	}{
		{"A1_Branch", 23, true, "Direct error leakage"},
		{"A2_Reassignment", 32, true, "Error reassignment"},
		{"A3_Alias", 38, true, "Error alias"},
		{"A4_Wrapper", 47, true, "Wrapper method"},
		{"A5_NestedFunction", 53, true, "Nested function"},
		{"A6_Generic", 64, true, "Generic struct"},
		{"A7_SensitiveField", 69, false, "Custom PgError struct (not pgconn.PgError - fail-closed)"},
		{"A8_CalculatorReceiverNamedDB", 81, false, "Non-DB type with DB receiver name"},
		{"A9_ShadowedInner", 90, false, "Shadowed inner error"},
		{"A9_ShadowedOuter", 92, true, "Outer shadowed error"},
		{"A10_BranchReassignment", 101, true, "Branch reassignment"},
		{"A11_CustomStructWithPgErrorFields", 117, false, "Custom struct Detail field access (not pgconn.PgError - requires type info)"},
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

func TestA07_StandaloneRunner_DualPathParity(t *testing.T) {
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

	a07Issues := 0
	for _, iss := range res.Issues {
		if iss.Rule == "DATABASE_ERROR_LEAK" {
			a07Issues++
		}
	}
	// Updated expectations after fail-closed provenance fix (8 instead of 10)
	if a07Issues != 8 {
		t.Errorf("Dual-Path Parity FAILED on positive corpus: expected 8 issues from standalone runner, got %d", a07Issues)
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
		if iss.Rule == "DATABASE_ERROR_LEAK" {
			negIssues++
		}
	}
	if negIssues != 0 {
		t.Errorf("Dual-Path Parity FAILED on negative corpus: expected 0 issues from standalone runner, got %d", negIssues)
	}
}
