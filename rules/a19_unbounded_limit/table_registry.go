// Package a19_unbounded_limit maintains the high-cardinality table and key column registry for ARGUS-A19.
package a19_unbounded_limit

import (
	"strings"

	"github.com/will2469/argus/shared/config"
)

var defaultHighCardinalityTables = []string{
	"audit_logs",
	"transactions",
	"events",
	"orders",
	"activity_logs",
}

var defaultKeyColumns = []string{
	"id",
	"uuid",
	"pk",
	"key",
}

// GetHighCardinalityTables retrieves the set of configured high-growth / high-cardinality tables.
func GetHighCardinalityTables(cfg *config.Config) map[string]bool {
	tableMap := make(map[string]bool)

	if cfg != nil {
		custom := cfg.GetStringSlice(RuleCode, "high_cardinality_tables", nil)
		if len(custom) == 0 {
			custom = cfg.GetStringSlice(RuleCode, "high_growth_tables", nil)
		}
		if len(custom) > 0 {
			for _, t := range custom {
				if trimmed := strings.TrimSpace(t); trimmed != "" {
					tableMap[strings.ToLower(trimmed)] = true
				}
			}
			return tableMap
		}
	}

	// Default fallback if not configured in .argus.yaml
	for _, t := range defaultHighCardinalityTables {
		tableMap[strings.ToLower(t)] = true
	}
	return tableMap
}

// GetKeyColumns retrieves the set of configured point-lookup / primary key columns.
func GetKeyColumns(cfg *config.Config) map[string]bool {
	keyMap := make(map[string]bool)

	// Seed generic defaults
	for _, k := range defaultKeyColumns {
		keyMap[strings.ToLower(k)] = true
	}

	if cfg == nil {
		return keyMap
	}

	custom := cfg.GetStringSlice(RuleCode, "point_lookup_columns", nil)
	if len(custom) == 0 {
		custom = cfg.GetStringSlice(RuleCode, "key_columns", nil)
	}
	for _, k := range custom {
		if trimmed := strings.TrimSpace(k); trimmed != "" {
			keyMap[strings.ToLower(trimmed)] = true
		}
	}
	return keyMap
}

// IsHighCardinalityTable checks if a given table name matches the registered high-cardinality set.
func IsHighCardinalityTable(tableName string, tableMap map[string]bool) bool {
	normalized := strings.ToLower(strings.TrimSpace(tableName))
	normalized = strings.Trim(normalized, `"'`)
	if idx := strings.LastIndex(normalized, "."); idx != -1 {
		normalized = normalized[idx+1:]
	}
	return tableMap[normalized]
}
