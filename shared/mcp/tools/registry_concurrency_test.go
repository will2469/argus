package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/will2469/argus/shared/mcp/errors"
	"github.com/will2469/argus/shared/mcp/security"
)

type concurrentMockTool struct {
	name string
}

func (c *concurrentMockTool) Name() string {
	return c.name
}

func (c *concurrentMockTool) Definition() ToolDef {
	return ToolDef{
		Name:        c.name,
		Description: "concurrent mock tool",
		InputSchema: security.Schema{Type: "object"},
	}
}

func (c *concurrentMockTool) ValidatePolicy(args json.RawMessage) error {
	return nil
}

func (c *concurrentMockTool) Execute(ctx context.Context, id any, args json.RawMessage) *errors.JSONRPCResponse {
	return errors.ToolSuccess(id, "ok")
}

func (c *concurrentMockTool) Cost() ResourceCost {
	return CostCheap
}

// TestRegistry_ConcurrentAccess validates thread safety (RWMutex) against data races.
func TestRegistry_ConcurrentAccess(t *testing.T) {
	reg := NewRegistry()
	var wg sync.WaitGroup
	workers := 16
	iterations := 100

	// Initial seeds
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("seed_%d", i)
		reg.Register(&concurrentMockTool{name: name})
	}

	// 1. Concurrent Readers (Get, GetCost, ListDefs, Dispatch)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = reg.ListDefs()
				toolName := fmt.Sprintf("seed_%d", i%5)
				_, _ = reg.Get(toolName)
				_, _ = reg.GetCost(toolName)
				_ = reg.Dispatch(context.Background(), toolName, i, json.RawMessage(`{}`))
			}
		}(w)
	}

	// 2. Concurrent Writers (Register, Unregister)
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				dynName := fmt.Sprintf("dyn_%d_%d", workerID, i)
				reg.Register(&concurrentMockTool{name: dynName})
				_, _ = reg.Get(dynName)
				reg.Unregister(dynName)
			}
		}(w)
	}

	wg.Wait()
}
