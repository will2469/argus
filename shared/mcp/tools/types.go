package tools

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/will2469/argus/shared/mcp/errors"
	"github.com/will2469/argus/shared/mcp/security"
	"github.com/will2469/argus/shared/mcp/transport"
)

// ResourceCost classifies the computational weight of an operation to prevent Head-of-Line blocking.
type ResourceCost = transport.ResourceCost

const (
	// CostCheap is for low-latency in-memory operations (ping, rules lookup, tools/list).
	CostCheap = transport.CostCheap
	// CostExpensive is for heavy CPU/memory operations (repository AST scans, migration parses).
	CostExpensive = transport.CostExpensive
)

// ToolDef describes an MCP tool definition advertised to clients.
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema security.Schema `json:"inputSchema"`
}

// Tool defines the modular interface that every MCP tool must satisfy.
type Tool interface {
	Name() string
	Definition() ToolDef
	ValidatePolicy(args json.RawMessage) error
	Execute(ctx context.Context, id any, args json.RawMessage) *errors.JSONRPCResponse
	Cost() ResourceCost
}

// Registry manages the collection of available MCP tools and handles multi-tier validation dispatching.
// It is protected by sync.RWMutex to prevent data races during dynamic registration or hot-reloads.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	order []string
}

// DefaultRegistry is the shared tool registry for the Argus MCP server.
var DefaultRegistry = NewRegistry()

// Concrete Tool Implementations

type checkMigrationTool struct{}
type explainRuleTool struct{}
