package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/will2469/argus/shared/config"
	mcperrors "github.com/will2469/argus/shared/mcp/errors"
	"github.com/will2469/argus/shared/mcp/security"
	"github.com/will2469/argus/shared/mcp/tools"
)

// NewReportIssueTool initializes the argus_report_issue tool.
func NewReportIssueTool() tools.Tool {
	return &reportIssueTool{}
}

func (t *reportIssueTool) Name() string {
	return "argus_report_issue"
}

func (t *reportIssueTool) Cost() tools.ResourceCost {
	return tools.CostCheap
}

func (t *reportIssueTool) Definition() tools.ToolDef {
	return tools.ToolDef{
		Name: "argus_report_issue",
		Description: "HUMAN-IN-THE-LOOP: Reports a false positive, missing scenario, or rule " +
			"improvement to the Argus GitHub repository. This tool enforces a CRYPTOGRAPHIC TWO-PHASE flow: " +
			"(1) Call without approval_token to generate a preview draft and obtain a short-lived, single-use approval token. " +
			"You MUST present this preview and ask for explicit human approval. " +
			"(2) After the user explicitly approves, call again with the identical payload and the provided 'approval_token' to submit. " +
			"Any modification to the payload will invalidate the approval token.",
		InputSchema: security.Schema{
			Type: "object",
			Properties: map[string]security.Property{
				"rule_code": {
					Type:        "string",
					Description: "The Argus rule code related to this report, e.g. \"A14\", \"A17\".",
				},
				"title": {
					Type:        "string",
					Description: "Brief summary of the issue.",
				},
				"description": {
					Type:        "string",
					Description: "Detailed explanation of the false positive, missing scenario, or improvement.",
				},
				"snippet": {
					Type:        "string",
					Description: "The Go or SQL code snippet that triggered the issue.",
				},
				"category": {
					Type:        "string",
					Description: "One of: false-positive, missing-scenario, rule-improvement.",
					Enum:        []string{"false-positive", "missing-scenario", "rule-improvement"},
				},
				"confirm": {
					Type:        "boolean",
					Description: "Optional confirmation flag (legacy).",
				},
				"approval_token": {
					Type:        "string",
					Description: "Cryptographic single-use token obtained from Phase 1 preview. REQUIRED to execute outbound submission.",
				},
			},
			Required: []string{"title", "description"},
		},
	}
}

func (t *reportIssueTool) ValidatePolicy(rawArgs json.RawMessage) error {
	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Snippet     string `json:"snippet"`
	}
	if err := json.Unmarshal(rawArgs, &input); err != nil {
		return fmt.Errorf("failed to parse report issue input: %w", err)
	}
	if len(input.Title) > security.MaxReportTitleChars {
		return fmt.Errorf("title exceeds maximum limit of %d characters", security.MaxReportTitleChars)
	}
	if len(input.Description) > security.MaxReportPayloadBytes {
		return fmt.Errorf("description exceeds maximum limit of %d bytes", security.MaxReportPayloadBytes)
	}
	if len(input.Snippet) > security.MaxReportPayloadBytes {
		return fmt.Errorf("snippet exceeds maximum limit of %d bytes", security.MaxReportPayloadBytes)
	}
	return nil
}

func (t *reportIssueTool) Execute(ctx context.Context, id any, rawArgs json.RawMessage) *mcperrors.JSONRPCResponse {
	return HandleReportIssue(id, rawArgs)
}

// HandleReportIssue implements a cryptographically bound Human-in-the-Loop (HITL) pattern.
func HandleReportIssue(id any, args json.RawMessage) *mcperrors.JSONRPCResponse {
	cfg, _ := config.LoadConfig(".")
	if !cfg.IsTelemetryEnabled() {
		return mcperrors.ToolError(id, "🔒 Issue reporting is disabled by policy (telemetry: false / ARGUS_TELEMETRY=false). Outbound submission blocked.")
	}

	var input ReportIssueInput
	if err := json.Unmarshal(args, &input); err != nil {
		return mcperrors.InvalidParamsError(id, "Invalid params")
	}

	if input.Title == "" || input.Description == "" {
		return mcperrors.InvalidParamsError(id, "Missing required parameters: title and description")
	}

	category := input.Category
	if category == "" {
		category = "false-positive"
	}

	issueTitle := FormatIssueTitle(input.RuleCode, input.Title)
	issueBody := FormatIssueBody(input.RuleCode, input.Description, input.Snippet, category)
	label := LabelForCategory(category)

	payloadHash := security.ComputeIssueHash(input.RuleCode, input.Title, input.Description, input.Snippet, category)

	// Phase 1: Preview Mode
	if input.ApprovalToken == "" {
		token, err := security.DefaultApprovalManager.CreateToken(payloadHash)
		if err != nil {
			return mcperrors.ToolError(id, fmt.Sprintf("Failed to generate approval token: %v", err))
		}
		return BuildPreview(id, issueTitle, issueBody, category, token)
	}

	// Phase 2: Submission Mode
	if err := security.DefaultApprovalManager.ConsumeToken(input.ApprovalToken, payloadHash); err != nil {
		return mcperrors.ToolError(id, fmt.Sprintf("🔒 HUMAN APPROVAL AUTHORIZATION REJECTED: %v", err))
	}

	// Use configured absolute path if available, otherwise check PATH
	ghPath := cfg.GetGHCliPath()
	if ghPath != "" {
		return SubmitViaGH(id, issueTitle, issueBody, label, ghPath)
	}
	if _, err := exec.LookPath("gh"); err == nil {
		return SubmitViaGH(id, issueTitle, issueBody, label, "")
	}
	return SubmitViaURL(id, issueTitle, issueBody, label, "GitHub CLI (`gh`) not found in PATH")
}
