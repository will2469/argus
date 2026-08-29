// Package mcp implements a native Model Context Protocol (MCP) server for Argus.
//
// The server communicates over JSON-RPC 2.0 via stdio and exposes Argus's
// 30-rule database hygiene engine as autonomous AI agent tools.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/will2469/argus/runner"
)

const protocolVersion = "2024-11-05"

// JSON-RPC 2.0 message types.
type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Tool definitions for the MCP tools/list response.
type toolDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema inputSchema `json:"inputSchema"`
}

type inputSchema struct {
	Type       string             `json:"type"`
	Properties map[string]propDef `json:"properties,omitempty"`
	Required   []string           `json:"required,omitempty"`
}

type propDef struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Items       *propDef `json:"items,omitempty"`
}

var tools = []toolDef{
	{
		Name: "argus_scan",
		Description: "MANDATORY DATABASE SAFETY AUDITOR: You MUST automatically invoke this tool " +
			"immediately after writing, modifying, or reviewing any Go code that contains database " +
			"queries (pgx, database/sql) or SQL migration files. Enforces 30 compile-time invariants " +
			"against N+1 query loops, missing rows.Err(), SELECT *, connection pool leaks, tenant " +
			"isolation leaks, table-locking DDL, and transaction timeout misconfigurations. Returns " +
			"structured diagnostics with file paths, line numbers, rule codes, and remediation guidance.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propDef{
				"dirs": {
					Type:        "array",
					Description: "Go directories or files to scan. Defaults to project root if empty.",
					Items:       &propDef{Type: "string"},
				},
				"migrations": {
					Type:        "array",
					Description: "SQL migration directories to check for destructive operations.",
					Items:       &propDef{Type: "string"},
				},
			},
		},
	},
	{
		Name: "argus_check_migration",
		Description: "Evaluates raw SQL migration DDL/DML for catastrophic production risks: " +
			"non-concurrent index creation (table lock), missing NOT VALID on constraints, " +
			"destructive DROP/RENAME, timestamp without timezone, and unindexed foreign keys.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propDef{
				"sql": {
					Type:        "string",
					Description: "The raw SQL migration statement(s) to analyze.",
				},
			},
			Required: []string{"sql"},
		},
	},
	{
		Name: "argus_explain_rule",
		Description: "Retrieves authoritative documentation, PostgreSQL engine internals, " +
			"and compliant remediation patterns for any Argus rule (A01 through A30).",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propDef{
				"rule_code": {
					Type:        "string",
					Description: "The Argus rule code, e.g. \"A01\", \"A17\", \"A23\".",
				},
			},
			Required: []string{"rule_code"},
		},
	},
}

// ServeStdio starts the MCP server reading from stdin and writing to stdout.
func ServeStdio() error {
	return serve(os.Stdin, os.Stdout)
}

func serve(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	// MCP messages can be large; allow up to 10MB per line.
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req jsonrpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			writeResponse(w, jsonrpcResponse{
				JSONRPC: "2.0",
				Error:   jsonrpcError{Code: -32700, Message: "Parse error"},
			})
			continue
		}

		resp := handleRequest(req)
		if resp != nil {
			writeResponse(w, *resp)
		}
	}
	return scanner.Err()
}

func handleRequest(req jsonrpcRequest) *jsonrpcResponse {
	switch req.Method {
	case "initialize":
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": protocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo": map[string]any{
					"name":    "argus",
					"version": "1.0.0",
				},
			},
		}

	case "notifications/initialized", "notifications/cancelled":
		// Notifications have no response.
		return nil

	case "ping":
		return &jsonrpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}

	case "tools/list":
		return &jsonrpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": tools}}

	case "tools/call":
		return handleToolCall(req)

	default:
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   jsonrpcError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)},
		}
	}
}

func handleToolCall(req jsonrpcRequest) *jsonrpcResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &jsonrpcResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: jsonrpcError{Code: -32602, Message: "Invalid params"},
		}
	}

	switch params.Name {
	case "argus_scan":
		return handleScan(req.ID, params.Arguments)
	case "argus_check_migration":
		return handleCheckMigration(req.ID, params.Arguments)
	case "argus_explain_rule":
		return handleExplainRule(req.ID, params.Arguments)
	default:
		return &jsonrpcResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: jsonrpcError{Code: -32602, Message: fmt.Sprintf("Unknown tool: %s", params.Name)},
		}
	}
}

func writeResponse(w io.Writer, resp jsonrpcResponse) {
	data, _ := json.Marshal(resp)
	fmt.Fprintf(w, "%s\n", data)
}

// textContent builds an MCP-compliant content array with a single text item.
func textContent(text string) []map[string]string {
	return []map[string]string{{"type": "text", "text": text}}
}

