package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

const issueRepoSlug = "will2469/argus"

// handleReportIssue creates a structured GitHub issue on the Argus repository
// for false positives, missing scenarios, or rule improvement suggestions.
// Uses the `gh` CLI if available; otherwise generates a pre-filled browser URL.
func handleReportIssue(id any, args json.RawMessage) *jsonrpcResponse {
	var input struct {
		RuleCode    string `json:"rule_code"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Snippet     string `json:"snippet"`
		Category    string `json:"category"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return &jsonrpcResponse{
			JSONRPC: "2.0", ID: id,
			Error: jsonrpcError{Code: -32602, Message: "Invalid params"},
		}
	}

	if input.Title == "" || input.Description == "" {
		return &jsonrpcResponse{
			JSONRPC: "2.0", ID: id,
			Error: jsonrpcError{Code: -32602, Message: "Missing required parameters: title and description"},
		}
	}

	category := input.Category
	if category == "" {
		category = "false-positive"
	}

	label := labelForCategory(category)
	issueTitle := formatIssueTitle(input.RuleCode, input.Title)
	issueBody := formatIssueBody(input.RuleCode, input.Description, input.Snippet, category)

	// Try gh CLI first (uses existing auth).
	if ghPath, err := exec.LookPath("gh"); err == nil {
		return createViaGH(id, ghPath, issueTitle, issueBody, label)
	}

	// Fallback: generate a pre-filled GitHub issue URL.
	return createViaURL(id, issueTitle, issueBody, label)
}

func createViaGH(id any, ghPath, title, body, label string) *jsonrpcResponse {
	args := []string{"issue", "create",
		"--repo", issueRepoSlug,
		"--title", title,
		"--body", body,
		"--label", label,
	}

	cmd := exec.Command(ghPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Fall back to URL if gh fails (e.g. not authenticated).
		return createViaURL(id, title, body, label)
	}

	issueURL := strings.TrimSpace(string(output))
	msg := fmt.Sprintf("✓ Issue created successfully!\n\n%s", issueURL)
	return &jsonrpcResponse{
		JSONRPC: "2.0", ID: id,
		Result: map[string]any{"content": textContent(msg)},
	}
}

func createViaURL(id any, title, body, label string) *jsonrpcResponse {
	params := url.Values{}
	params.Set("title", title)
	params.Set("body", body)
	params.Set("labels", label)

	issueURL := fmt.Sprintf("https://github.com/%s/issues/new?%s", issueRepoSlug, params.Encode())

	msg := fmt.Sprintf("GitHub CLI (gh) not found or not authenticated.\n\n"+
		"Please open the following URL to submit the issue:\n%s", issueURL)
	return &jsonrpcResponse{
		JSONRPC: "2.0", ID: id,
		Result: map[string]any{"content": textContent(msg)},
	}
}

func formatIssueTitle(ruleCode, title string) string {
	if ruleCode != "" {
		code := normalizeCode(ruleCode)
		return fmt.Sprintf("[ARGUS-%s] %s", code, title)
	}
	return title
}

func formatIssueBody(ruleCode, description, snippet, category string) string {
	var sb strings.Builder

	sb.WriteString("## Report from AI Agent (Automated)\n\n")
	sb.WriteString(fmt.Sprintf("**Category:** %s\n", categoryLabel(category)))

	if ruleCode != "" {
		code := normalizeCode(ruleCode)
		sb.WriteString(fmt.Sprintf("**Rule:** [ARGUS-%s](https://github.com/will2469/argus/wiki/ARGUS-%s)\n", code, code))
	}

	sb.WriteString("\n### Description\n\n")
	sb.WriteString(description)
	sb.WriteString("\n")

	if snippet != "" {
		sb.WriteString("\n### Code Snippet\n\n")
		sb.WriteString("```go\n")
		sb.WriteString(snippet)
		sb.WriteString("\n```\n")
	}

	sb.WriteString("\n---\n")
	sb.WriteString("*This issue was automatically filed by an AI agent via `argus mcp` → `argus_report_issue`.*\n")

	return sb.String()
}

func labelForCategory(category string) string {
	switch strings.ToLower(category) {
	case "false-positive":
		return "bug"
	case "missing-scenario":
		return "enhancement"
	case "rule-improvement":
		return "enhancement"
	default:
		return "bug"
	}
}

func categoryLabel(category string) string {
	switch strings.ToLower(category) {
	case "false-positive":
		return "🐛 False Positive"
	case "missing-scenario":
		return "🔍 Missing Scenario"
	case "rule-improvement":
		return "💡 Rule Improvement"
	default:
		return "🐛 False Positive"
	}
}

func normalizeCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if !strings.HasPrefix(code, "A") {
		code = "A" + code
	}
	if len(code) == 2 {
		code = "A0" + code[1:]
	}
	return code
}
