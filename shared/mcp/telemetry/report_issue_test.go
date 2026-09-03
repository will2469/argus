package telemetry

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestReportIssueTelemetryDisabled(t *testing.T) {
	t.Setenv("ARGUS_TELEMETRY", "false")

	resp := HandleReportIssue(1, json.RawMessage(`{"title":"test","description":"test"}`))
	data, _ := json.Marshal(resp)
	var parsed struct {
		Result map[string]any `json:"result"`
	}
	_ = json.Unmarshal(data, &parsed)

	isError, _ := parsed.Result["isError"].(bool)
	if !isError {
		t.Fatal("expected isError=true when telemetry is disabled")
	}

	content := parsed.Result["content"].([]any)
	item := content[0].(map[string]any)
	text := item["text"].(string)
	if !strings.Contains(text, "telemetry: false") {
		t.Fatalf("expected telemetry disabled message, got: %s", text)
	}
}

func TestReportIssueTelemetryEnabledPreview(t *testing.T) {
	t.Setenv("ARGUS_TELEMETRY", "true")

	resp := HandleReportIssue(2, json.RawMessage(`{"rule_code":"A14","title":"Preview test","description":"Testing preview mode"}`))
	data, _ := json.Marshal(resp)
	var parsed struct {
		Result map[string]any `json:"result"`
	}
	_ = json.Unmarshal(data, &parsed)

	content := parsed.Result["content"].([]any)
	item := content[0].(map[string]any)
	text := item["text"].(string)
	if !strings.Contains(text, "Issue Draft Preview") {
		t.Fatalf("expected preview message, got: %s", text)
	}
	if !strings.Contains(text, "Approval Token:") {
		t.Fatalf("expected approval token in preview, got: %s", text)
	}
}

func TestReportIssue_CryptographicApprovalAndAntiMutation(t *testing.T) {
	t.Setenv("ARGUS_TELEMETRY", "true")

	resp1 := HandleReportIssue(10, json.RawMessage(`{"rule_code":"A14","title":"Original Title","description":"Original Description"}`))
	data1, _ := json.Marshal(resp1)
	var parsed1 struct {
		Result map[string]any `json:"result"`
	}
	_ = json.Unmarshal(data1, &parsed1)
	text1 := parsed1.Result["content"].([]any)[0].(map[string]any)["text"].(string)

	re := regexp.MustCompile(`appr_[a-f0-9]+`)
	token := re.FindString(text1)
	if token == "" {
		t.Fatalf("failed to find approval token in preview text: %s", text1)
	}

	mutatedReq := `{"rule_code":"A14","title":"Original Title","description":"MUTATED SPAM OR EXFILTRATION","approval_token":"` + token + `"}`
	resp2 := HandleReportIssue(11, json.RawMessage(mutatedReq))
	data2, _ := json.Marshal(resp2)
	var parsed2 struct {
		Result map[string]any `json:"result"`
	}
	_ = json.Unmarshal(data2, &parsed2)

	if isErr, _ := parsed2.Result["isError"].(bool); !isErr {
		t.Fatal("CRITICAL: expected isError: true when payload is mutated, but submission proceeded!")
	}
	text2 := parsed2.Result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text2, "payload mutation detected") {
		t.Fatalf("expected payload mutation rejection, got: %s", text2)
	}

	replayReq := `{"rule_code":"A14","title":"Original Title","description":"Original Description","approval_token":"` + token + `"}`
	resp3 := HandleReportIssue(12, json.RawMessage(replayReq))
	data3, _ := json.Marshal(resp3)
	var parsed3 struct {
		Result map[string]any `json:"result"`
	}
	_ = json.Unmarshal(data3, &parsed3)

	if isErr, _ := parsed3.Result["isError"].(bool); !isErr {
		t.Fatal("expected isError: true for already consumed token")
	}
	text3 := parsed3.Result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text3, "already consumed") {
		t.Fatalf("expected already consumed rejection, got: %s", text3)
	}
}

