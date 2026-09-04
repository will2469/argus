package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/will2469/argus/shared/mcp/errors"
	"github.com/will2469/argus/shared/mcp/security"
)

// NewRegistry initializes an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry. It enforces strict structural contract
// invariants (non-nil, non-empty matching name, object schema, valid cost)
// so that rogue or malformed tools cannot enter the dispatch lifecycle.
func (r *Registry) Register(t Tool) {
	if t == nil {
		panic("mcp: cannot register nil tool")
	}
	name := strings.TrimSpace(t.Name())
	if name == "" {
		panic("mcp: tool name cannot be empty")
	}
	def := t.Definition()
	if def.Name != name {
		panic(fmt.Sprintf("mcp: tool definition name %q does not match Name() %q", def.Name, name))
	}
	if def.InputSchema.Type != "object" {
		panic(fmt.Sprintf("mcp: tool %q input schema type must be \"object\", got %q", name, def.InputSchema.Type))
	}
	cost := t.Cost()
	if cost != CostCheap && cost != CostExpensive {
		panic(fmt.Sprintf("mcp: tool %q has invalid cost classification: %v", name, cost))
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, name)
	}
	r.tools[name] = t
}

// Unregister removes a tool from the registry.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
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
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// GetCost returns the declared resource cost of a tool and whether the tool was found in the registry.
func (r *Registry) GetCost(name string) (ResourceCost, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if tool, ok := r.tools[name]; ok {
		return tool.Cost(), true
	}
	return CostCheap, false
}

// ListDefs returns definitions of all registered tools in deterministic order.
func (r *Registry) ListDefs() []ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]ToolDef, 0, len(r.order))
	for _, name := range r.order {
		defs = append(defs, r.tools[name].Definition())
	}
	return defs
}

// Dispatch coordinates Tier-1 Schema validation, Tier-2 Policy validation,
// and tool execution for incoming tool calls with cancellation context support.
func (r *Registry) Dispatch(ctx context.Context, name string, id any, args json.RawMessage) *errors.JSONRPCResponse {
	r.mu.RLock()
	tool, exists := r.tools[name]
	r.mu.RUnlock()
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

// RegisterTool registers a tool into the default registry.
func RegisterTool(t Tool) {
	DefaultRegistry.Register(t)
}

// UnregisterTool removes a tool from the default registry.
func UnregisterTool(name string) {
	DefaultRegistry.Unregister(name)
}
