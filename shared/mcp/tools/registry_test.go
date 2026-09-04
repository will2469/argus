package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/will2469/argus/shared/mcp/errors"
	"github.com/will2469/argus/shared/mcp/security"
)

// TestRegistry_AllRegisteredTools_ExhaustivePolicyParity iterates through EVERY
// tool registered in DefaultRegistry and asserts that Dispatch structurally enforces
// both Tier-1 Schema validation and Tier-2 Policy validation before execution.
func TestRegistry_AllRegisteredTools_ExhaustivePolicyParity(t *testing.T) {
	defs := DefaultRegistry.ListDefs()
	if len(defs) == 0 {
		t.Fatal("expected DefaultRegistry to have registered tools")
	}

	// Policy violation payloads specific to each registered tool
	policyViolations := map[string]json.RawMessage{
		"argus_scan":            json.RawMessage(`{"dirs": ["../../etc"]}`),
		"argus_check_migration": json.RawMessage(`{"sql": "   "}`),
		"argus_explain_rule":    json.RawMessage(`{"rule_code": "  "}`),
	}

	for _, def := range defs {
		toolName := def.Name
		t.Run(toolName, func(t *testing.T) {
			ctx := context.Background()

			// 1. Structural Schema Invariant: Non-object payload MUST be rejected at Tier 1
			respNonObject := DefaultRegistry.Dispatch(ctx, toolName, 1, json.RawMessage(`["not_an_object"]`))
			assertErrorContains(t, respNonObject, "Schema violation")

			// 2. Strict Schema Invariant: Unexpected properties MUST be rejected at Tier 1
			respUnexpectedKey := DefaultRegistry.Dispatch(ctx, toolName, 2, json.RawMessage(`{"__rogue_unexpected_prop__": 999}`))
			assertErrorContains(t, respUnexpectedKey, "Schema violation")

			// 3. Policy Invariant: Policy-violating inputs MUST be rejected at Tier 2
			violationPayload, exists := policyViolations[toolName]
			if !exists {
				t.Fatalf("REGRESSION TRAP: Tool %q registered in DefaultRegistry has no policy violation test case! Every tool must have policy parity verification.", toolName)
			}

			respPolicyViolation := DefaultRegistry.Dispatch(ctx, toolName, 3, violationPayload)
			assertErrorContains(t, respPolicyViolation, "Policy violation")
		})
	}
}

// TestRegistry_Dispatch_StructuralPipelineStrictOrdering proves that Dispatch
// unconditionally enforces the sequence: Lookup -> Schema -> Policy -> Execute.
func TestRegistry_Dispatch_StructuralPipelineStrictOrdering(t *testing.T) {
	reg := NewRegistry()

	instTool := &instrumentedPipelineTool{
		name: "pipeline_test_tool",
	}
	reg.Register(instTool)

	ctx := context.Background()

	// Scenario A: Schema violation -> halts BEFORE Policy and Execute
	instTool.reset()
	respA := reg.Dispatch(ctx, "pipeline_test_tool", 1, json.RawMessage(`{"unexpected_field": true}`))
	assertErrorContains(t, respA, "Schema violation")
	if instTool.policyInvoked {
		t.Error("expected policy validation NOT to be invoked when schema validation fails")
	}
	if instTool.executeInvoked {
		t.Error("expected execute NOT to be invoked when schema validation fails")
	}

	// Scenario B: Policy violation -> halts BEFORE Execute
	instTool.reset()
	respB := reg.Dispatch(ctx, "pipeline_test_tool", 2, json.RawMessage(`{"deny": true}`))
	assertErrorContains(t, respB, "Policy violation")
	if !instTool.policyInvoked {
		t.Error("expected policy validation to be invoked when schema passes")
	}
	if instTool.executeInvoked {
		t.Error("expected execute NOT to be invoked when policy validation fails")
	}

	// Scenario C: Both pass -> Execute invoked
	instTool.reset()
	respC := reg.Dispatch(ctx, "pipeline_test_tool", 3, json.RawMessage(`{"deny": false}`))
	if respC.Error != nil {
		t.Fatalf("expected success, got error: %v", respC.Error)
	}
	if !instTool.policyInvoked {
		t.Error("expected policy validation to be invoked")
	}
	if !instTool.executeInvoked {
		t.Error("expected execute to be invoked when all validations pass")
	}
}

