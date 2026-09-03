package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/will2469/argus/runner"
	mcperrors "github.com/will2469/argus/shared/mcp/errors"
	"github.com/will2469/argus/shared/mcp/security"
)

// NewExplainRuleTool initializes the argus_explain_rule tool.
func NewExplainRuleTool() Tool {
	return &explainRuleTool{}
}

func (t *explainRuleTool) Name() string {
	return "argus_explain_rule"
}

func (t *explainRuleTool) Cost() ResourceCost {
	return CostCheap
}

func (t *explainRuleTool) Definition() ToolDef {
	return ToolDef{
		Name: "argus_explain_rule",
		Description: "Retrieves authoritative documentation, PostgreSQL engine internals, " +
			"and compliant remediation patterns for any Argus rule (A01 through A30).",
		InputSchema: security.Schema{
			Type: "object",
			Properties: map[string]security.Property{
				"rule_code": {
					Type:        "string",
					Description: "The Argus rule code, e.g. \"A01\", \"A17\", \"A23\".",
				},
			},
			Required: []string{"rule_code"},
		},
	}
}

func (t *explainRuleTool) ValidatePolicy(rawArgs json.RawMessage) error {
	var input struct {
		RuleCode string `json:"rule_code"`
	}
	if err := json.Unmarshal(rawArgs, &input); err != nil {
		return fmt.Errorf("failed to parse rule_code: %w", err)
	}
	if strings.TrimSpace(input.RuleCode) == "" {
		return fmt.Errorf("rule_code cannot be empty")
	}
	if len(input.RuleCode) > 32 {
		return fmt.Errorf("rule_code exceeds maximum allowed length of 32 characters")
	}
	return nil
}

func (t *explainRuleTool) Execute(ctx context.Context, id any, rawArgs json.RawMessage) *mcperrors.JSONRPCResponse {
	var input struct {
		RuleCode string `json:"rule_code"`
	}
	_ = json.Unmarshal(rawArgs, &input)

	code := strings.ToUpper(strings.TrimSpace(input.RuleCode))
	if !strings.HasPrefix(code, "A") {
		code = "A" + code
	}
	if len(code) == 2 {
		code = "A0" + code[1:]
	}

	desc, ok := runner.CanonicalDescriptions[code]
	if !ok {
		return mcperrors.ToolError(id, fmt.Sprintf("Unknown rule code: %s. Valid codes: A01 through A30.", code))
	}

	wikiURL := fmt.Sprintf("https://github.com/will2469/argus/wiki/ARGUS-%s", code)
	summary := fmt.Sprintf("## ARGUS-%s: %s\n\n"+
		"**Rule Code:** ARGUS-%s\n"+
		"**Canonical Name:** %s\n"+
		"**Full Documentation:** %s\n\n"+
		"Use the Wiki link above for complete examples, compliant patterns, "+
		"PostgreSQL internals explanation, and remediation guidance.",
		code, desc, code, desc, wikiURL)

	return mcperrors.ToolSuccess(id, summary)
}
