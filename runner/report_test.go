package runner

import (
	"fmt"
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
		"### MISSING_ROWS_ERR_CHECK (ARGUS-A18)",
		"### PARAM_LIMIT_65535 (ARGUS-A20)",
		"### UNBOUNDED_ROW_LOCK_BLOCKING (ARGUS-A21)",
		"### TRANSACTION_TIMEOUT_CONFIG (ARGUS-A23)",
		"### EXPENSIVE_CPU_IN_TRANSACTION (ARGUS-A25)",
		"### LIKE_WILDCARD_INJECTION (ARGUS-A26)",
	}
	for _, h := range expectedHeaders {
		if !strings.Contains(report, h) {
			t.Errorf("expected report to contain header %q, but got:\n%s", h, report)
		}
	}
}

func TestIssue_DisplayTagAndRuleCode(t *testing.T) {
	tests := []struct {
		rawRule    string
		file       string
		wantCode   string
		wantDesc   string
		wantTag    string
		wantIgnore string
	}{
		{
			rawRule:    "UNSAFE_SQL_CONCATENATION",
			file:       "repo/user.go",
			wantCode:   "ARGUS-A01",
			wantDesc:   "UNSAFE_SQL_CONCATENATION",
			wantTag:    "ARGUS-A01: UNSAFE_SQL_CONCATENATION",
			wantIgnore: "// argus:ignore A01 <reason (min 2 words)>",
		},
		{
			rawRule:    "ARGUS-A11",
			file:       "migrations/001_init.sql",
			wantCode:   "ARGUS-A11",
			wantDesc:   "DESTRUCTIVE_MIGRATION",
			wantTag:    "ARGUS-A11: DESTRUCTIVE_MIGRATION",
			wantIgnore: "-- argus:ignore A11 <reason (min 2 words)>",
		},
		{
			rawRule:    "A24",
			file:       "service/tenant.go",
			wantCode:   "ARGUS-A24",
			wantDesc:   "TENANT_ISOLATION_LEAK",
			wantTag:    "ARGUS-A24: TENANT_ISOLATION_LEAK",
			wantIgnore: "// argus:ignore A24 <reason (min 2 words)>",
		},
		{
			rawRule:    "ARGUS-E001",
			file:       "migrations/invalid.sql",
			wantCode:   "ARGUS-E001",
			wantDesc:   "UNABLE_TO_ANALYZE_MIGRATION",
			wantTag:    "ARGUS-E001: UNABLE_TO_ANALYZE_MIGRATION",
			wantIgnore: "-- argus:ignore E001 <reason (min 2 words)>",
		},
		{
			rawRule:    "CUSTOM_CHECK",
			file:       "custom.go",
			wantCode:   "CUSTOM_CHECK",
			wantDesc:   "CUSTOM_CHECK",
			wantTag:    "CUSTOM_CHECK",
			wantIgnore: "// argus:ignore CUSTOM_CHECK <reason (min 2 words)>",
		},
	}

	for _, tc := range tests {
		iss := Issue{Rule: tc.rawRule, File: tc.file, Line: 10, Message: "test violation"}
		if got := iss.RuleCode(); got != tc.wantCode {
			t.Errorf("RuleCode(%q) = %q, want %q", tc.rawRule, got, tc.wantCode)
		}
		if got := iss.RuleDescription(); got != tc.wantDesc {
			t.Errorf("RuleDescription(%q) = %q, want %q", tc.rawRule, got, tc.wantDesc)
		}
		if got := iss.DisplayTag(); got != tc.wantTag {
			t.Errorf("DisplayTag(%q) = %q, want %q", tc.rawRule, got, tc.wantTag)
		}
		if got := iss.DisplayTitle(); got != tc.wantTag {
			t.Errorf("DisplayTitle(%q) = %q, want %q", tc.rawRule, got, tc.wantTag)
		}

		res := &AuditResult{Issues: []Issue{iss}}
		md := GenerateMarkdownReport(res, "/root")
		expectedHeader := fmt.Sprintf("### %s (%s)", tc.wantDesc, tc.wantCode)
		if !strings.Contains(md, expectedHeader) {
			t.Errorf("GenerateMarkdownReport for %q should contain header %q", tc.rawRule, expectedHeader)
		}
		if !strings.Contains(md, "- **Severity:**") {
			t.Errorf("GenerateMarkdownReport for %q should contain Severity metadata", tc.rawRule)
		}
		if !strings.Contains(md, "- **Category:**") {
			t.Errorf("GenerateMarkdownReport for %q should contain Category metadata", tc.rawRule)
		}
		if !strings.Contains(md, "- **Description:**") {
			t.Errorf("GenerateMarkdownReport for %q should contain Description metadata", tc.rawRule)
		}
		if !strings.Contains(md, "- **Message:**") {
			t.Errorf("GenerateMarkdownReport for %q should contain Message metadata", tc.rawRule)
		}
		if !strings.Contains(md, tc.wantIgnore) {
			t.Errorf("GenerateMarkdownReport for %q should contain suppression hint %q", tc.rawRule, tc.wantIgnore)
		}
	}
}

func TestGetRuleMetadata(t *testing.T) {
	metaA01 := GetRuleMetadata("ARGUS-A01")
	if metaA01.Severity != "CRITICAL" || metaA01.Identifier != "UNSAFE_SQL_CONCATENATION" {
		t.Errorf("unexpected meta for A01: %+v", metaA01)
	}
	if !strings.Contains(metaA01.WikiURL, "ARGUS-A01") {
		t.Errorf("expected wiki url to contain ARGUS-A01, got %s", metaA01.WikiURL)
	}

	metaA18 := GetRuleMetadata("a18")
	if metaA18.Severity != "HIGH" || metaA18.Identifier != "MISSING_ROWS_ERR_CHECK" {
		t.Errorf("unexpected meta for a18: %+v", metaA18)
	}
}
