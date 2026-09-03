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
	"A01": "UNSAFE_SQL_CONCATENATION",
	"A02": "MISSING_DEFER_CLOSE",
	"A03": "UNBOUNDED_CONTEXT",
	"A04": "UNSAFE_ORDER_BY",
	"A05": "AUDIT_LOG_IMMUTABILITY",
	"A06": "RUNTIME_DDL",
	"A07": "ERROR_LEAK",
	"A08": "TX_EXTERNAL_IO",
	"A09": "ADVISORY_LOCK",
	"A10": "ISOLATION_LEVEL",
	"A11": "DESTRUCTIVE_MIGRATION",
	"A12": "TIMEOUT_CONFIG",
	"A13": "MISSING_DOWN_MIGRATION",
	"A14": "FORBIDDEN_SELECT_STAR",
	"A15": "FORBIDDEN_DDL_APP_ROLE_GRANT",
	"A16": "MAX_CONNS_CONFIG",
	"A17": "FORBIDDEN_QUERY_IN_LOOP",
	"A18": "ROWS_ERR",
	"A19": "UNBOUNDED_QUERY_LIMIT",
	"A20": "PARAM_LIMIT_65535",
	"A21": "UNBOUNDED_ROW_LOCK_BLOCKING",
	"A22": "SERIALIZATION_FAILURE_RETRY",
	"A23": "TRANSACTION_TIMEOUT_CONFIG",
	"A24": "TENANT_ISOLATION_LEAK",
	"A25": "EXPENSIVE_CPU_IN_TRANSACTION",
	"A26": "LIKE_WILDCARD_INJECTION",
	"A27": "NON_CONCURRENT_INDEX_CREATION",
	"A28": "TABLE_LOCKING_CONSTRAINT_ADDITION",
	"A29": "UNINDEXED_FOREIGN_KEY",
	"A30": "TIMESTAMP_WITHOUT_TIMEZONE",
	"E001": "UNABLE_TO_ANALYZE_MIGRATION",
}

// CalculateCheckedComponents determines the relevant component count for a rule.
func CalculateCheckedComponents(id string, querySites, migrationFiles, totalFiles int) int {
	switch id {
	case "A11", "A13", "A15", "A27", "A28", "A29", "A30", "E001":
		return migrationFiles
	case "A05", "A06", "A08", "A09", "A10", "A12", "A16", "A22", "A23", "A25":
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

		count := issueCounts[code] + issueCounts[id] + issueCounts[desc]
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
			count := issueCounts[code] + issueCounts[id] + issueCounts[desc]
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
	name := an.Name
	parts := strings.Split(name, "_")
	if len(parts) >= 2 && strings.HasPrefix(parts[1], "a") {
		id = strings.ToUpper(parts[1])
		code = "ARGUS-" + id
	} else {
		id = strings.ToUpper(name)
		code = id
	}

	if canon, ok := CanonicalDescriptions[id]; ok {
		desc = canon
	} else if len(parts) >= 3 {
		desc = strings.ToUpper(strings.Join(parts[2:], "_"))
	} else {
		desc = strings.ToUpper(name)
	}

	return id, code, desc
}

func formatID(i int) string {
	if i < 10 {
		return "A0" + strconv.Itoa(i)
	}
	return "A" + strconv.Itoa(i)
}
