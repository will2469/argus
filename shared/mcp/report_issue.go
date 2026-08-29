package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/will2469/argus/shared/config"
)

const issueRepoSlug = "will2469/argus"

// handleReportIssue implements a two-phase Human-in-the-Loop (HITL) pattern:
//
// Phase 1 (confirm=false, default): Returns a formatted preview of the issue
// for the AI agent to show to the human. No side effects.
//
// Phase 2 (confirm=true): Only after the human explicitly approves, the issue
// is submitted to GitHub via `gh` CLI or a pre-filled browser URL.
func handleReportIssue(id any, args json.RawMessage) *jsonrpcResponse {
	cfg, _ := config.LoadConfig(".")
	if !cfg.IsTelemetryEnabled() {
		return &jsonrpcResponse{
			JSONRPC: "2.0", ID: id,
			Result: map[string]any{
				"content": textContent("🔒 Issue reporting is disabled by policy (telemetry: false / ARGUS_TELEMETRY=false). Outbound submission blocked."),
				"isError": true,
			},
		}
	}

	var input struct {
		RuleCode    string `json:"rule_code"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Snippet     string `json:"snippet"`
		Category    string `json:"category"`
		Confirm     bool   `json:"confirm"`
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

	issueTitle := formatIssueTitle(input.RuleCode, input.Title)
	issueBody := formatIssueBody(input.RuleCode, input.Description, input.Snippet, category)
	label := labelForCategory(category)

	// Phase 1: Preview mode (default). Show draft to the human for approval.
	if !input.Confirm {
		return buildPreview(id, issueTitle, issueBody, category)
	}

	// Phase 2: Human approved. Submit the issue.
	if ghPath, err := exec.LookPath("gh"); err == nil {
		return submitViaGH(id, ghPath, issueTitle, issueBody, label)
	}
	return submitViaURL(id, issueTitle, issueBody, label)
}

func buildPreview(id any, title, body, category string) *jsonrpcResponse {
	var sb strings.Builder
	sb.WriteString("📋 **Issue Draft Preview — Awaiting Human Approval**\n\n")
	sb.WriteString(fmt.Sprintf("**Title:** %s\n", title))
	sb.WriteString(fmt.Sprintf("**Category:** %s\n", categoryLabel(category)))
	sb.WriteString(fmt.Sprintf("**Repository:** github.com/%s\n\n", issueRepoSlug))
	sb.WriteString("---\n\n")
	sb.WriteString(body)
	sb.WriteString("\n---\n\n")
	sb.WriteString("⚠️ **This issue has NOT been submitted yet.**\n")
	sb.WriteString("Ask the user for approval, then call this tool again with `\"confirm\": true` to submit.")

	return &jsonrpcResponse{
		JSONRPC: "2.0", ID: id,
		Result: map[string]any{"content": textContent(sb.String())},
	}
}

func submitViaGH(id any, ghPath, title, body, label string) *jsonrpcResponse {
	args := []string{"issue", "create",
		"--repo", issueRepoSlug,
		"--title", title,
		"--body", body,
		"--label", label,
	}

	cmd := exec.Command(ghPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return submitViaURL(id, title, body, label)
	}

	issueURL := strings.TrimSpace(string(output))
	msg := fmt.Sprintf("✓ Issue submitted successfully!\n\n%s", issueURL)
	return &jsonrpcResponse{
		JSONRPC: "2.0", ID: id,
		Result: map[string]any{"content": textContent(msg)},
	}
}

func submitViaURL(id any, title, body, label string) *jsonrpcResponse {
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
	sb.WriteString("*This issue was filed via `argus mcp` → `argus_report_issue` with human approval.*\n")

	return sb.String()
}

func labelForCategory(category string) string {
	switch strings.ToLower(category) {
	case "false-positive":
		return "bug"
	case "missing-scenario", "rule-improvement":
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
