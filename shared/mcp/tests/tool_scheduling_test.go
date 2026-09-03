package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/will2469/argus/shared/mcp"
	mcperrors "github.com/will2469/argus/shared/mcp/errors"
	"github.com/will2469/argus/shared/mcp/security"
	"github.com/will2469/argus/shared/mcp/tools"
)

type blockingCheapTool struct {
	started atomic.Bool
}

func (b *blockingCheapTool) Name() string { return "blocking_cheap_tool" }
func (b *blockingCheapTool) Definition() tools.ToolDef {
	return tools.ToolDef{Name: "blocking_cheap_tool", InputSchema: security.Schema{Type: "object"}}
}
func (b *blockingCheapTool) ValidatePolicy(raw json.RawMessage) error { return nil }
func (b *blockingCheapTool) Cost() tools.ResourceCost                 { return tools.CostCheap }
func (b *blockingCheapTool) Execute(ctx context.Context, id any, raw json.RawMessage) *mcperrors.JSONRPCResponse {
	b.started.Store(true)
	time.Sleep(200 * time.Millisecond)
	return &mcperrors.JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: map[string]any{"ok": true}}
}

func TestToolScheduling_UnknownToolRejectedPreScheduling(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"phantom_nonexistent_tool","arguments":{}}}` + "\n"

	var out bytes.Buffer
	err := mcp.Serve(strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("serve returned error: %v", err)
	}

	var resp mcperrors.JSONRPCResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\nraw: %s", err, out.String())
	}

	if resp.Error == nil {
		t.Fatalf("expected error response for unknown tool, got result: %v", resp.Result)
	}

	errMap, ok := resp.Error.(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %T", resp.Error)
	}
	code := int(errMap["code"].(float64))
	if code != -32602 {
		t.Errorf("expected code -32602 (InvalidParams), got %d", code)
	}
	msg, _ := errMap["message"].(string)
	if !strings.Contains(msg, "Unknown tool: phantom_nonexistent_tool") {
		t.Errorf("expected message to mention unknown tool, got: %q", msg)
	}
}

func TestToolScheduling_MissingNameRejectedPreScheduling(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{
			name:    "empty params object",
			payload: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{}}` + "\n",
		},
		{
			name:    "params without name field",
			payload: `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"arguments":{}}}` + "\n",
		},
		{
			name:    "params with empty name string",
			payload: `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":""}}` + "\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := mcp.Serve(strings.NewReader(tc.payload), &out)
			if err != nil {
				t.Fatalf("serve returned error: %v", err)
			}

			var resp mcperrors.JSONRPCResponse
			if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v\nraw: %s", err, out.String())
			}

			if resp.Error == nil {
				t.Fatalf("expected error for missing name, got result: %v", resp.Result)
			}
			errMap, ok := resp.Error.(map[string]any)
			if !ok {
				t.Fatalf("expected error object, got %T", resp.Error)
			}
			code := int(errMap["code"].(float64))
			if code != -32602 {
				t.Errorf("expected code -32602 (InvalidParams), got %d", code)
			}
		})
	}
}

func TestToolScheduling_UnknownToolBypassesWorkerPool(t *testing.T) {
	tool := &blockingCheapTool{}
	tools.RegisterTool(tool)
	defer tools.UnregisterTool(tool.Name())

	pr, pw := io.Pipe()
	var outBuf bytes.Buffer
	serverDone := make(chan error, 1)

	go func() {
		serverDone <- mcp.Serve(pr, &outBuf)
	}()

	// 1. Send Request A: Valid, slow cheap tool call (200ms)
	reqA := `{"jsonrpc":"2.0","id":"req-A-slow","method":"tools/call","params":{"name":"blocking_cheap_tool","arguments":{}}}` + "\n"
	_, _ = pw.Write([]byte(reqA))

	// Give a small pause to let reqA be read and dispatched
	time.Sleep(10 * time.Millisecond)

	// 2. Send Request B: Unknown tool call.
	// Since pre-scheduling rejects it immediately before queueing or worker dispatch,
	// it should be processed and written synchronously.
	reqB := `{"jsonrpc":"2.0","id":"req-B-unknown","method":"tools/call","params":{"name":"fake_tool_999"}}` + "\n"
	_, _ = pw.Write([]byte(reqB))

	// Wait briefly for synchronous response to be flushed to pipe
	time.Sleep(20 * time.Millisecond)

	_ = pw.Close()
	if err := <-serverDone; err != nil {
		t.Fatalf("serve returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 responses, got %d:\n%s", len(lines), outBuf.String())
	}

	// req-B-unknown was rejected synchronously at the boundary, so it must appear
	// in the output BEFORE req-A-slow completes its 200ms sleep!
	if !strings.Contains(lines[0], "req-B-unknown") {
		t.Fatalf("expected req-B-unknown to complete first via pre-scheduling rejection, got order:\n1: %s\n2: %s", lines[0], lines[1])
	}
	if !strings.Contains(lines[0], "Unknown tool: fake_tool_999") {
		t.Fatalf("expected line 0 to reject unknown tool, got: %s", lines[0])
	}
}
