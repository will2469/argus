package a19_test

import (
	"fmt"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/rules/a19_unbounded_limit"
	"github.com/will2469/argus/runner"
	"github.com/will2469/argus/shared/directives"
)

type issueInfo struct {
	Pos     token.Pos
	Message string
}

func parseAndInspect(t *testing.T, relPath string) ([]issueInfo, *token.FileSet) {
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
	tableMap := a19_unbounded_limit.GetHighCardinalityTables(nil)
	keyColumnMap := a19_unbounded_limit.GetKeyColumns(nil)

	var issues []issueInfo
	a19_unbounded_limit.InspectFile(file, fset, dm, tableMap, keyColumnMap, func(pos token.Pos, format string, args ...any) {
		issues = append(issues, issueInfo{
			Pos:     pos,
			Message: fmt.Sprintf(format, args...),
		})
	})
	return issues, fset
}

func TestA19_PositiveCorpus(t *testing.T) {
	issues, fset := parseAndInspect(t, "positive/positive.go")

	t.Logf("=== Positive Corpus Results (%d issues found) ===", len(issues))
	expectedLines := map[int]string{
		19: "P1_Obvious",
		24: "P2_Indirect",
		29: "P3_Helper",
		34: "P4_Nested",
		39: "P5_Alias",
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

func TestA19_NegativeCorpus(t *testing.T) {
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

func TestA19_AdversarialCorpus(t *testing.T) {
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
		{"A1_Branch", 16, true},
		{"A2_Reassignment", 23, true},
		{"A3_Alias", 30, true},
		{"A4_Wrapper", 39, true},
		{"A5_NestedFunction", 45, true},
		{"A6_Generic", 56, true},
		{"A7_SchemaPrefix", 61, true},
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

func TestA19_StandaloneRunner_DualPathParity(t *testing.T) {
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

	a19Issues := 0
	for _, iss := range res.Issues {
		if iss.Rule == "UNBOUNDED_HIGH_CARDINALITY_QUERY" || iss.Rule == "ARGUS-A19" {
			a19Issues++
		}
	}
	if a19Issues == 0 {
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
		if iss.Rule == "UNBOUNDED_HIGH_CARDINALITY_QUERY" || iss.Rule == "ARGUS-A19" {
			negIssues++
		}
	}
	if negIssues != 0 {
		t.Errorf("Dual-Path Parity FAILED on negative corpus: expected 0 issues from standalone runner, got %d", negIssues)
	}
}
