package runner

import (
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
)

// CanonicalDescriptions provides standardized descriptions for standard Argus rules.
var CanonicalDescriptions = map[string]string{
	"A01":  "UNSAFE_SQL_CONCATENATION",
	"A02":  "MISSING_DEFER_CLOSE",
	"A03":  "UNBOUNDED_CONTEXT",
	"A04":  "UNSAFE_ORDER_BY",
	"A05":  "AUDIT_LOG_IMMUTABILITY",
	"A06":  "RUNTIME_DDL",
	"A07":  "ERROR_LEAK",
	"A08":  "TX_EXTERNAL_IO",
	"A09":  "ADVISORY_LOCK",
	"A10":  "ISOLATION_LEVEL",
	"A11":  "DESTRUCTIVE_MIGRATION",
	"A12":  "TIMEOUT_CONFIG",
	"A13":  "MISSING_DOWN_MIGRATION",
	"A14":  "FORBIDDEN_SELECT_STAR",
	"A15":  "FORBIDDEN_DDL_APP_ROLE_GRANT",
	"A16":  "MAX_CONNS_CONFIG",
	"A17":  "FORBIDDEN_QUERY_IN_LOOP",
	"A18":  "MISSING_ROWS_ERR_CHECK",
	"A19":  "UNBOUNDED_QUERY_LIMIT",
	"A20":  "PARAM_LIMIT_65535",
	"A21":  "UNBOUNDED_ROW_LOCK_BLOCKING",
	"A22":  "SERIALIZATION_FAILURE_RETRY",
	"A23":  "TRANSACTION_TIMEOUT_CONFIG",
	"A24":  "TENANT_ISOLATION_LEAK",
	"A25":  "EXPENSIVE_CPU_IN_TRANSACTION",
	"A26":  "LIKE_WILDCARD_INJECTION",
	"A27":  "NON_CONCURRENT_INDEX_CREATION",
	"A28":  "TABLE_LOCKING_CONSTRAINT_ADDITION",
	"A29":  "UNINDEXED_FOREIGN_KEY",
	"A30":  "TIMESTAMP_WITHOUT_TIMEZONE",
	"A31":  "UNGUARDED_MUTATION_WITHOUT_AUDIT",
	"E001": "UNABLE_TO_ANALYZE_MIGRATION",
}

// RuleAliases maps scanner rule identifiers and descriptive names to standardized ARGUS-Axx rule codes.
var RuleAliases = map[string]string{
	"UNSAFE_SQL_CONCATENATION":          "ARGUS-A01",
	"MISSING_DEFER_CLOSE":               "ARGUS-A02",
	"UNBOUNDED_CONTEXT":                 "ARGUS-A03",
	"UNSAFE_ORDER_BY":                   "ARGUS-A04",
	"UNSAFE_DYNAMIC_ORDERBY":            "ARGUS-A04",
	"AUDIT_LOG_IMMUTABILITY":            "ARGUS-A05",
	"FORBIDDEN_AUDIT_MUTATION":          "ARGUS-A05",
	"RUNTIME_DDL":                       "ARGUS-A06",
	"RUNTIME_DDL_EXECUTION":             "ARGUS-A06",
	"ERROR_LEAK":                        "ARGUS-A07",
	"DATABASE_ERROR_LEAK":               "ARGUS-A07",
	"TX_EXTERNAL_IO":                    "ARGUS-A08",
	"TRANSACTION_BLOCKING_IO":           "ARGUS-A08",
	"ADVISORY_LOCK":                     "ARGUS-A09",
	"UNSAFE_ADVISORY_LOCK":              "ARGUS-A09",
	"ISOLATION_LEVEL":                   "ARGUS-A10",
	"WEAK_ISOLATION_LEVEL":              "ARGUS-A10",
	"DESTRUCTIVE_MIGRATION":             "ARGUS-A11",
	"TIMEOUT_CONFIG":                    "ARGUS-A12",
	"TIMEOUT_CONFIG_MISSING":            "ARGUS-A12",
	"MISSING_DOWN_MIGRATION":            "ARGUS-A13",
	"FORBIDDEN_SELECT_STAR":             "ARGUS-A14",
	"FORBIDDEN_DDL_APP_ROLE_GRANT":      "ARGUS-A15",
	"MAX_CONNS_CONFIG":                  "ARGUS-A16",
	"UNBOUNDED_MAX_CONNS":               "ARGUS-A16",
	"FORBIDDEN_QUERY_IN_LOOP":           "ARGUS-A17",
	"ROWS_ERR":                          "ARGUS-A18",
	"MISSING_ROWS_ERR_CHECK":            "ARGUS-A18",
	"UNCHECKED_ROWS_ERROR":              "ARGUS-A18",
	"UNBOUNDED_QUERY_LIMIT":             "ARGUS-A19",
	"UNBOUNDED_HIGH_CARDINALITY_QUERY":  "ARGUS-A19",
	"PARAM_LIMIT_65535":                 "ARGUS-A20",
	"UNBOUNDED_BATCH_PARAMS":            "ARGUS-A20",
	"WIRE_PARAM_LIMIT":                  "ARGUS-A20",
	"UNBOUNDED_ROW_LOCK_BLOCKING":       "ARGUS-A21",
	"BLOCKING_ROW_LOCK":                 "ARGUS-A21",
	"LOCK_CONVOY":                       "ARGUS-A21",
	"SERIALIZATION_FAILURE_RETRY":       "ARGUS-A22",
	"MISSING_SERIALIZABLE_RETRY":        "ARGUS-A22",
	"RETRY_TRANSACTION":                 "ARGUS-A22",
	"TRANSACTION_TIMEOUT_CONFIG":        "ARGUS-A23",
	"MISSING_TX_TIMEOUT":                "ARGUS-A23",
	"TX_TIMEOUT_GUC":                    "ARGUS-A23",
	"TENANT_ISOLATION_LEAK":             "ARGUS-A24",
	"EXPENSIVE_CPU_IN_TRANSACTION":      "ARGUS-A25",
	"EXPENSIVE_CPU_IN_TX":               "ARGUS-A25",
	"LIKE_WILDCARD_INJECTION":           "ARGUS-A26",
	"NON_CONCURRENT_INDEX_CREATION":     "ARGUS-A27",
	"TABLE_LOCKING_CONSTRAINT_ADDITION": "ARGUS-A28",
	"UNINDEXED_FOREIGN_KEY":             "ARGUS-A29",
	"TIMESTAMP_WITHOUT_TIMEZONE":        "ARGUS-A30",
	"UNGUARDED_MUTATION_WITHOUT_AUDIT":  "ARGUS-A31",
	"UNABLE_TO_ANALYZE_MIGRATION":       "ARGUS-E001",
}

