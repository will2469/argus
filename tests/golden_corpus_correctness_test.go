package tests_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/will2469/argus/runner"
)

type RuleCorrectnessMetric struct {
	RuleCode        string
	Category        string
	PositiveCount   int
	NegativeCount   int
	AdversarialCount int
	SoundnessPassed bool
}

var ruleTag = map[string]string{
	"ARGUS-A01": "UNSAFE_SQL_CONCATENATION",
	"ARGUS-A02": "MISSING_DEFER_CLOSE",
	"ARGUS-A03": "UNBOUNDED_CONTEXT",
	"ARGUS-A04": "UNSAFE_DYNAMIC_ORDERBY",
	"ARGUS-A05": "FORBIDDEN_AUDIT_MUTATION",
	"ARGUS-A06": "RUNTIME_DDL_EXECUTION",
	"ARGUS-A07": "DATABASE_ERROR_LEAK",
	"ARGUS-A08": "TRANSACTION_BLOCKING_IO",
	"ARGUS-A09": "UNSAFE_ADVISORY_LOCK",
	"ARGUS-A10": "WEAK_ISOLATION_LEVEL",
	"ARGUS-A11": "ARGUS-A11",
	"ARGUS-A12": "TIMEOUT_CONFIG_MISSING",
	"ARGUS-A13": "ARGUS-A13",
	"ARGUS-A14": "FORBIDDEN_SELECT_STAR",
	"ARGUS-A15": "ARGUS-A15",
	"ARGUS-A16": "UNBOUNDED_MAX_CONNS",
	"ARGUS-A17": "FORBIDDEN_QUERY_IN_LOOP",
	"ARGUS-A18": "UNCHECKED_ROWS_ERROR",
	"ARGUS-A19": "UNBOUNDED_HIGH_CARDINALITY_QUERY",
	"ARGUS-A20": "WIRE_PARAM_LIMIT",
	"ARGUS-A21": "LOCK_CONVOY",
	"ARGUS-A22": "RETRY_TRANSACTION",
	"ARGUS-A23": "TX_TIMEOUT_GUC",
	"ARGUS-A24": "TENANT_ISOLATION_LEAK",
	"ARGUS-A25": "EXPENSIVE_CPU_IN_TRANSACTION",
	"ARGUS-A26": "LIKE_WILDCARD_INJECTION",
	"ARGUS-A27": "ARGUS-A27",
	"ARGUS-A28": "ARGUS-A28",
	"ARGUS-A29": "ARGUS-A29",
	"ARGUS-A30": "ARGUS-A30",
}

func filterIssuesForRule(issues []runner.Issue, code string) []runner.Issue {
	tag := ruleTag[code]
	var res []runner.Issue
	for _, iss := range issues {
		if iss.Rule == code || iss.Rule == tag || strings.Contains(iss.Message, code) || strings.Contains(iss.Rule, code) {
			res = append(res, iss)
		}
	}
	return res
}

