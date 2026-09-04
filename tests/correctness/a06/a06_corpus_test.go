package a06_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/rules/a06_runtime_ddl"
	"github.com/will2469/argus/runner"
	"github.com/will2469/argus/shared/directives"
)

func parseAndInspect(t *testing.T, relPath string) ([]a06_runtime_ddl.Issue, *token.FileSet) {
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
	issues := a06_runtime_ddl.InspectFile(nil, fset, file, dm)
	return issues, fset
}

func TestA06_PositiveCorpus(t *testing.T) {
	issues, fset := parseAndInspect(t, "positive/positive.go")

	t.Logf("=== Positive Corpus Results (%d issues found) ===", len(issues))
	expectedLines := map[int]string{
		18: "P1_Obvious",
		25: "P2_Indirect",
		35: "P3_Helper",
		41: "P4_Nested",
		48: "P5_Alias",
		55: "P6_DynamicConcat",
		61: "P7_InlineConcat",
		70: "P8_StringBuilder",
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

func TestA06_NegativeCorpus(t *testing.T) {
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

func TestA06_AdversarialCorpus(t *testing.T) {
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
		{"A1_Branch", 17, true, "Direct DB call"},
		{"A2_Reassignment", 28, true, "Direct DB call"},
		{"A3_Alias", 36, true, "Direct DB call"},
		{"A4_Wrapper", 46, true, "Repository struct wrapping DB"},
		{"A5_NestedFunction", 53, true, "Closure wrapping DB"},
		{"A6_Generic", 65, true, "Generic struct wrapping DB"},
		{"A7_Interface", 72, true, "Type asserted interface"},
		{"A8_ShadowedInner", 83, true, "Direct DB call"},
		{"A8_ShadowedOuter", 87, false, "Safe SELECT query"},
		{"A9_BranchReassignment", 97, true, "Direct DB call"},
		{"A10_NonDBTypeSpoofing", 108, false, "Non-DB interface with Query & Exec methods"},
		{"A11_CustomBuilderTypeSpoofing", 122, false, "Non-builder type with safe string"},
		{"A12_UnconventionalReceiverName", 128, true, "Receiver named client with proven DBExecutor type"},
		{"A13_FakeDBReceiverName", 134, false, "Receiver named db but with non-DB SearchEngine type"},
	}

	for _, a := range assertions {
		detected := detectedLines[a.line]
		if a.mustDetect && !detected {
			t.Errorf("Adversarial Evasion Alert: %s at line %d was NOT detected (%s)", a.vector, a.line, a.note)
		} else if !a.mustDetect && detected {
			t.Errorf("Adversarial False Alarm: %s at line %d was falsely flagged (%s)", a.vector, a.line, a.note)
		} else {
			t.Logf("  [RESILIENT] %s -> %s", a.vector, map[bool]string{true: "CAUGHT", false: "CORRECTLY SKIPPED"}[a.mustDetect])
		}
	}
}

func TestA06_StandaloneRunner_DualPathParity(t *testing.T) {
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

	a06Issues := 0
	for _, iss := range res.Issues {
		if iss.Rule == "RUNTIME_DDL_EXECUTION" {
			a06Issues++
		}
	}
	if a06Issues != 8 {
		t.Errorf("Dual-Path Parity FAILED on positive corpus: expected 8 issues from standalone runner, got %d", a06Issues)
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
		if iss.Rule == "RUNTIME_DDL_EXECUTION" {
			negIssues++
		}
	}
	if negIssues != 0 {
		t.Errorf("Dual-Path Parity FAILED on negative corpus: expected 0 issues from standalone runner, got %d", negIssues)
	}
}