// CalculateCheckedComponents determines the relevant component count for a rule.
func CalculateCheckedComponents(id string, querySites, migrationFiles, totalFiles int) int {
	switch id {
	case "A11", "A13", "A15", "A27", "A28", "A29", "A30", "E001":
		return migrationFiles
	case "A05", "A06", "A08", "A09", "A10", "A12", "A16", "A22", "A23", "A25", "A31":
		return totalFiles
	default:
		if querySites > 0 {
			return querySites
		}
		return totalFiles
	}
}

// BuildDynamicRuleAuditInfo dynamically builds the audit info list from active analyzers and config.
func BuildDynamicRuleAuditInfo(analyzers []*analysis.Analyzer, cfg *config.Config, querySites, migrationFiles, totalScannedFiles int, issues []Issue) []RuleAuditInfo {
	issueCounts := make(map[string]int)
	for _, issue := range issues {
		norm := normalizeRuleCode(issue.Rule)
		issueCounts[norm]++
	}

	seen := make(map[string]bool)
	var rules []RuleAuditInfo

	// 1. If analyzers slice is provided, extract dynamically from active analyzers
	for _, an := range analyzers {
		if an == nil {
			continue
		}
		id, code, desc := parseAnalyzerMeta(an)
		if seen[code] {
			continue
		}
		seen[code] = true

		if cfg != nil && !cfg.IsRuleEnabled(code) {
			continue
		}

		count := issueCounts[code]
		if count == 0 {
			count = issueCounts[id] + issueCounts[desc]
		}
		status := "PASS"
		if count > 0 {
			status = "FAILED"
		}

		checked := CalculateCheckedComponents(id, querySites, migrationFiles, totalScannedFiles)

		rules = append(rules, RuleAuditInfo{
			ID:                id,
			Code:              code,
			Description:       desc,
			CheckedComponents: checked,
			IssuesFound:       count,
			Status:            status,
		})
	}

	// 2. Fallback: if no analyzers slice provided, construct from enabled rules in config
	if len(rules) == 0 {
		for i := 1; i <= 30; i++ {
			id := formatID(i)
			code := "ARGUS-" + id
			if cfg != nil && !cfg.IsRuleEnabled(code) {
				continue
			}

			desc := CanonicalDescriptions[id]
			count := issueCounts[code]
			if count == 0 {
				count = issueCounts[id] + issueCounts[desc]
			}
			status := "PASS"
			if count > 0 {
				status = "FAILED"
			}

			checked := CalculateCheckedComponents(id, querySites, migrationFiles, totalScannedFiles)

			rules = append(rules, RuleAuditInfo{
				ID:                id,
				Code:              code,
				Description:       desc,
				CheckedComponents: checked,
				IssuesFound:       count,
				Status:            status,
			})
		}
	}

	// Sort rules numerically by ID (e.g. A01, A02, ... A10, ... A100)
	sort.Slice(rules, func(i, j int) bool {
		numI, errI := strconv.Atoi(strings.TrimPrefix(rules[i].ID, "A"))
		numJ, errJ := strconv.Atoi(strings.TrimPrefix(rules[j].ID, "A"))
		if errI == nil && errJ == nil {
			return numI < numJ
		}
		return rules[i].ID < rules[j].ID
	})

	return rules
}

func parseAnalyzerMeta(an *analysis.Analyzer) (id, code, desc string) {
	name := strings.TrimSpace(an.Name)
	upper := strings.ToUpper(name)
	parts := strings.Split(upper, "_")

	if strings.HasPrefix(upper, "ARGUS-") {
		id = strings.TrimPrefix(upper, "ARGUS-")
		code = upper
	} else if len(upper) >= 2 && upper[0] == 'A' {
		id = parts[0]
		code = "ARGUS-" + id
	} else if len(parts) >= 2 && strings.HasPrefix(parts[1], "A") {
		id = parts[1]
		code = "ARGUS-" + id
	} else {
		id = upper
		code = "ARGUS-" + upper
	}

	if canon, ok := CanonicalDescriptions[id]; ok {
		desc = canon
	} else if len(parts) >= 3 {
		desc = strings.ToUpper(strings.Join(parts[2:], "_"))
	} else {
		desc = upper
	}

	return id, code, desc
}

func formatID(i int) string {
	if i < 10 {
		return "A0" + strconv.Itoa(i)
	}
	return "A" + strconv.Itoa(i)
}
