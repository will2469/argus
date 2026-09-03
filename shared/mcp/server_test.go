package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestInitialize(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2026-07-28","capabilities":{},"clientInfo":{"name":"test","version":"0.1.0"}}}` + "\n"

	var out bytes.Buffer
	err := serve(strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("serve returned error: %v", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\nraw: %s", err, out.String())
	}

	if resp.ID != float64(1) {
		t.Fatalf("expected id=1, got %v", resp.ID)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map, got %T", resp.Result)
	}
	if result["protocolVersion"] != LatestProtocolVersion {
		t.Fatalf("expected protocolVersion=%s, got %v", LatestProtocolVersion, result["protocolVersion"])
	}
}

func TestToolsList(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"

	var out bytes.Buffer
	err := serve(strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("serve returned error: %v", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected result to be map, got %T", resp.Result)
	}

	toolsRaw, ok := result["tools"]
	if !ok {
		t.Fatal("missing 'tools' key in result")
	}

	toolsList, ok := toolsRaw.([]any)
	if !ok {
		t.Fatalf("expected tools to be array, got %T", toolsRaw)
	}

	if len(toolsList) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(toolsList))
	}

	expectedNames := map[string]bool{
		"argus_scan":            false,
		"argus_check_migration": false,
		"argus_explain_rule":    false,
		"argus_report_issue":    false,
	}
	for _, toolRaw := range toolsList {
		tool, ok := toolRaw.(map[string]any)
		if !ok {
			t.Fatalf("expected tool to be map, got %T", toolRaw)
		}
		name, _ := tool["name"].(string)
		if _, exists := expectedNames[name]; !exists {
			t.Fatalf("unexpected tool name: %s", name)
		}
		expectedNames[name] = true
	}
	for name, found := range expectedNames {
		if !found {
			t.Fatalf("tool %s not found in response", name)
		}
	}
}

func TestPing(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":3,"method":"ping"}` + "\n"

	var out bytes.Buffer
	err := serve(strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("serve returned error: %v", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error on ping: %v", resp.Error)
	}
}

func TestExplainRuleValid(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"argus_explain_rule","arguments":{"rule_code":"A17"}}}` + "\n"

	var out bytes.Buffer
	err := serve(strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("serve returned error: %v", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected result map, got %T", resp.Result)
	}
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatal("expected non-empty content")
	}
	item, _ := content[0].(map[string]any)
	text, _ := item["text"].(string)
	if !strings.Contains(text, "FORBIDDEN_QUERY_IN_LOOP") {
		t.Fatalf("expected A17 description in response, got: %s", text)
	}
}

func TestExplainRuleInvalid(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"argus_explain_rule","arguments":{"rule_code":"A99"}}}` + "\n"

	var out bytes.Buffer
	err := serve(strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("serve returned error: %v", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected result map, got %T", resp.Result)
	}
	isError, _ := result["isError"].(bool)
	if !isError {
		t.Fatal("expected isError=true for unknown rule")
	}
}

func TestUnknownMethod(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":6,"method":"nonexistent/method"}` + "\n"

	var out bytes.Buffer
	err := serve(strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("serve returned error: %v", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
}

func TestNotificationNoResponse(t *testing.T) {
	req := `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"

	var out bytes.Buffer
	err := serve(strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("serve returned error: %v", err)
	}

	if out.Len() != 0 {
		t.Fatalf("expected no response for notification, got: %s", out.String())
	}
}
