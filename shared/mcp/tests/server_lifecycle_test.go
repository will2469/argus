package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/will2469/argus/shared/mcp"
	mcperrors "github.com/will2469/argus/shared/mcp/errors"
	"github.com/will2469/argus/shared/mcp/security"
	"github.com/will2469/argus/shared/mcp/tools"
)

type sleepTool struct{}

func (s *sleepTool) Name() string                                              { return "sleep_tool" }
func (s *sleepTool) Definition() tools.ToolDef                                { return tools.ToolDef{Name: "sleep_tool", InputSchema: security.Schema{Type: "object"}} }
func (s *sleepTool) ValidatePolicy(raw json.RawMessage) error                  { return nil }
func (s *sleepTool) Cost() tools.ResourceCost                                  { return tools.CostExpensive }
func (s *sleepTool) Execute(ctx context.Context, id any, raw json.RawMessage) *mcperrors.JSONRPCResponse {
	time.Sleep(150 * time.Millisecond)
	return &mcperrors.JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: map[string]any{"status": "sleep_done"}}
}

func TestServe_LifecycleIsolation_NonBlocking(t *testing.T) {
	tool := &sleepTool{}
	tools.RegisterTool(tool)
	defer tools.UnregisterTool(tool.Name())

	pr, pw := io.Pipe()
	var outBuf bytes.Buffer
	serverDone := make(chan error, 1)

	go func() {
		serverDone <- mcp.Serve(pr, &outBuf)
	}()

	// 1. Send Request A: Slow execution (150ms)
	reqA := `{"jsonrpc":"2.0","id":"req-A-slow","method":"tools/call","params":{"name":"sleep_tool","arguments":{}}}` + "\n"
	_, _ = pw.Write([]byte(reqA))

	// Small pause to guarantee reqA enters execution first
	time.Sleep(20 * time.Millisecond)

	// 2. Send Request B: Fast ping
	reqB := `{"jsonrpc":"2.0","id":"req-B-fast-ping","method":"ping"}` + "\n"
	_, _ = pw.Write([]byte(reqB))

	// Close write pipe
	_ = pw.Close()

	if err := <-serverDone; err != nil {
		t.Fatalf("serve failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d:\n%s", len(lines), outBuf.String())
	}

	var firstResp, secondResp mcperrors.JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[0]), &firstResp)
	_ = json.Unmarshal([]byte(lines[1]), &secondResp)

	if firstResp.ID != "req-B-fast-ping" {
		t.Errorf("head-of-line blocking detected: ping should finish first, but got: %v", firstResp.ID)
	}
	if secondResp.ID != "req-A-slow" {
		t.Errorf("slow request should finish second, but got: %v", secondResp.ID)
	}
}

type slowCancelTool struct {
	started chan struct{}
}

func (s *slowCancelTool) Name() string                                              { return "slow_cancel_tool" }
func (s *slowCancelTool) Definition() tools.ToolDef                                { return tools.ToolDef{Name: "slow_cancel_tool", Description: "test tool that blocks until cancelled", InputSchema: security.Schema{Type: "object"}} }
func (s *slowCancelTool) ValidatePolicy(raw json.RawMessage) error                  { return nil }
func (s *slowCancelTool) Cost() tools.ResourceCost                                  { return tools.CostCheap }
func (s *slowCancelTool) Execute(ctx context.Context, id any, raw json.RawMessage) *mcperrors.JSONRPCResponse {
	close(s.started)
	<-ctx.Done()
	return &mcperrors.JSONRPCResponse{JSONRPC: "2.0", ID: id, Error: mcperrors.JSONRPCError{Code: mcp.CodeCancelled, Message: "Request cancelled by client"}}
}

