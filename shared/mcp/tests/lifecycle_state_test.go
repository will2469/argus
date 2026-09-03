package tests

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/will2469/argus/shared/mcp"
	mcperrors "github.com/will2469/argus/shared/mcp/errors"
)

// TestLifecycle_StateTransitions verifies the deterministic 3-state lifecycle:
// statePreInit -> stateInitializing -> stateInitialized
func TestLifecycle_StateTransitions(t *testing.T) {
	// 1. Calling normal operations during stateInitializing (before notifications/initialized) must be rejected (-32002)
	t.Run("call_during_initializing_rejected", func(t *testing.T) {
		dialogue := strings.Join([]string{
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
			`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		}, "\n") + "\n"

		var out bytes.Buffer
		_ = mcp.Serve(strings.NewReader(dialogue), &out, mcp.WithStrictLifecycle(true))

		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 responses, got %d: %s", len(lines), out.String())
		}

		var respInit, respTools mcperrors.JSONRPCResponse
		_ = json.Unmarshal([]byte(lines[0]), &respInit)
		_ = json.Unmarshal([]byte(lines[1]), &respTools)

		if respInit.Error != nil {
			t.Fatalf("expected initialize success, got error: %v", respInit.Error)
		}
		if respTools.Error == nil {
			t.Fatal("expected tools/list to be rejected during stateInitializing")
		}
		errMap := respTools.Error.(map[string]any)
		if int(errMap["code"].(float64)) != mcp.CodeServerNotInitialized {
			t.Fatalf("expected code %d (-32002), got: %v", mcp.CodeServerNotInitialized, errMap["code"])
		}
	})

	// 2. Ping is permitted during statePreInit
	t.Run("ping_permitted_during_pre_init", func(t *testing.T) {
		req := `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"

		var out bytes.Buffer
		_ = mcp.Serve(strings.NewReader(req), &out, mcp.WithStrictLifecycle(true))

		var resp mcperrors.JSONRPCResponse
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error for ping during pre_init: %v", resp.Error)
		}
	})

	// 3. Ping is permitted during stateInitializing
	t.Run("ping_permitted_during_initializing", func(t *testing.T) {
		dialogue := strings.Join([]string{
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
			`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		}, "\n") + "\n"

		var out bytes.Buffer
		_ = mcp.Serve(strings.NewReader(dialogue), &out, mcp.WithStrictLifecycle(true))

		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 responses, got %d: %s", len(lines), out.String())
		}

		var respInit, respPing mcperrors.JSONRPCResponse
		_ = json.Unmarshal([]byte(lines[0]), &respInit)
		_ = json.Unmarshal([]byte(lines[1]), &respPing)

		if respInit.Error != nil {
			t.Fatalf("expected initialize success, got error: %v", respInit.Error)
		}
		if respPing.Error != nil {
			t.Fatalf("expected ping success during initializing, got error: %v", respPing.Error)
		}
	})

	// 4a. notifications/initialized before initialize produces zero output (ignored notification)
	t.Run("initialized_before_initialize_ignored", func(t *testing.T) {
		req := `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"

		var out bytes.Buffer
		_ = mcp.Serve(strings.NewReader(req), &out, mcp.WithStrictLifecycle(true))

		if out.Len() != 0 {
			t.Fatalf("expected 0 bytes output for unsolicited notification, got: %s", out.String())
		}
	})

	// 4b. tools/list after fake notifications/initialized must still be rejected with -32002
	t.Run("tools_list_after_fake_initialized_rejected", func(t *testing.T) {
		dialogue := strings.Join([]string{
			`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
			`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		}, "\n") + "\n"

		var out bytes.Buffer
		_ = mcp.Serve(strings.NewReader(dialogue), &out, mcp.WithStrictLifecycle(true))

		var resp mcperrors.JSONRPCResponse
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected tools/list to be rejected even after premature notifications/initialized")
		}
		errMap := resp.Error.(map[string]any)
		if int(errMap["code"].(float64)) != mcp.CodeServerNotInitialized {
			t.Fatalf("expected code %d (-32002), got: %v", mcp.CodeServerNotInitialized, errMap["code"])
		}
		msg := errMap["message"].(string)
		if !strings.Contains(msg, "initialize") {
			t.Fatalf("expected message mentioning 'initialize', got: %s", msg)
		}
	})

	// 5. Duplicate initialize after stateInitialized must be rejected (-32600)
	t.Run("duplicate_initialize_after_initialized_rejected", func(t *testing.T) {
		dialogue := strings.Join([]string{
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
			`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
			`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		}, "\n") + "\n"

		var out bytes.Buffer
		_ = mcp.Serve(strings.NewReader(dialogue), &out, mcp.WithStrictLifecycle(true))

		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 responses, got %d: %s", len(lines), out.String())
		}

		var respInit, respDup mcperrors.JSONRPCResponse
		_ = json.Unmarshal([]byte(lines[0]), &respInit)
		_ = json.Unmarshal([]byte(lines[1]), &respDup)

		if respInit.Error != nil {
			t.Fatalf("expected first initialize success, got: %v", respInit.Error)
		}
		if respDup.Error == nil {
			t.Fatal("expected duplicate initialize to be rejected")
		}
		errMap := respDup.Error.(map[string]any)
		if int(errMap["code"].(float64)) != mcp.CodeInvalidRequest {
			t.Fatalf("expected code %d (-32600), got: %v", mcp.CodeInvalidRequest, errMap["code"])
		}
	})
}