func TestReportIssue_FallbackURLSemanticStatus(t *testing.T) {
	t.Setenv("ARGUS_TELEMETRY", "true")
	t.Setenv("PATH", "")

	resp1 := HandleReportIssue(20, json.RawMessage(`{"rule_code":"A14","title":"Fallback Test","description":"Test fallback semantics"}`))
	data1, _ := json.Marshal(resp1)
	var parsed1 struct {
		Result map[string]any `json:"result"`
	}
	_ = json.Unmarshal(data1, &parsed1)
	text1 := parsed1.Result["content"].([]any)[0].(map[string]any)["text"].(string)

	re := regexp.MustCompile(`appr_[a-f0-9]+`)
	token := re.FindString(text1)
	if token == "" {
		t.Fatalf("expected token in %s", text1)
	}

	submitReq := `{"rule_code":"A14","title":"Fallback Test","description":"Test fallback semantics","approval_token":"` + token + `"}`
	resp2 := HandleReportIssue(21, json.RawMessage(submitReq))
	data2, _ := json.Marshal(resp2)
	var parsed2 struct {
		Result map[string]any `json:"result"`
	}
	_ = json.Unmarshal(data2, &parsed2)
	text2 := parsed2.Result["content"].([]any)[0].(map[string]any)["text"].(string)

	if !strings.Contains(text2, "STATUS: READY_FOR_SUBMISSION (NOT YET CREATED)") {
		t.Fatalf("expected READY_FOR_SUBMISSION status, got: %s", text2)
	}
	if strings.Contains(text2, "Issue submitted successfully") {
		t.Fatalf("semantic violation: claimed issue submitted when only URL was generated: %s", text2)
	}
	if !strings.Contains(text2, "https://github.com/will2469/argus/issues/new") {
		t.Fatalf("expected browser creation URL in output, got: %s", text2)
	}
}

func TestEscapeCodeFence(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no backticks",
			input:    "func foo() { return 42 }",
			expected: "func foo() { return 42 }",
		},
		{
			name:     "single backtick",
			input:    "func `foo`() { return 42 }",
			expected: "func `foo`() { return 42 }",
		},
		{
			name:     "double backtick",
			input:    "func ``foo``() { return 42 }",
			expected: "func ``foo``() { return 42 }",
		},
		{
			name:     "triple backtick - content spoofing attempt",
			input:    "func foo() { return 42 }\n```\n## FAKE METADATA\n- **FAKE:** true",
			expected: "func foo() { return 42 }\n`\u200b``\n## FAKE METADATA\n- **FAKE:** true",
		},
		{
			name:     "multiple triple backticks",
			input:    "```\ncode\n```\nmore code\n```",
			expected: "`\u200b``\ncode\n`\u200b``\nmore code\n`\u200b``",
		},
		{
			name:     "mixed fence lengths",
			input:    "```go\nfunc foo() {}\n```\n```text\nplain text\n```",
			expected: "`\u200b``go\nfunc foo() {}\n`\u200b``\n`\u200b``text\nplain text\n`\u200b``",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := escapeCodeFence(tc.input)
			if result != tc.expected {
				t.Fatalf("escapeCodeFence(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestFormatIssueBody_CodeFenceSecurity(t *testing.T) {
	// Test that malicious snippets cannot break out of code fence
	maliciousSnippet := "```go\nfunc foo() { return 42 }\n```\n## SPOOFED METADATA\n- **Reported via:** FAKE SERVER"

	body := FormatIssueBody("A14", "Test description", maliciousSnippet, "false-positive")

	// Verify that the metadata section is still intact after the code snippet
	metadataPos := strings.Index(body, "## Metadata")
	if metadataPos == -1 {
		t.Fatal("## Metadata section not found in formatted body")
	}

	// Verify that the malicious fence sequence was escaped
	if strings.Contains(body, "```\n## SPOOFED METADATA") {
		t.Fatal("CRITICAL: Content spoofing detected - fence sequence not escaped properly")
	}

	// Verify the zero-width space was inserted
	zeroWidthSpace := string(rune(0x200B))
	if !strings.Contains(body, "`"+zeroWidthSpace+"``") {
		t.Fatal("Expected zero-width space escape sequence not found")
	}

	// Verify the legitimate metadata is preserved
	if !strings.Contains(body, "**Reported via:** Argus MCP Server") {
		t.Fatal("Legitimate metadata was corrupted by spoofing attempt")
	}
}
