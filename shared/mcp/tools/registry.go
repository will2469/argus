package tools

import (
	"context"
	"encoding/json"
	"fmt"

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

// Registry manages the collection of available MCP tools and handles two-tier validation dispatching.
type Registry struct {
	tools map[string]Tool
	order []string
}

// NewRegistry initializes an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	name := t.Name()
	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, name)
	}
	r.tools[name] = t
}

// Unregister removes a tool from the registry.
func (r *Registry) Unregister(name string) {
	delete(r.tools, name)
	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
}

// Get retrieves a registered tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// GetCost returns the declared resource cost of a tool, defaulting to CostCheap if unknown.
func (r *Registry) GetCost(name string) ResourceCost {
	if tool, ok := r.tools[name]; ok {
		return tool.Cost()
	}
	return CostCheap
}

// ListDefs returns definitions of all registered tools in deterministic order.
func (r *Registry) ListDefs() []ToolDef {
	defs := make([]ToolDef, 0, len(r.order))
	for _, name := range r.order {
		defs = append(defs, r.tools[name].Definition())
	}
	return defs
}

// Dispatch coordinates Tier-1 Schema validation, Tier-2 Policy validation,
// and tool execution for incoming tool calls with cancellation context support.
func (r *Registry) Dispatch(ctx context.Context, name string, id any, args json.RawMessage) *errors.JSONRPCResponse {
	tool, exists := r.tools[name]
	if !exists {
		return errors.InvalidParamsError(id, fmt.Sprintf("Unknown tool: %s", name))
	}

	def := tool.Definition()

	// Tier 1: Runtime Schema Validation
	if err := security.ValidateSchema(name, def.InputSchema, args); err != nil {
		return errors.InvalidParamsError(id, fmt.Sprintf("Schema violation: %v", err))
	}

	// Tier 2: Policy & Resource Sandbox Validation
	if err := tool.ValidatePolicy(args); err != nil {
		return errors.InvalidParamsError(id, fmt.Sprintf("Policy violation: %v", err))
	}

	// Tier 3: Execution with cancellation context
	return tool.Execute(ctx, id, args)
}

// DefaultRegistry is the shared tool registry for the Argus MCP server.
var DefaultRegistry = NewRegistry()

// RegisterTool registers a tool into the default registry.
func RegisterTool(t Tool) {
	DefaultRegistry.Register(t)
}

// UnregisterTool removes a tool from the default registry.
func UnregisterTool(name string) {
	DefaultRegistry.Unregister(name)
}
