package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/will2469/argus/shared/mcp"
	mcperrors "github.com/will2469/argus/shared/mcp/errors"
)

// TestGauntlet_JSONRPC runs the foundation gauntlet for all JSON-RPC protocol edge cases.
func TestGauntlet_JSONRPC(t *testing.T) {
	tests := []struct {
		name          string
		payload       string
		expectError   bool
		expectedCode  int
		expectEmpty   bool
		expectNullID  bool
		expectSuccess bool
	}{
		{
			name:         "wrong jsonrpc version 1.0",
			payload:      `{"jsonrpc":"1.0","id":1,"method":"ping"}` + "\n",
			expectError:  true,
			expectedCode: -32600,
		},
		{
			name:         "wrong jsonrpc version 3.0",
			payload:      `{"jsonrpc":"3.0","id":1,"method":"ping"}` + "\n",
			expectError:  true,
			expectedCode: -32600,
		},
		{
			name:         "missing method field",
			payload:      `{"jsonrpc":"2.0","id":1}` + "\n",
			expectError:  true,
			expectedCode: -32600,
		},
		{
			name:         "invalid id boolean",
			payload:      `{"jsonrpc":"2.0","id":true,"method":"ping"}` + "\n",
			expectError:  true,
			expectedCode: -32600,
		},
		{
			name:         "invalid id object",
			payload:      `{"jsonrpc":"2.0","id":{"foo":"bar"},"method":"ping"}` + "\n",
			expectError:  true,
			expectedCode: -32600,
		},
		{
			name:         "valid explicit null id",
			payload:      `{"jsonrpc":"2.0","id":null,"method":"ping"}` + "\n",
			expectNullID: true,
		},
		{
			name:        "notification without id produces zero output",
			payload:     `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n",
			expectEmpty: true,
		},
		{
			name:        "unknown notification produces zero output and does not crash",
			payload:     `{"jsonrpc":"2.0","method":"notifications/custom_unknown_event","params":{"data":123}}` + "\n",
			expectEmpty: true,
		},
		{
			name:         "malformed params primitive string",
			payload:      `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":"not_an_object"}` + "\n",
			expectError:  true,
			expectedCode: -32600,
		},
		{
			name:         "batch request rejected cleanly",
			payload:      `[{"jsonrpc":"2.0","id":1,"method":"ping"},{"jsonrpc":"2.0","id":2,"method":"ping"}]` + "\n",
			expectError:  true,
			expectedCode: -32600,
		},
		{
			name:         "duplicate fields in json rejected",
			payload:      `{"jsonrpc":"2.0","id":1,"method":"ping","method":"ping"}` + "\n",
			expectError:  true,
			expectedCode: -32600,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := mcp.Serve(strings.NewReader(tc.payload), &out)
			if err != nil {
				t.Fatalf("serve returned unexpected error: %v", err)
			}

			if tc.expectEmpty {
				if out.Len() != 0 {
					t.Fatalf("expected empty output for notification, got: %s", out.String())
				}
				return
			}

			var resp mcperrors.JSONRPCResponse
			if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse json-rpc response: %v, raw: %s", err, out.String())
			}

			if tc.expectNullID {
				if resp.ID != nil {
					t.Fatalf("expected null id, got: %v", resp.ID)
				}
				if resp.Error != nil {
					t.Fatalf("unexpected error: %v", resp.Error)
				}
				return
			}

			if tc.expectError {
				if resp.Error == nil {
					t.Fatalf("expected error code %d, but got success: %s", tc.expectedCode, out.String())
				}
				errMap, ok := resp.Error.(map[string]any)
				if !ok {
					t.Fatalf("expected error object, got: %T", resp.Error)
				}
				if int(errMap["code"].(float64)) != tc.expectedCode {
					t.Fatalf("expected error code %d, got: %v", tc.expectedCode, errMap["code"])
				}
				return
			}

			if tc.expectSuccess && resp.Error != nil {
				t.Fatalf("expected success, got error: %v", resp.Error)
			}
		})
	}
}

// TestGauntlet_Lifecycle verifies strict lifecycle state machine and EOF handling.
func TestGauntlet_Lifecycle(t *testing.T) {
	// 1. Call before initialize -> rejected with -32002
	t.Run("call_before_initialize_rejected", func(t *testing.T) {
		req := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
		var out bytes.Buffer
		_ = mcp.Serve(strings.NewReader(req), &out, mcp.WithStrictLifecycle(true))

		var resp mcperrors.JSONRPCResponse
		_ = json.Unmarshal(out.Bytes(), &resp)
		if resp.Error == nil {
			t.Fatal("expected rejection when calling tools/list before initialize")
		}
		errMap := resp.Error.(map[string]any)
		if int(errMap["code"].(float64)) != -32002 {
			t.Fatalf("expected code -32002, got: %v", errMap["code"])
		}
	})

	// 2. Full lifecycle: initialize -> initialized -> call -> success
	t.Run("full_lifecycle_success", func(t *testing.T) {
		dialogue := strings.Join([]string{
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
			`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
			`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		}, "\n") + "\n"

		var out bytes.Buffer
		_ = mcp.Serve(strings.NewReader(dialogue), &out, mcp.WithStrictLifecycle(true))

		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 responses (initialize, tools/list), got %d: %s", len(lines), out.String())
		}

		var respInit, respTools mcperrors.JSONRPCResponse
		_ = json.Unmarshal([]byte(lines[0]), &respInit)
		_ = json.Unmarshal([]byte(lines[1]), &respTools)

		if respInit.Error != nil || respTools.Error != nil {
			t.Fatalf("unexpected errors in full lifecycle: init=%v, tools=%v", respInit.Error, respTools.Error)
		}
	})

	// 3. Duplicate initialize -> rejected with -32600
	t.Run("duplicate_initialize_rejected", func(t *testing.T) {
		dialogue := strings.Join([]string{
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
			`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		}, "\n") + "\n"

		var out bytes.Buffer
		_ = mcp.Serve(strings.NewReader(dialogue), &out, mcp.WithStrictLifecycle(true))

		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 responses, got %d", len(lines))
		}

		var resp2 *mcperrors.JSONRPCResponse
		for _, line := range lines {
			var r mcperrors.JSONRPCResponse
			if err := json.Unmarshal([]byte(line), &r); err == nil && r.ID == float64(2) {
				resp2 = &r
				break
			}
		}
		if resp2 == nil {
			t.Fatalf("response with id=2 not found in lines: %v", lines)
		}
		if resp2.Error == nil {
			t.Fatal("expected duplicate initialize to be rejected")
		}
		errMap := resp2.Error.(map[string]any)
		if int(errMap["code"].(float64)) != mcp.CodeInvalidRequest {
			t.Fatalf("expected code %d, got %v", mcp.CodeInvalidRequest, errMap["code"])
		}
	})

	// 4. EOF during partial request -> cleanly terminates without panic
	t.Run("eof_during_partial_request", func(t *testing.T) {
		partialReq := `{"jsonrpc":"2.0","id":` // cut off mid-stream without newline
		var out bytes.Buffer
		err := mcp.Serve(strings.NewReader(partialReq), &out)
		if err != nil && err != io.EOF {
			t.Fatalf("unexpected error on partial EOF: %v", err)
		}
	})
}