// TestRegistry_FutureTool_RegressionTrapDefense verifies that any newly added tool
// cannot execute without passing through Dispatch's mandatory policy gate.
func TestRegistry_FutureTool_RegressionTrapDefense(t *testing.T) {
	reg := NewRegistry()

	futureTool := &futureMockTool{
		name: "future_critical_tool",
	}
	reg.Register(futureTool)

	ctx := context.Background()

	// Malicious call attempting to perform an unauthorized operation
	badReq := json.RawMessage(`{"target": "sensitive_system", "force": true}`)
	resp := reg.Dispatch(ctx, "future_critical_tool", 42, badReq)

	assertErrorContains(t, resp, "Policy violation: force operation not authorized")
	if futureTool.executed {
		t.Fatal("SECURITY ESCAPE: future_critical_tool executed despite policy violation!")
	}
}

// TestRegistry_Register_ContractInvariants verifies that Register panics when
// a tool does not satisfy mandatory contract invariants.
func TestRegistry_Register_ContractInvariants(t *testing.T) {
	reg := NewRegistry()

	assertPanic(t, "nil tool", func() { reg.Register(nil) })
	assertPanic(t, "empty name", func() { reg.Register(&brokenMockTool{name: ""}) })
	assertPanic(t, "mismatched name", func() { reg.Register(&brokenMockTool{name: "tool_a", defName: "tool_b"}) })
	assertPanic(t, "non-object schema", func() { reg.Register(&brokenMockTool{name: "c", defName: "c", schemaType: "array"}) })
	assertPanic(t, "invalid cost", func() { reg.Register(&brokenMockTool{name: "d", defName: "d", schemaType: "object", cost: "BadCost"}) })
}

// Helper mock types

type instrumentedPipelineTool struct {
	name           string
	policyInvoked  bool
	executeInvoked bool
}

func (m *instrumentedPipelineTool) reset()       { m.policyInvoked = false; m.executeInvoked = false }
func (m *instrumentedPipelineTool) Name() string { return m.name }
func (m *instrumentedPipelineTool) Definition() ToolDef {
	return ToolDef{
		Name: m.name,
		InputSchema: security.Schema{
			Type:       "object",
			Properties: map[string]security.Property{"deny": {Type: "boolean"}},
		},
	}
}
func (m *instrumentedPipelineTool) Cost() ResourceCost { return CostCheap }
func (m *instrumentedPipelineTool) ValidatePolicy(raw json.RawMessage) error {
	m.policyInvoked = true
	var args struct {
		Deny bool `json:"deny"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return fmt.Errorf("invalid arguments: %w", err)
		}
	}
	if args.Deny {
		return fmt.Errorf("denied by pipeline test policy")
	}
	return nil
}
func (m *instrumentedPipelineTool) Execute(ctx context.Context, id any, raw json.RawMessage) *errors.JSONRPCResponse {
	m.executeInvoked = true
	return &errors.JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: map[string]any{"status": "ok"}}
}

type futureMockTool struct {
	name     string
	executed bool
}

func (f *futureMockTool) Name() string { return f.name }
func (f *futureMockTool) Definition() ToolDef {
	return ToolDef{
		Name: f.name,
		InputSchema: security.Schema{
			Type: "object",
			Properties: map[string]security.Property{
				"target": {Type: "string"},
				"force":  {Type: "boolean"},
			},
		},
	}
}
func (f *futureMockTool) Cost() ResourceCost { return CostExpensive }
func (f *futureMockTool) ValidatePolicy(raw json.RawMessage) error {
	var args struct {
		Force bool `json:"force"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Force {
		return fmt.Errorf("force operation not authorized")
	}
	return nil
}
func (f *futureMockTool) Execute(ctx context.Context, id any, raw json.RawMessage) *errors.JSONRPCResponse {
	f.executed = true
	return &errors.JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: map[string]any{"executed": true}}
}

type brokenMockTool struct {
	name, defName, schemaType string
	cost                      ResourceCost
}

func (b *brokenMockTool) Name() string { return b.name }
func (b *brokenMockTool) Definition() ToolDef {
	return ToolDef{Name: b.defName, InputSchema: security.Schema{Type: b.schemaType}}
}
func (b *brokenMockTool) Cost() ResourceCost                       { return b.cost }
func (b *brokenMockTool) ValidatePolicy(raw json.RawMessage) error { return nil }
func (b *brokenMockTool) Execute(ctx context.Context, id any, raw json.RawMessage) *errors.JSONRPCResponse {
	return nil
}

func assertErrorContains(t *testing.T, resp *errors.JSONRPCResponse, needle string) {
	t.Helper()
	if resp == nil || resp.Error == nil {
		t.Fatalf("expected error containing %q, got nil response or nil error", needle)
	}
	errStr := fmt.Sprintf("%v", resp.Error)
	if !strings.Contains(errStr, needle) {
		t.Fatalf("expected error to contain %q, got: %s", needle, errStr)
	}
}

func assertPanic(t *testing.T, desc string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for %s, but no panic occurred", desc)
		}
	}()
	fn()
}
