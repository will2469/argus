// Package a24_tenant_leak manages configuration and registry for multi-tenant table isolation.
package a24_tenant_leak

import (
	"strings"

	"github.com/will2469/argus/shared/config"
)

// TenantConfig stores the tenant column and multi-tenant table lookup map.
type TenantConfig struct {
	TenantColumn string
	TenantTables map[string]bool
}

// LoadTenantConfig loads tenant settings from config with open-source defaults.
func LoadTenantConfig(cfg *config.Config) *TenantConfig {
	tenantCol := "tenant_id"
	defaultTables := []string{
		"tenant_data",
	}

	if cfg != nil {
		if col := cfg.GetString("ARGUS-A24", "tenant_column", ""); col != "" {
			tenantCol = strings.ToLower(strings.TrimSpace(col))
		}
		if tables := cfg.GetStringSlice("ARGUS-A24", "tenant_tables", nil); len(tables) > 0 {
			defaultTables = tables
		}
	}

	tableMap := make(map[string]bool, len(defaultTables))
	for _, t := range defaultTables {
		norm := strings.ToLower(strings.TrimSpace(t))
		if norm != "" {
			tableMap[norm] = true
		}
	}

	return &TenantConfig{
		TenantColumn: tenantCol,
		TenantTables: tableMap,
	}
}

// IsTenantTable checks if a table name is in the multi-tenant registry.
func (tc *TenantConfig) IsTenantTable(table string) bool {
	if tc == nil || tc.TenantTables == nil {
		return false
	}
	norm := strings.ToLower(strings.TrimSpace(table))
	return tc.TenantTables[norm]
}