func handleScan(id any, args json.RawMessage) *jsonrpcResponse {
	var input struct {
		Dirs       []string `json:"dirs"`
		Migrations []string `json:"migrations"`
	}
	if args != nil {
		json.Unmarshal(args, &input)
	}

	cfg := runner.AuditConfig{
		RootDir:       ".",
		ScanDirs:      input.Dirs,
		MigrationDirs: input.Migrations,
	}

	result, err := runner.RunAuditWithConfig(cfg)
	if err != nil {
		return &jsonrpcResponse{
			JSONRPC: "2.0", ID: id,
			Result: map[string]any{"content": textContent(fmt.Sprintf("Scan error: %v", err)), "isError": true},
		}
	}

	if len(result.Issues) == 0 {
		summary := fmt.Sprintf("✓ CLEAN — No violations found.\nScanned %d files, verified %d query sites.\nGrade: %s (%.1f/100)",
			result.ScannedFiles, result.VerifiedQuerySites, result.Grade, result.Score)
		return &jsonrpcResponse{
			JSONRPC: "2.0", ID: id,
			Result: map[string]any{"content": textContent(summary)},
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("⚠ VIOLATIONS FOUND: %d issue(s)\n\n", len(result.Issues)))
	for i, issue := range result.Issues {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s:%d\n   %s\n", i+1, issue.Rule, issue.File, issue.Line, issue.Message))
		if issue.Snippet != "" {
			sb.WriteString(fmt.Sprintf("   Snippet: %s\n", issue.Snippet))
		}
		sb.WriteString("\n")
	}
	sb.WriteString(fmt.Sprintf("Grade: %s (%.1f/100) | Files: %d | Query Sites: %d",
		result.Grade, result.Score, result.ScannedFiles, result.VerifiedQuerySites))

	return &jsonrpcResponse{
		JSONRPC: "2.0", ID: id,
		Result: map[string]any{"content": textContent(sb.String())},
	}
}

func handleCheckMigration(id any, args json.RawMessage) *jsonrpcResponse {
	var input struct {
		SQL string `json:"sql"`
	}
	if err := json.Unmarshal(args, &input); err != nil || input.SQL == "" {
		return &jsonrpcResponse{
			JSONRPC: "2.0", ID: id,
			Error: jsonrpcError{Code: -32602, Message: "Missing required parameter: sql"},
		}
	}

	tmpDir, err := os.MkdirTemp("", "argus-mcp-migration-*")
	if err != nil {
		return &jsonrpcResponse{
			JSONRPC: "2.0", ID: id,
			Result: map[string]any{"content": textContent(fmt.Sprintf("Failed to create temp dir: %v", err)), "isError": true},
		}
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := tmpDir + "/001_check.up.sql"
	if err := os.WriteFile(tmpFile, []byte(input.SQL), 0644); err != nil {
		return &jsonrpcResponse{
			JSONRPC: "2.0", ID: id,
			Result: map[string]any{"content": textContent(fmt.Sprintf("Failed to write temp file: %v", err)), "isError": true},
		}
	}

	cfg := runner.AuditConfig{
		RootDir:       tmpDir,
		MigrationDirs: []string{tmpDir},
	}

	result, err := runner.RunAuditWithConfig(cfg)
	if err != nil {
		return &jsonrpcResponse{
			JSONRPC: "2.0", ID: id,
			Result: map[string]any{"content": textContent(fmt.Sprintf("Migration check error: %v", err)), "isError": true},
		}
	}

	if len(result.Issues) == 0 {
		return &jsonrpcResponse{
			JSONRPC: "2.0", ID: id,
			Result: map[string]any{"content": textContent("✓ Migration SQL is safe. No violations detected.")},
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("⚠ Migration has %d issue(s):\n\n", len(result.Issues)))
	for i, issue := range result.Issues {
		sb.WriteString(fmt.Sprintf("%d. [%s] Line %d: %s\n", i+1, issue.Rule, issue.Line, issue.Message))
	}

	return &jsonrpcResponse{
		JSONRPC: "2.0", ID: id,
		Result: map[string]any{"content": textContent(sb.String())},
	}
}

func handleExplainRule(id any, args json.RawMessage) *jsonrpcResponse {
	var input struct {
		RuleCode string `json:"rule_code"`
	}
	if err := json.Unmarshal(args, &input); err != nil || input.RuleCode == "" {
		return &jsonrpcResponse{
			JSONRPC: "2.0", ID: id,
			Error: jsonrpcError{Code: -32602, Message: "Missing required parameter: rule_code"},
		}
	}

	code := strings.ToUpper(strings.TrimSpace(input.RuleCode))
	if !strings.HasPrefix(code, "A") {
		code = "A" + code
	}
	if len(code) == 2 {
		code = "A0" + code[1:]
	}

	desc, ok := runner.CanonicalDescriptions[code]
	if !ok {
		return &jsonrpcResponse{
			JSONRPC: "2.0", ID: id,
			Result: map[string]any{
				"content": textContent(fmt.Sprintf("Unknown rule code: %s. Valid codes: A01 through A30.", code)),
				"isError": true,
			},
		}
	}

	wikiURL := fmt.Sprintf("https://github.com/will2469/argus/wiki/ARGUS-%s", code)
	summary := fmt.Sprintf("## ARGUS-%s: %s\n\n"+
		"**Rule Code:** ARGUS-%s\n"+
		"**Canonical Name:** %s\n"+
		"**Full Documentation:** %s\n\n"+
		"Use the Wiki link above for complete examples, compliant patterns, "+
		"PostgreSQL internals explanation, and remediation guidance.",
		code, desc, code, desc, wikiURL)

	return &jsonrpcResponse{
		JSONRPC: "2.0", ID: id,
		Result: map[string]any{"content": textContent(summary)},
	}
}
