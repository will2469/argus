package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("expected non-nil default config")
	}
	if !cfg.IsRuleEnabled("ARGUS-A01") {
		t.Errorf("expected ARGUS-A01 enabled by default")
	}
	if !cfg.IsRuleEnabled("ARGUS-A30") {
		t.Errorf("expected ARGUS-A30 enabled by default")
	}
}

func TestLoadConfigFallback(t *testing.T) {
	tempDir := t.TempDir()
	cfg, err := LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("unexpected error on fallback: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil fallback config")
	}
	if !cfg.IsRuleEnabled("ARGUS-A14") {
		t.Errorf("expected ARGUS-A14 enabled in fallback")
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	tempDir := t.TempDir()
	yamlContent := `
version: "1"
rules:
  ARGUS-A01:
    enabled: false
  ARGUS-A05:
    enabled: true
    audit_tables:
      - "audit_logs"
      - "custom_audit"
  ARGUS-A10:
    enabled: true
    critical_tables:
      - "custom_ledger"
  ARGUS-A16:
    enabled: true
    max_conns_limit: 42
  ARGUS-A24:
    enabled: true
    tenant_column: "organization_id"
`
	if err := os.WriteFile(filepath.Join(tempDir, ".argus.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed writing test .argus.yaml: %v", err)
	}

	cfg, err := LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("failed loading config: %v", err)
	}
	if cfg.IsRuleEnabled("ARGUS-A01") {
		t.Errorf("expected ARGUS-A01 to be disabled from YAML")
	}

	auditTables := cfg.GetStringSlice("ARGUS-A05", "audit_tables", []string{"default"})
	if len(auditTables) != 2 || auditTables[1] != "custom_audit" {
		t.Errorf("unexpected audit_tables: %v", auditTables)
	}

	criticalTables := cfg.GetStringSlice("ARGUS-A10", "critical_tables", []string{"default"})
	if len(criticalTables) != 1 || criticalTables[0] != "custom_ledger" {
		t.Errorf("unexpected critical_tables: %v", criticalTables)
	}

	conns := cfg.GetInt("ARGUS-A16", "max_conns_limit", 10)
	if conns != 42 {
		t.Errorf("expected max_conns_limit 42, got %d", conns)
	}

	col := cfg.GetString("ARGUS-A24", "tenant_column", "tenant_id")
	if col != "organization_id" {
		t.Errorf("expected tenant_column organization_id, got %s", col)
	}
}
