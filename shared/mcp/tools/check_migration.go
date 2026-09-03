package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/will2469/argus/runner"
	mcperrors "github.com/will2469/argus/shared/mcp/errors"
	"github.com/will2469/argus/shared/mcp/security"
)

type checkMigrationTool struct{}

// NewCheckMigrationTool initializes the argus_check_migration tool.
func NewCheckMigrationTool() Tool {
	return &checkMigrationTool{}
}

func (t *checkMigrationTool) Name() string {
	return "argus_check_migration"
}

func (t *checkMigrationTool) Cost() ResourceCost {
	return CostExpensive
}

func (t *checkMigrationTool) Definition() ToolDef {
	return ToolDef{
		Name: "argus_check_migration",
		Description: "Evaluates raw SQL migration DDL/DML for catastrophic production risks: " +
			"non-concurrent index creation (table lock), missing NOT VALID on constraints, " +
			"destructive DROP/RENAME, timestamp without timezone, and unindexed foreign keys.",
		InputSchema: security.Schema{
			Type: "object",
			Properties: map[string]security.Property{
				"sql": {
					Type:        "string",
					Description: "The raw SQL migration statement(s) to analyze.",
				},
			},
			Required: []string{"sql"},
		},
	}
}

func (t *checkMigrationTool) ValidatePolicy(rawArgs json.RawMessage) error {
	var input struct {
		SQL string `json:"sql"`
	}
	if err := json.Unmarshal(rawArgs, &input); err != nil {
		return fmt.Errorf("failed to parse migration sql: %w", err)
	}
	if strings.TrimSpace(input.SQL) == "" {
		return fmt.Errorf("sql statement cannot be empty or whitespace only")
	}
	if len(input.SQL) > security.MaxMigrationSQLBytes {
		return fmt.Errorf("sql statement exceeds maximum allowed size (%d bytes)", security.MaxMigrationSQLBytes)
	}
	return nil
}

func (t *checkMigrationTool) Execute(ctx context.Context, id any, rawArgs json.RawMessage) *mcperrors.JSONRPCResponse {
	var input struct {
		SQL string `json:"sql"`
	}
	_ = json.Unmarshal(rawArgs, &input)

	tmpDir, err := os.MkdirTemp("", "argus-mcp-migration-*")
	if err != nil {
		return mcperrors.ToolError(id, fmt.Sprintf("Failed to create temp dir: %v", err))
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := tmpDir + "/001_check.up.sql"
	if err := os.WriteFile(tmpFile, []byte(input.SQL), 0644); err != nil {
		return mcperrors.ToolError(id, fmt.Sprintf("Failed to write temp file: %v", err))
	}

	cfg := runner.AuditConfig{
		RootDir:       tmpDir,
		MigrationDirs: []string{tmpDir},
		Context:       ctx,
	}

	result, err := runner.RunAuditWithConfig(cfg)
	if err != nil {
		if errors.Is(err, context.Canceled) || (ctx != nil && ctx.Err() != nil) {
			return mcperrors.CancelledError(id, "Request cancelled by client")
		}
		return mcperrors.ToolError(id, fmt.Sprintf("Migration check error: %v", err))
	}

	if len(result.Issues) == 0 {
		return mcperrors.ToolSuccess(id, "✓ Migration SQL is safe. No violations detected.")
	}

	hasAnalysisFailure := HasAnalysisFailure(result.Issues)
	var sb strings.Builder
	if hasAnalysisFailure {
		sb.WriteString(fmt.Sprintf("❌ MIGRATION ANALYSIS FAILED: %d issue(s) encountered (parser or syntax errors):\n\n", len(result.Issues)))
	} else {
		sb.WriteString(fmt.Sprintf("⚠ Migration has %d violation(s):\n\n", len(result.Issues)))
	}

	for i, issue := range result.Issues {
		sb.WriteString(fmt.Sprintf("%d. [%s] Line %d: %s\n", i+1, issue.Rule, issue.Line, issue.Message))
	}

	if hasAnalysisFailure {
		return mcperrors.ToolError(id, sb.String())
	}
	return mcperrors.ToolSuccess(id, sb.String())
}
