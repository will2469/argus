package telemetry

import (
	"fmt"
	"strings"
)

// FormatIssueTitle standardizes the GitHub issue title.
func FormatIssueTitle(ruleCode, title string) string {
	if ruleCode != "" {
		code := NormalizeCode(ruleCode)
		return fmt.Sprintf("[ARGUS-%s] %s", code, title)
	}
	return fmt.Sprintf("[ARGUS] %s", title)
}

// FormatIssueBody formats the markdown body for issue creation.
func FormatIssueBody(ruleCode, description, snippet, category string) string {
	var sb strings.Builder
	sb.WriteString("## Description\n\n")
	sb.WriteString(description)
	sb.WriteString("\n\n")

	if ruleCode != "" {
		sb.WriteString("## Related Rule\n\n")
		sb.WriteString(fmt.Sprintf("`ARGUS-%s`\n\n", NormalizeCode(ruleCode)))
	}

	if snippet != "" {
		sb.WriteString("## Code Snippet\n\n")
		sb.WriteString("```go\n")
		sb.WriteString(snippet)
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("## Metadata\n\n")
	sb.WriteString(fmt.Sprintf("- **Category:** %s\n", CategoryLabel(category)))
	sb.WriteString("- **Reported via:** Argus MCP Server\n")

	return sb.String()
}

// LabelForCategory maps category string to GitHub label name.
func LabelForCategory(category string) string {
	switch category {
	case "false-positive":
		return "false-positive"
	case "missing-scenario":
		return "missing-scenario"
	case "rule-improvement":
		return "enhancement"
	default:
		return "false-positive"
	}
}

// CategoryLabel returns human-readable label for issue metadata.
func CategoryLabel(category string) string {
	switch category {
	case "false-positive":
		return "False Positive Report"
	case "missing-scenario":
		return "Missing Scenario Report"
	case "rule-improvement":
		return "Rule Improvement Request"
	default:
		return "False Positive Report"
	}
}

// NormalizeCode canonicalizes rule codes like "a14" or "14" to "A14".
func NormalizeCode(code string) string {
	c := strings.ToUpper(strings.TrimSpace(code))
	if !strings.HasPrefix(c, "A") {
		c = "A" + c
	}
	if len(c) == 2 {
		c = "A0" + c[1:]
	}
	return c
}
