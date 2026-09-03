package tests

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/will2469/argus/shared/mcp"
	mcperrors "github.com/will2469/argus/shared/mcp/errors"
)

func TestInitialize_ProtocolVersionNegotiation(t *testing.T) {
	// 1a. Supported protocol version (2024-11-05) matches exactly
	t.Run("supported_2024_version_negotiated", func(t *testing.T) {
		req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}` + "\n"
		var out bytes.Buffer
		_ = mcp.Serve(strings.NewReader(req), &out)

		var resp mcperrors.JSONRPCResponse
		_ = json.Unmarshal(out.Bytes(), &resp)
		if resp.Error != nil {
			t.Fatalf("unexpected error: %v", resp.Error)
		}
		resMap := resp.Result.(map[string]any)
		if resMap["protocolVersion"] != "2024-11-05" {
			t.Fatalf("expected protocolVersion=2024-11-05, got: %v", resMap["protocolVersion"])
		}
	})

	// 1b. Supported protocol version (2026-07-28) matches exactly
	t.Run("supported_2026_version_negotiated", func(t *testing.T) {
		req := `{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"2026-07-28"}}` + "\n"
		var out bytes.Buffer
		_ = mcp.Serve(strings.NewReader(req), &out)

		var resp mcperrors.JSONRPCResponse
		_ = json.Unmarshal(out.Bytes(), &resp)
		if resp.Error != nil {
			t.Fatalf("unexpected error: %v", resp.Error)
		}
		resMap := resp.Result.(map[string]any)
		if resMap["protocolVersion"] != "2026-07-28" {
			t.Fatalf("expected protocolVersion=2026-07-28, got: %v", resMap["protocolVersion"])
		}
	})

	// 2. Unsupported protocol version falls back to best supported version (MCP spec)
	t.Run("unsupported_version_falls_back_to_best_supported", func(t *testing.T) {
		req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2099-01-01"}}` + "\n"
		var out bytes.Buffer
		_ = mcp.Serve(strings.NewReader(req), &out)

		var resp mcperrors.JSONRPCResponse
		_ = json.Unmarshal(out.Bytes(), &resp)
		if resp.Error != nil {
			t.Fatalf("unexpected error: %v", resp.Error)
		}
		resMap := resp.Result.(map[string]any)
		if resMap["protocolVersion"] != mcp.LatestProtocolVersion {
			t.Fatalf("expected protocolVersion=%s, got: %v", mcp.LatestProtocolVersion, resMap["protocolVersion"])
		}
	})

	// 3. Missing protocolVersion in params rejected with -32602
	t.Run("missing_protocol_version_rejected", func(t *testing.T) {
		dialogue := strings.Join([]string{
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
			`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		}, "\n") + "\n"
		var out bytes.Buffer
		_ = mcp.Serve(strings.NewReader(dialogue), &out, mcp.WithStrictLifecycle(true))

		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 responses, got %d", len(lines))
		}

		var respInit, respTools mcperrors.JSONRPCResponse
		_ = json.Unmarshal([]byte(lines[0]), &respInit)
		_ = json.Unmarshal([]byte(lines[1]), &respTools)

		if respInit.Error == nil {
			t.Fatal("expected error on initialize with missing protocolVersion")
		}
		errMap := respInit.Error.(map[string]any)
		if int(errMap["code"].(float64)) != mcp.CodeInvalidParams {
			t.Fatalf("expected code -32602, got: %v", errMap["code"])
		}

		// State must remain statePreInit, so tools/list is rejected with -32002
		if respTools.Error == nil {
			t.Fatal("expected tools/list to be rejected after failed initialize")
		}
		toolErrMap := respTools.Error.(map[string]any)
		if int(toolErrMap["code"].(float64)) != mcp.CodeServerNotInitialized {
			t.Fatalf("expected code -32002, got: %v", toolErrMap["code"])
		}
	})

	// 4. Empty string protocolVersion rejected
	t.Run("empty_protocol_version_rejected", func(t *testing.T) {
		req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":""}}` + "\n"
		var out bytes.Buffer
		_ = mcp.Serve(strings.NewReader(req), &out)

		var resp mcperrors.JSONRPCResponse
		_ = json.Unmarshal(out.Bytes(), &resp)
		if resp.Error == nil {
			t.Fatal("expected error for empty protocolVersion")
		}
		errMap := resp.Error.(map[string]any)
		if int(errMap["code"].(float64)) != mcp.CodeInvalidParams {
			t.Fatalf("expected code -32602, got: %v", errMap["code"])
		}
	})

	// 5. Non-string protocolVersion rejected
	t.Run("non_string_protocol_version_rejected", func(t *testing.T) {
		req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":12345}}` + "\n"
		var out bytes.Buffer
		_ = mcp.Serve(strings.NewReader(req), &out)

		var resp mcperrors.JSONRPCResponse
		_ = json.Unmarshal(out.Bytes(), &resp)
		if resp.Error == nil {
			t.Fatal("expected error for non-string protocolVersion")
		}
		errMap := resp.Error.(map[string]any)
		if int(errMap["code"].(float64)) != mcp.CodeInvalidParams {
			t.Fatalf("expected code -32602, got: %v", errMap["code"])
		}
	})
}
