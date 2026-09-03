package telemetry

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	mcperrors "github.com/will2469/argus/shared/mcp/errors"
)

const issueRepoSlug = "will2469/argus"

// BuildPreview constructs the Phase 1 draft preview for user confirmation.
func BuildPreview(id any, title, body, category, token string) *mcperrors.JSONRPCResponse {
	var sb strings.Builder
	sb.WriteString("📋 **Issue Draft Preview — Awaiting Human Approval**\n\n")
	sb.WriteString(fmt.Sprintf("**Title:** %s\n", title))
	sb.WriteString(fmt.Sprintf("**Category:** %s\n", CategoryLabel(category)))
	sb.WriteString(fmt.Sprintf("**Repository:** github.com/%s\n\n", issueRepoSlug))
	sb.WriteString("---\n\n")
	sb.WriteString(body)
	sb.WriteString("\n---\n\n")
	sb.WriteString("⚠️ **This issue has NOT been submitted yet.**\n")
	sb.WriteString("Ask the user for explicit approval.\n\n")
	sb.WriteString(fmt.Sprintf("**Approval Token:** `%s`\n", token))
	sb.WriteString("*(Valid for 10 minutes, single-use, bound cryptographically to this exact payload)*\n\n")
	sb.WriteString(fmt.Sprintf("To submit after obtaining human approval, invoke `argus_report_issue` with the identical parameters and `\"approval_token\": \"%s\"`.\n"+
		"Note: Any alteration to title, description, snippet, or category will invalidate this token.", token))

	return mcperrors.ToolSuccess(id, sb.String())
}

// SubmitViaGH performs outbound issue creation using GitHub CLI.
func SubmitViaGH(id any, title, body, label string) *mcperrors.JSONRPCResponse {
	args := []string{"issue", "create",
		"--repo", issueRepoSlug,
		"--title", title,
		"--body", body,
		"--label", label,
	}

	cmd := exec.Command("gh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		reason := fmt.Sprintf("gh command failed: %s", strings.TrimSpace(string(output)))
		return SubmitViaURL(id, title, body, label, reason)
	}

	issueURL := strings.TrimSpace(string(output))
	msg := fmt.Sprintf("STATUS: SUBMITTED\n\n✓ Issue created successfully on GitHub!\nIssue URL: %s", issueURL)
	return mcperrors.ToolSuccess(id, msg)
}

// SubmitViaURL generates a prefilled issue creation URL when GitHub CLI is unavailable.
func SubmitViaURL(id any, title, body, label, reason string) *mcperrors.JSONRPCResponse {
	params := url.Values{}
	params.Set("title", title)
	params.Set("body", body)
	params.Set("labels", label)

	issueURL := fmt.Sprintf("https://github.com/%s/issues/new?%s", issueRepoSlug, params.Encode())

	var sb strings.Builder
	sb.WriteString("STATUS: READY_FOR_SUBMISSION (NOT YET CREATED)\n\n")
	sb.WriteString("⚠️ Automatic submission could not be completed.\n")
	if reason != "" {
		sb.WriteString(fmt.Sprintf("Reason: %s\n\n", reason))
	} else {
		sb.WriteString("Reason: GitHub CLI (`gh`) is not available or authenticated.\n\n")
	}
	sb.WriteString("The issue draft is prepared. Please instruct the human user to open the URL below in a browser to complete the submission:\n\n")
	sb.WriteString(issueURL)

	return mcperrors.ToolSuccess(id, sb.String())
}