func TestServe_DeterministicCancellation(t *testing.T) {
	tool := &slowCancelTool{started: make(chan struct{})}
	tools.RegisterTool(tool)
	defer tools.UnregisterTool(tool.Name())

	pr, pw := io.Pipe()
	var outBuf bytes.Buffer
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- mcp.Serve(pr, &outBuf)
	}()

	// 1. Send request
	req := `{"jsonrpc":"2.0","id":777,"method":"tools/call","params":{"name":"slow_cancel_tool","arguments":{}}}` + "\n"
	_, _ = pw.Write([]byte(req))

	// 2. Wait until the tool starts executing
	select {
	case <-tool.started:
	case <-time.After(1 * time.Second):
		t.Fatal("tool failed to start")
	}

	// 3. Send cancellation notification
	cancelNotif := `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":777}}` + "\n"
	_, _ = pw.Write([]byte(cancelNotif))

	_ = pw.Close()

	if err := <-serverDone; err != nil {
		t.Fatalf("serve error: %v", err)
	}

	var resp mcperrors.JSONRPCResponse
	if err := json.Unmarshal(outBuf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse cancelled response: %v, raw: %s", err, outBuf.String())
	}

	if resp.ID != float64(777) {
		t.Errorf("expected id=777, got %v", resp.ID)
	}
}

func TestServeBoundaryRejections(t *testing.T) {
	inputs := []struct {
		name         string
		rawLine      string
		expectedCode int
		expectedID   any
	}{
		{
			name:         "missing jsonrpc rejected at serve boundary",
			rawLine:      `{"id":1,"method":"ping"}` + "\n",
			expectedCode: -32600,
			expectedID:   float64(1),
		},
		{
			name:         "jsonrpc 1.0 rejected at serve boundary",
			rawLine:      `{"jsonrpc":"1.0","id":2,"method":"ping"}` + "\n",
			expectedCode: -32600,
			expectedID:   float64(2),
		},
		{
			name:         "illegal boolean id rejected at serve boundary",
			rawLine:      `{"jsonrpc":"2.0","id":true,"method":"ping"}` + "\n",
			expectedCode: -32600,
			expectedID:   nil,
		},
	}

	for _, tc := range inputs {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := mcp.Serve(strings.NewReader(tc.rawLine), &out)
			if err != nil {
				t.Fatalf("unexpected serve error: %v", err)
			}

			var resp mcperrors.JSONRPCResponse
			if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v\nraw: %s", err, out.String())
			}

			errMap, ok := resp.Error.(map[string]any)
			if !ok {
				t.Fatalf("expected error map in response, got %T", resp.Error)
			}
			code := int(errMap["code"].(float64))
			if code != tc.expectedCode {
				t.Errorf("expected code %d, got %d", tc.expectedCode, code)
			}
			if resp.ID != tc.expectedID {
				t.Errorf("expected id %v, got %v", tc.expectedID, resp.ID)
			}
		})
	}
}

func TestServe_ToolCallBoundaryEnforcement(t *testing.T) {
	tests := []struct {
		name        string
		rawReq      string
		expectedMsg string
	}{
		{
			name:        "path traversal blocked at tools/call boundary",
			rawReq:      `{"jsonrpc":"2.0","id":100,"method":"tools/call","params":{"name":"argus_scan","arguments":{"dirs":["../../outside"]}}}` + "\n",
			expectedMsg: "Policy violation: path authority violation",
		},
		{
			name:        "hallucinated argument blocked at tools/call boundary",
			rawReq:      `{"jsonrpc":"2.0","id":101,"method":"tools/call","params":{"name":"argus_check_migration","arguments":{"sql":"SELECT 1","fake_flag":true}}}` + "\n",
			expectedMsg: `Schema violation: unexpected property "fake_flag"`,
		},
		{
			name:        "type mismatch blocked at tools/call boundary",
			rawReq:      `{"jsonrpc":"2.0","id":102,"method":"tools/call","params":{"name":"argus_check_migration","arguments":{"sql":12345}}}` + "\n",
			expectedMsg: `Schema violation: property "sql" must be a string`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := mcp.Serve(strings.NewReader(tc.rawReq), &out)
			if err != nil {
				t.Fatalf("serve returned error: %v", err)
			}

			var resp mcperrors.JSONRPCResponse
			if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v\nraw: %s", err, out.String())
			}

			if resp.Error == nil {
				t.Fatalf("expected error in response, got success: %v", resp.Result)
			}

			errMap, ok := resp.Error.(map[string]any)
			if !ok {
				t.Fatalf("expected error map, got %T", resp.Error)
			}

			code := int(errMap["code"].(float64))
			if code != -32602 {
				t.Errorf("expected code -32602, got %d", code)
			}

			msg, _ := errMap["message"].(string)
			if !strings.Contains(msg, tc.expectedMsg) {
				t.Errorf("expected message containing %q, got: %q", tc.expectedMsg, msg)
			}
		})
	}
}
