package runner

import (
	"strings"
	"testing"
	"time"
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
