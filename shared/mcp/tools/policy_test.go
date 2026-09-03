package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateToolPolicy_ScanTraversal(t *testing.T) {
	tool, ok := DefaultRegistry.Get("argus_scan")
	if !ok {
		t.Fatal("argus_scan not found in default registry")
	}

	// argus_scan path traversal in dirs
	err := tool.ValidatePolicy(json.RawMessage(`{"dirs": ["../../escape"]}`))
	if err == nil || !strings.Contains(err.Error(), "path authority violation") {
		t.Fatalf("expected traversal error in dirs, got: %v", err)
	}

	// argus_scan path traversal in migrations
	err = tool.ValidatePolicy(json.RawMessage(`{"migrations": ["/etc"]}`))
	if err == nil || !strings.Contains(err.Error(), "path authority violation") {
		t.Fatalf("expected traversal error in migrations, got: %v", err)
	}

	// valid scan dirs within root
	err = tool.ValidatePolicy(json.RawMessage(`{"dirs": ["rules"]}`))
	if err != nil {
		t.Fatalf("expected valid dirs to pass, got: %v", err)
	}
}

func TestValidateToolPolicy_MigrationSQL(t *testing.T) {
	tool, ok := DefaultRegistry.Get("argus_check_migration")
	if !ok {
		t.Fatal("argus_check_migration not found in default registry")
	}

	// Empty SQL
	err := tool.ValidatePolicy(json.RawMessage(`{"sql": "   "}`))
	if err == nil || !strings.Contains(err.Error(), "empty or whitespace only") {
		t.Fatalf("expected empty sql error, got: %v", err)
	}

	// Oversized SQL (> 1MB)
	hugeSQL := strings.Repeat("A", 1024*1024+10)
	rawHuge, _ := json.Marshal(map[string]string{"sql": hugeSQL})
	err = tool.ValidatePolicy(rawHuge)
	if err == nil || !strings.Contains(err.Error(), "exceeds maximum allowed size") {
		t.Fatalf("expected oversized sql error, got: %v", err)
	}
}

func TestValidateToolPolicy_ExplainRule(t *testing.T) {
	tool, ok := DefaultRegistry.Get("argus_explain_rule")
	if !ok {
		t.Fatal("argus_explain_rule not found in default registry")
	}

	// Empty rule_code
	err := tool.ValidatePolicy(json.RawMessage(`{"rule_code": "  "}`))
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("expected empty rule_code error, got: %v", err)
	}

	// Oversized rule_code (> 32 chars)
	err = tool.ValidatePolicy(json.RawMessage(`{"rule_code": "` + strings.Repeat("A", 40) + `"}`))
	if err == nil || !strings.Contains(err.Error(), "exceeds maximum allowed length") {
		t.Fatalf("expected oversized rule_code error, got: %v", err)
	}
}
