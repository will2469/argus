package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestReportIssueTelemetryDisabled(t *testing.T) {
	t.Setenv("ARGUS_TELEMETRY", "false")

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"argus_report_issue","arguments":{"title":"test","description":"test","confirm":true}}}` + "\n"

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
		t.Fatal("expected isError=true when telemetry is disabled")
	}

	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatal("expected content")
	}
	item, _ := content[0].(map[string]any)
	text, _ := item["text"].(string)
	if !strings.Contains(text, "telemetry: false") {
		t.Fatalf("expected telemetry disabled message, got: %s", text)
	}
}

func TestReportIssueTelemetryEnabledPreview(t *testing.T) {
	t.Setenv("ARGUS_TELEMETRY", "true")

	req := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"argus_report_issue","arguments":{"rule_code":"A14","title":"Preview test","description":"Testing preview mode","confirm":false}}}` + "\n"

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

	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatal("expected content")
	}
	item, _ := content[0].(map[string]any)
	text, _ := item["text"].(string)
	if !strings.Contains(text, "Issue Draft Preview") {
		t.Fatalf("expected preview message, got: %s", text)
	}
}
