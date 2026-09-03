package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/will2469/argus/runner"
	"github.com/will2469/argus/shared/config"
	mcperrors "github.com/will2469/argus/shared/mcp/errors"
	"github.com/will2469/argus/shared/mcp/security"
)

type scanTool struct {
	allowedRoots []string
}

// NewScanTool initializes the argus_scan tool with default roots.
func NewScanTool() Tool {
	return &scanTool{}
}

// NewScanToolWithRoots initializes the argus_scan tool with explicit allowed roots.
func NewScanToolWithRoots(roots []string) Tool {
	return &scanTool{allowedRoots: roots}
}

func (t *scanTool) Name() string {
	return "argus_scan"
}

func (t *scanTool) Cost() ResourceCost {
	return CostExpensive
}

func (t *scanTool) Definition() ToolDef {
	return ToolDef{
		Name: "argus_scan",
		Description: "Database safety auditor for Go and PostgreSQL. Analyzes Go code containing " +
			"database queries (pgx, database/sql) and SQL migration files to enforce 30 compile-time " +
			"invariants against N+1 query loops, missing rows.Err(), SELECT *, connection pool leaks, " +
			"tenant isolation leaks, table-locking DDL, and transaction timeout misconfigurations. " +
			"Recommended for use after writing, modifying, or reviewing database-related code. Returns " +
			"structured diagnostics with file paths, line numbers, rule codes, and remediation guidance.",
		InputSchema: security.Schema{
			Type: "object",
			Properties: map[string]security.Property{
				"dirs": {
					Type:        "array",
					Description: "Go directories or files to scan. Defaults to project root if empty.",
					Items:       &security.Property{Type: "string"},
				},
				"migrations": {
					Type:        "array",
					Description: "SQL migration directories to check for destructive operations.",
					Items:       &security.Property{Type: "string"},
				},
			},
		},
	}
}

func (t *scanTool) getAuthority() (*security.PathAuthority, error) {
	if len(t.allowedRoots) > 0 {
		return security.NewPathAuthority(t.allowedRoots...)
	}
	cfg, err := config.LoadConfig(".")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	roots := cfg.GetAllowedRoots()
	if len(roots) == 0 {
		return nil, fmt.Errorf("no allowed roots configured")
	}
	return security.NewPathAuthority(roots...)
}

func (t *scanTool) ValidatePolicy(rawArgs json.RawMessage) error {
	var input struct {
		Dirs       []string `json:"dirs"`
		Migrations []string `json:"migrations"`
	}
	// Allow empty args for default behavior (scan project root)
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &input); err != nil {
			return fmt.Errorf("invalid arguments: %w", err)
		}
	}
	if len(input.Dirs)+len(input.Migrations) > security.MaxScanDirsLimit {
		return fmt.Errorf("too many scan directories specified (limit: %d)", security.MaxScanDirsLimit)
	}

	authority, err := t.getAuthority()
	if err != nil {
		return fmt.Errorf("failed to initialize path authority: %w", err)
	}

	for _, dir := range input.Dirs {
		if _, err := authority.ValidatePath(dir); err != nil {
			return err
		}
	}
	for _, mig := range input.Migrations {
		if _, err := authority.ValidatePath(mig); err != nil {
			return err
		}
	}
	return nil
}

func (t *scanTool) Execute(ctx context.Context, id any, rawArgs json.RawMessage) *mcperrors.JSONRPCResponse {
	var input struct {
		Dirs       []string `json:"dirs"`
		Migrations []string `json:"migrations"`
	}
	// Allow empty args for default behavior (scan project root)
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &input); err != nil {
			return mcperrors.ToolError(id, fmt.Sprintf("Invalid arguments: %v", err))
		}
	}

	authority, err := t.getAuthority()
	if err != nil {
		return mcperrors.ToolError(id, fmt.Sprintf("Security configuration error: %v", err))
	}

	// Mandatory capability acquisition: Atomically validate and open root capability
	cap, cleanDirs, cleanMigs, err := authority.AuthorizeAndOpen(input.Dirs, input.Migrations)
	if err != nil {
		return mcperrors.ToolError(id, fmt.Sprintf("Path authority violation: %v", err))
	}
	defer cap.Close()

	cfg := runner.AuditConfig{
		RootDir:       authority.PrimaryRoot(),
		ScanDirs:      cleanDirs,
		MigrationDirs: cleanMigs,
		Context:       ctx,
		FS:            cap.FS(),
	}

	result, err := runner.RunAuditWithConfig(cfg)
	if err != nil {
		if errors.Is(err, context.Canceled) || (ctx != nil && ctx.Err() != nil) {
			return mcperrors.CancelledError(id, "Request cancelled by client")
		}
		return mcperrors.ToolError(id, fmt.Sprintf("Scan error: %v", err))
	}

	// Guard against 0 scanned targets being misrepresented as clean
	if result.ScannedFiles == 0 && result.ScannedMigrationFiles == 0 {
		var targets string
		if len(input.Dirs) > 0 || len(input.Migrations) > 0 {
			targets = fmt.Sprintf("dirs: %v, migrations: %v", input.Dirs, input.Migrations)
		} else {
			targets = "default project root"
		}
		return mcperrors.ToolError(id, fmt.Sprintf("⚠ NO TARGETS ANALYZED: No Go source files (.go) or SQL migrations (.sql) were found in %s.\n"+
			"Verify that specified directories exist and contain readable files.", targets))
	}

	if len(result.Issues) == 0 {
		summary := fmt.Sprintf("✓ CLEAN — No violations found.\nScanned %d files, verified %d query sites.\nGrade: %s (%.1f/100)",
			result.ScannedFiles, result.VerifiedQuerySites, result.Grade, result.Score)
		return mcperrors.ToolSuccess(id, summary)
	}

	hasAnalysisFailure := HasAnalysisFailure(result.Issues)
	var sb strings.Builder
	if hasAnalysisFailure {
		sb.WriteString(fmt.Sprintf("❌ ANALYSIS FAILED: %d issue(s) encountered (parser or compiler errors)\n\n", len(result.Issues)))
	} else {
		sb.WriteString(fmt.Sprintf("⚠ VIOLATIONS FOUND: %d issue(s)\n\n", len(result.Issues)))
	}

	for i, issue := range result.Issues {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s:%d\n   %s\n", i+1, issue.Rule, issue.File, issue.Line, issue.Message))
		if issue.Snippet != "" {
			sb.WriteString(fmt.Sprintf("   Snippet: %s\n", issue.Snippet))
		}
		sb.WriteString("\n")
	}
	sb.WriteString(fmt.Sprintf("Grade: %s (%.1f/100) | Files: %d | Query Sites: %d",
		result.Grade, result.Score, result.ScannedFiles, result.VerifiedQuerySites))

	if hasAnalysisFailure {
		return mcperrors.ToolError(id, sb.String())
	}
	return mcperrors.ToolSuccess(id, sb.String())
}

// HasAnalysisFailure detects internal parser or engine failure (ARGUS-E...).
func HasAnalysisFailure(issues []runner.Issue) bool {
	for _, issue := range issues {
		if strings.HasPrefix(issue.Rule, "ARGUS-E") {
			return true
		}
	}
	return false
}