// TestGoldenCorpus_CorrectnessGate programmatically orchestrates runtime execution of all 30 rules,
// validating the Semantic Correctness Gate (Gate 2).
func TestGoldenCorpus_CorrectnessGate(t *testing.T) {
	rootDir, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("failed to resolve root dir: %v", err)
	}

	totalRules := 30
	var metrics []RuleCorrectnessMetric
	totalPos := 0
	totalNegClean := 0
	totalAdv := 0

	for i := 1; i <= totalRules; i++ {
		id := fmt.Sprintf("A%02d", i)
		code := fmt.Sprintf("ARGUS-%s", id)
		folder := strings.ToLower(id)

		isMigration := false
		switch id {
		case "A11", "A13", "A15", "A27", "A28", "A29", "A30":
			isMigration = true
		}

		metric := RuleCorrectnessMetric{
			RuleCode: code,
		}

		dummyGoFile := filepath.Join(rootDir, "tests", "golden", "pgxpool", "pool.go")

		if isMigration {
			metric.Category = "Migration"
			baseDir := filepath.Join(rootDir, "tests", "migration", folder)

			// 1. Positive Corpus Execution
			posDir := filepath.Join(baseDir, "positive", "migrations")
			posRes, err := runner.RunAuditWithConfig(runner.AuditConfig{
				RootDir:       rootDir,
				ScanDirs:      []string{dummyGoFile},
				MigrationDirs: []string{posDir},
			})
			if err != nil {
				t.Fatalf("%s positive scan failed: %v", code, err)
			}
			posIssues := filterIssuesForRule(posRes.Issues, code)
			metric.PositiveCount = len(posIssues)
			totalPos += metric.PositiveCount

			// 2. Negative Corpus Execution (Zero False-Positive Invariant)
			negDir := filepath.Join(baseDir, "negative", "migrations")
			negRes, err := runner.RunAuditWithConfig(runner.AuditConfig{
				RootDir:       rootDir,
				ScanDirs:      []string{dummyGoFile},
				MigrationDirs: []string{negDir},
			})
			if err != nil {
				t.Fatalf("%s negative scan failed: %v", code, err)
			}
			negIssues := filterIssuesForRule(negRes.Issues, code)
			metric.NegativeCount = len(negIssues)
			if metric.NegativeCount == 0 {
				totalNegClean++
			}

			// 3. Adversarial Corpus Execution
			advDir := filepath.Join(baseDir, "adversarial", "migrations")
			advRes, err := runner.RunAuditWithConfig(runner.AuditConfig{
				RootDir:       rootDir,
				ScanDirs:      []string{dummyGoFile},
				MigrationDirs: []string{advDir},
			})
			if err != nil {
				t.Fatalf("%s adversarial scan failed: %v", code, err)
			}
			advIssues := filterIssuesForRule(advRes.Issues, code)
			metric.AdversarialCount = len(advIssues)
			totalAdv += metric.AdversarialCount

		} else {
			metric.Category = "Correctness"
			baseDir := filepath.Join(rootDir, "tests", "correctness", folder)

			// 1. Positive Corpus Execution
			posDir := filepath.Join(baseDir, "positive")
			posRes, err := runner.RunAuditWithConfig(runner.AuditConfig{
				RootDir:  rootDir,
				ScanDirs: []string{posDir},
			})
			if err != nil {
				t.Fatalf("%s positive scan failed: %v", code, err)
			}
			posIssues := filterIssuesForRule(posRes.Issues, code)
			metric.PositiveCount = len(posIssues)
			totalPos += metric.PositiveCount

			// 2. Negative Corpus Execution (Zero False-Positive Invariant)
			negDir := filepath.Join(baseDir, "negative")
			negRes, err := runner.RunAuditWithConfig(runner.AuditConfig{
				RootDir:  rootDir,
				ScanDirs: []string{negDir},
			})
			if err != nil {
				t.Fatalf("%s negative scan failed: %v", code, err)
			}
			negIssues := filterIssuesForRule(negRes.Issues, code)
			metric.NegativeCount = len(negIssues)
			if metric.NegativeCount == 0 {
				totalNegClean++
			}

			// 3. Adversarial Corpus Execution
			advDir := filepath.Join(baseDir, "adversarial")
			advRes, err := runner.RunAuditWithConfig(runner.AuditConfig{
				RootDir:  rootDir,
				ScanDirs: []string{advDir},
			})
			if err != nil {
				t.Fatalf("%s adversarial scan failed: %v", code, err)
			}
			advIssues := filterIssuesForRule(advRes.Issues, code)
			metric.AdversarialCount = len(advIssues)
			totalAdv += metric.AdversarialCount
		}

		// Semantic soundness rule check:
		// - Must detect positive violations (>= 1)
		// - Must NOT produce any false positives on negative corpus (== 0)
		// - Must catch adversarial vectors (>= 1)
		if metric.PositiveCount > 0 && metric.NegativeCount == 0 && metric.AdversarialCount > 0 {
			metric.SoundnessPassed = true
		} else {
			metric.SoundnessPassed = false
			t.Errorf("RULE %s SOUNDNESS FAILURE: pos=%d (want >0), neg=%d (want 0), adv=%d (want >0)",
				code, metric.PositiveCount, metric.NegativeCount, metric.AdversarialCount)
		}

		metrics = append(metrics, metric)
	}

	t.Log("=========================================================================================================")
	t.Log("ARGUS 1-SSOT GOLDEN CORPUS — GATE 2: SEMANTIC CORRECTNESS & DUAL-PATH PARITY MATRIX")
	t.Log("=========================================================================================================")
	t.Logf("%-12s | %-12s | %-10s | %-10s | %-12s | %s",
		"RULE CODE", "CATEGORY", "POS DETECT", "NEG NOISE", "ADV CAUGHT", "SEMANTIC SOUNDNESS")
	t.Log("---------------------------------------------------------------------------------------------------------")

	passedRules := 0
	for _, m := range metrics {
		status := "FAIL"
		if m.SoundnessPassed {
			status = "SOUND (PASS)"
			passedRules++
		}
		t.Logf("%-12s | %-12s | %-10d | %-10d | %-12d | %s",
			m.RuleCode, m.Category, m.PositiveCount, m.NegativeCount, m.AdversarialCount, status)
	}
	t.Log("=========================================================================================================")
	t.Logf("Aggregate Metrics:")
	t.Logf("  Total Positive Violations Detected : %d", totalPos)
	t.Logf("  Total Negative Suites Preserved    : %d / %d (100%% Zero-Noise Invariant)", totalNegClean, totalRules)
	t.Logf("  Total Adversarial Vectors Caught   : %d", totalAdv)
	t.Logf("  Semantic Correctness Gate Score    : %d / %d rules sound (%.1f%%)",
		passedRules, totalRules, float64(passedRules)/float64(totalRules)*100)
	t.Log("=========================================================================================================")

	if passedRules != totalRules {
		t.Fatalf("GATE 2 FAILED: %d / %d rules failed semantic soundness evaluation", totalRules-passedRules, totalRules)
	}
}
