package runner

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GenerateMarkdownReport creates the standardized markdown report from AuditResult.
func GenerateMarkdownReport(result *AuditResult, rootDir string) string {
	timestamp := result.Timestamp.Format("2006-01-02T15:04:05.000Z07:00")
	if result.Timestamp.IsZero() {
		timestamp = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	}

	status := "PASSED (Clean)"
	if len(result.Issues) > 0 {
		status = "FAILED (Violations Found)"
	}

	// Calculate components checked
	componentsChecked := result.VerifiedQuerySites + result.VerifiedParameterizedSites
	if componentsChecked == 0 {
		componentsChecked = result.ScannedFiles
	}

	// Dynamic rules list from attached analyzers
	attachedRules := result.AttachedRules
	if len(attachedRules) == 0 {
		attachedRules = BuildDynamicRuleAuditInfo(nil, nil, result.VerifiedQuerySites, result.ScannedMigrationFiles, result.ScannedFiles, result.Issues)
	}

	var sb strings.Builder
	sb.WriteString("# Argus Database SQL Hygiene & Anti-Overfetching Audit Report\n\n")
	sb.WriteString(fmt.Sprintf("**Timestamp:** %s  \n", timestamp))
	sb.WriteString(fmt.Sprintf("**Status:** %s  \n\n", status))

	// 1. Summary Section
	sb.WriteString("## Summary\n\n")
	sb.WriteString("| Metric | Jumlah |\n")
	sb.WriteString("| :--- | :--- |\n")
	sb.WriteString(fmt.Sprintf("| Total Berkas | %d |\n", result.ScannedFiles))
	sb.WriteString(fmt.Sprintf("| Komponen Diperiksa | %d |\n", componentsChecked))
	sb.WriteString(fmt.Sprintf("| Rules Attached | %d |\n", len(attachedRules)))
	sb.WriteString(fmt.Sprintf("| Total Issues | %d |\n\n", len(result.Issues)))

	// 2. Detailed Info Section
	sb.WriteString("## Detailed Info\n\n")
	sb.WriteString("| ID | Descriptions | Komponen Diperiksa | Issues Found | Status |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- | :--- |\n")
	for _, r := range attachedRules {
		sb.WriteString(fmt.Sprintf("| %s | %s | %d | %d | %s |\n",
			r.ID, r.Description, r.CheckedComponents, r.IssuesFound, r.Status))
	}
	sb.WriteString("\n")

	// 3. Result Section
	sb.WriteString("## Result\n\n")
	if len(result.Issues) == 0 {
		sb.WriteString("No known vulnerabilities found\n")
	} else {
		sb.WriteString(fmt.Sprintf("Found %d violations across database operations:\n\n", len(result.Issues)))

		// Group issues by rule
		groupedByRule := make(map[string][]Issue)
		for _, issue := range result.Issues {
			norm := normalizeRuleCode(issue.Rule)
			groupedByRule[norm] = append(groupedByRule[norm], issue)
		}

		var sortedRules []string
		for rule := range groupedByRule {
			sortedRules = append(sortedRules, rule)
		}
		sort.Strings(sortedRules)

		for _, rule := range sortedRules {
			issues := groupedByRule[rule]
			desc := GetRuleDescription(rule)
			sb.WriteString(fmt.Sprintf("### %s (%s)\n\n", rule, desc))
			for _, item := range issues {
				relPath := item.File
				absPath := item.File
				if !filepath.IsAbs(absPath) {
					absPath = filepath.Join(rootDir, item.File)
				} else {
					if rel, err := filepath.Rel(rootDir, absPath); err == nil {
						relPath = rel
					}
				}

				linkPath := "/" + strings.TrimPrefix(filepath.ToSlash(absPath), "/")
				sb.WriteString(fmt.Sprintf("- **[%s:%d](file://%s#L%d)**\n", relPath, item.Line, linkPath, item.Line))
				sb.WriteString(fmt.Sprintf("  - *Rule:* %s\n", item.DisplayTag()))
				sb.WriteString(fmt.Sprintf("  - *Message:* %s\n", item.Message))
				if item.Snippet != "" {
					sb.WriteString(fmt.Sprintf("  - *Snippet:* `%s`\n", item.Snippet))
				}
				directivePrefix := "//"
				if strings.HasSuffix(strings.ToLower(item.File), ".sql") {
					directivePrefix = "--"
				}
				shortID := strings.TrimPrefix(item.RuleCode(), "ARGUS-")
				sb.WriteString(fmt.Sprintf("  - *Suppression:* `%s argus:ignore %s <reason (min 2 words)>`\n", directivePrefix, shortID))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// NormalizeRuleCode standardizes raw rule names, aliases, or identifiers to "ARGUS-Axx".
func NormalizeRuleCode(raw string) string {
	upper := strings.ToUpper(strings.TrimSpace(raw))
	if strings.HasPrefix(upper, "ARGUS-") {
		return upper
	}
	if strings.HasPrefix(upper, "A") && len(upper) == 3 {
		return "ARGUS-" + upper
	}
	if code, ok := RuleAliases[upper]; ok {
		return code
	}
	for id, desc := range CanonicalDescriptions {
		if desc == upper || id == upper {
			return "ARGUS-" + id
		}
	}
	return upper
}

// GetRuleDescription retrieves the canonical uppercase identifier for a rule.
func GetRuleDescription(ruleCode string) string {
	norm := NormalizeRuleCode(ruleCode)
	id := strings.TrimPrefix(norm, "ARGUS-")
	if desc, ok := CanonicalDescriptions[id]; ok {
		return desc
	}
	return ruleCode
}

func normalizeRuleCode(raw string) string {
	return NormalizeRuleCode(raw)
}

func getRuleDescription(ruleCode string) string {
	return GetRuleDescription(ruleCode)
}
