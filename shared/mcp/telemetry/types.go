package telemetry

// GitHub repository targeting for outbound issue reporting.
const IssueRepoSlug = "will2469/argus"

const issueRepoSlug = IssueRepoSlug

// reportIssueTool implements the argus_report_issue MCP tool.
type reportIssueTool struct{}

// ReportIssueInput defines the parameters passed to argus_report_issue.
type ReportIssueInput struct {
	RuleCode      string `json:"rule_code"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Snippet       string `json:"snippet"`
	Category      string `json:"category"`
	Confirm       bool   `json:"confirm"`
	ApprovalToken string `json:"approval_token"`
}
