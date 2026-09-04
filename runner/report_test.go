package runner

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/tools/go/analysis"
)

func TestGenerateMarkdownReport_Clean(t *testing.T) {
	result := &AuditResult{
		Timestamp:                  time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC),
		ScannedFiles:               100,
		VerifiedQuerySites:         50,
		VerifiedParameterizedSites: 120,
		Issues:                     nil,
		Score:                      100.0,
		Grade:                      "A+",
	}

	report := GenerateMarkdownReport(result, "/fake/root")
	if !strings.Contains(report, "PASSED (Clean)") {
		t.Errorf("expected report to contain PASSED (Clean)")
	}
	if !strings.Contains(report, "## Summary") {
		t.Errorf("expected report to contain Summary section")
	}
	if !strings.Contains(report, "## Detailed Info") {
		t.Errorf("expected report to contain Detailed Info section")
	}
	if !strings.Contains(report, "No known vulnerabilities found") {
		t.Errorf("expected report to contain 'No known vulnerabilities found'")
	}
	if !strings.Contains(report, "| A01 | UNSAFE_SQL_CONCATENATION | 50 | 0 | PASS |") {
		t.Errorf("expected report to contain detailed row for A01")
	}
}

func TestGenerateMarkdownReport_WithIssues(t *testing.T) {
	result := &AuditResult{
		Timestamp:    time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC),
		ScannedFiles: 10,
		Issues: []Issue{
			{
				File:     "repo.go",
				Line:     42,
				Rule:     "FORBIDDEN_SELECT_STAR",
				Message:  "Wildcard query forbidden",
				Snippet:  "SELECT * FROM users",
				Category: "performance",
			},
		},
		Score: 95.0,
		Grade: "A",
	}

	report := GenerateMarkdownReport(result, "/fake/root")
	if !strings.Contains(report, "FAILED (Violations Found)") {
		t.Errorf("expected report to indicate violations")
	}
	if !strings.Contains(report, "FORBIDDEN_SELECT_STAR") {
		t.Errorf("expected report to list FORBIDDEN_SELECT_STAR")
	}
	if !strings.Contains(report, "| A14 | FORBIDDEN_SELECT_STAR | 10 | 1 | FAILED |") {
		t.Errorf("expected report to contain detailed row with FAILED status for A14")
	}
}

func TestBuildDynamicRuleAuditInfo_AnalyzerMetaAndScannerNameMapping(t *testing.T) {
	analyzers := []*analysis.Analyzer{
		{Name: "a18"},
		{Name: "a20"},
		{Name: "a21"},
		{Name: "a23"},
		{Name: "a25"},
		{Name: "a26"},
	}

	issues := []Issue{
		{Rule: "MISSING_ROWS_ERR_CHECK", Line: 10, File: "a18.go", Message: "err check missing"},
		{Rule: "UNBOUNDED_BATCH_PARAMS", Line: 20, File: "a20.go", Message: "batch limit exceeded"},
		{Rule: "BLOCKING_ROW_LOCK", Line: 30, File: "a21.go", Message: "row lock without skip locked"},
		{Rule: "MISSING_TX_TIMEOUT", Line: 40, File: "a23.go", Message: "tx timeout missing"},
		{Rule: "EXPENSIVE_CPU_IN_TX", Line: 50, File: "a25.go", Message: "expensive cpu in tx"},
		{Rule: "LIKE_WILDCARD_INJECTION", Line: 60, File: "a26.go", Message: "like wildcard unsanitized"},
	}

	rulesInfo := BuildDynamicRuleAuditInfo(analyzers, nil, 10, 5, 20, issues)
	if len(rulesInfo) != 6 {
		t.Fatalf("expected 6 rules, got %d", len(rulesInfo))
	}

	for _, r := range rulesInfo {
		if r.IssuesFound != 1 {
			t.Errorf("rule %s (%s): expected 1 issue found, got %d", r.ID, r.Code, r.IssuesFound)
		}
		if r.Status != "FAILED" {
			t.Errorf("rule %s (%s): expected status FAILED, got %s", r.ID, r.Code, r.Status)
		}
		if !strings.HasPrefix(r.Code, "ARGUS-") {
			t.Errorf("rule %s: expected Code to have ARGUS- prefix, got %s", r.ID, r.Code)
		}
	}

	// Verify report generation displays canonical headers for these scanner names
	res := &AuditResult{
		AttachedRules: rulesInfo,
		Issues:        issues,
		Score:         50.0,
		Grade:         "F",
	}
	report := GenerateMarkdownReport(res, "/test/root")
	expectedHeaders := []string{
		"### ARGUS-A18 (MISSING_ROWS_ERR_CHECK)",
		"### ARGUS-A20 (PARAM_LIMIT_65535)",
		"### ARGUS-A21 (UNBOUNDED_ROW_LOCK_BLOCKING)",
		"### ARGUS-A23 (TRANSACTION_TIMEOUT_CONFIG)",
		"### ARGUS-A25 (EXPENSIVE_CPU_IN_TRANSACTION)",
		"### ARGUS-A26 (LIKE_WILDCARD_INJECTION)",
	}
	for _, h := range expectedHeaders {
		if !strings.Contains(report, h) {
			t.Errorf("expected report to contain header %q, but got:\n%s", h, report)
		}
	}
}

