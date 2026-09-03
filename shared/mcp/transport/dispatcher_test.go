package transport

import (
	"bytes"
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcperrors "github.com/will2469/argus/shared/mcp/errors"
)

func TestDispatcher_ShutdownGraceful(t *testing.T) {
	var out bytes.Buffer
	disp := NewDispatcher(&out, 2, 10)

	var running atomic.Int32
	var completed atomic.Int32

	// Launch 5 fast tasks
	for i := 0; i < 5; i++ {
		req := ParsedRequest{ID: i}
		disp.Dispatch(req, CostCheap, func(ctx context.Context, pr ParsedRequest) *mcperrors.JSONRPCResponse {
			running.Add(1)
			time.Sleep(30 * time.Millisecond)
			completed.Add(1)
			return &mcperrors.JSONRPCResponse{JSONRPC: "2.0", ID: pr.ID, Result: "done"}
		})
	}

	disp.Shutdown(500 * time.Millisecond)

	if completed.Load() != 5 {
		t.Fatalf("expected all 5 tasks to complete during graceful shutdown, got %d", completed.Load())
	}
}

func TestDispatcher_WorkloadPartitioning(t *testing.T) {
	var out bytes.Buffer
	disp := NewDispatcher(&out, 1, 10) // 1 expensive, 10 cheap
	defer disp.Shutdown(DefaultShutdownTimeout)

	var expensiveRunning atomic.Bool
	var cheapExecutedWhileExpensiveActive atomic.Bool
	var wg sync.WaitGroup

	// Task A: Expensive task holding the expensive slot for 100ms
	wg.Add(1)
	disp.Dispatch(ParsedRequest{ID: "exp"}, CostExpensive, func(ctx context.Context, pr ParsedRequest) *mcperrors.JSONRPCResponse {
		defer wg.Done()
		expensiveRunning.Store(true)
		time.Sleep(100 * time.Millisecond)
		expensiveRunning.Store(false)
		return &mcperrors.JSONRPCResponse{JSONRPC: "2.0", ID: pr.ID, Result: "exp_done"}
	})

	time.Sleep(10 * time.Millisecond)

	// Task B: Cheap task should execute IMMEDIATELY even while Task A runs
	wg.Add(1)
	disp.Dispatch(ParsedRequest{ID: "cheap"}, CostCheap, func(ctx context.Context, pr ParsedRequest) *mcperrors.JSONRPCResponse {
		defer wg.Done()
		if expensiveRunning.Load() {
			cheapExecutedWhileExpensiveActive.Store(true)
		}
		return &mcperrors.JSONRPCResponse{JSONRPC: "2.0", ID: pr.ID, Result: "cheap_done"}
	})

	wg.Wait()

	if !cheapExecutedWhileExpensiveActive.Load() {
		t.Fatal("cheap request was queued behind expensive request: partitioning invariant failed")
	}
}
