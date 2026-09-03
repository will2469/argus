package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/will2469/argus/shared/mcp"
	mcperrors "github.com/will2469/argus/shared/mcp/errors"
	"github.com/will2469/argus/shared/mcp/security"
	"github.com/will2469/argus/shared/mcp/tools"
)

type dualBlockTool struct {
	mu      sync.Mutex
	started map[string]chan struct{}
}

func newDualBlockTool() *dualBlockTool {
	return &dualBlockTool{
		started: map[string]chan struct{}{
			"n:1": make(chan struct{}),
			"s:1": make(chan struct{}),
		},
	}
}

func (t *dualBlockTool) Name() string { return "dual_block_tool" }
func (t *dualBlockTool) Definition() tools.ToolDef {
	return tools.ToolDef{Name: "dual_block_tool", InputSchema: security.Schema{Type: "object"}}
}
func (t *dualBlockTool) ValidatePolicy(raw json.RawMessage) error { return nil }
func (t *dualBlockTool) Cost() tools.ResourceCost                 { return tools.CostCheap }

func (t *dualBlockTool) Execute(ctx context.Context, id any, raw json.RawMessage) *mcperrors.JSONRPCResponse {
	key := mcp.IDKey(id)
	t.mu.Lock()
	ch, ok := t.started[key]
	t.mu.Unlock()
	if ok {
		close(ch)
	}

	<-ctx.Done()
	return mcperrors.CancelledError(id, "Cancelled")
}

// TestServe_Cancellation_AdversarialIDCollision verifies that integer ID 1 and string ID "1"
// maintain independent cancellation lifecycles over the JSON-RPC stdio transport without collision.
func TestServe_Cancellation_AdversarialIDCollision(t *testing.T) {
	tool := newDualBlockTool()
	tools.RegisterTool(tool)
	defer tools.UnregisterTool(tool.Name())

	pr, pw := io.Pipe()
	var outBuf bytes.Buffer
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- mcp.Serve(pr, &outBuf)
	}()

	// 1. Launch Request 1 with integer id: 1
	req1 := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"dual_block_tool","arguments":{}}}` + "\n"
	_, _ = pw.Write([]byte(req1))

	select {
	case <-tool.started["n:1"]:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for request 1 (int) to start")
	}

	// 2. Launch Request 2 with string id: "1"
	req2 := `{"jsonrpc":"2.0","id":"1","method":"tools/call","params":{"name":"dual_block_tool","arguments":{}}}` + "\n"
	_, _ = pw.Write([]byte(req2))

	select {
	case <-tool.started["s:1"]:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for request 2 (string) to start")
	}

	// 3. Cancel ONLY integer id 1
	cancelReq1 := `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":1}}` + "\n"
	_, _ = pw.Write([]byte(cancelReq1))

	// Small wait to allow request 1 cancellation to write its response
	time.Sleep(50 * time.Millisecond)

	// 4. Cancel string id "1"
	cancelReq2 := `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":"1"}}` + "\n"
	_, _ = pw.Write([]byte(cancelReq2))

	_ = pw.Close()

	if err := <-serverDone; err != nil {
		t.Fatalf("serve failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d:\n%s", len(lines), outBuf.String())
	}

	var resp1, resp2 mcperrors.JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[0]), &resp1)
	_ = json.Unmarshal([]byte(lines[1]), &resp2)

	// First response must correspond to integer 1 (cancelled first)
	if resp1.ID != float64(1) {
		t.Errorf("expected first cancelled response to have id=1 (float64), got %v", resp1.ID)
	}
	// Second response must correspond to string "1" (cancelled second)
	if resp2.ID != "1" {
		t.Errorf("expected second cancelled response to have id='1' (string), got %v", resp2.ID)
	}
}
