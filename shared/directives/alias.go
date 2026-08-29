package directives

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var (
	aliasMu   sync.RWMutex
	codeRegex = regexp.MustCompile(`^(?:ARGUS[-_]?)?A0*(\d+)$`)

	// Pre-populated aliases for all ARGUS-A01 through ARGUS-A30 rules.
	// Aliases are stored in cleaned format (uppercase, hyphens instead of underscores).
	ruleAliases = map[string]string{
		// A01 - A10
		"UNSAFE-SQL-CONCATENATION": "ARGUS-A01",
		"SQL-CONCAT":               "ARGUS-A01",
		"MISSING-DEFER-CLOSE":      "ARGUS-A02",
		"UNCLOSED-ROWS":            "ARGUS-A02",
		"UNBOUNDED-CONTEXT":        "ARGUS-A03",
		"CONTEXT":                  "ARGUS-A03",
		"UNSAFE-ORDER-BY":          "ARGUS-A04",
		"ORDER-BY":                 "ARGUS-A04",
		"AUDIT-LOG-IMMUTABILITY":   "ARGUS-A05",
		"AUDIT-IMMUTABILITY":       "ARGUS-A05",
		"RUNTIME-DDL":              "ARGUS-A06",
		"FORBIDDEN-RUNTIME-DDL":    "ARGUS-A06",
		"ERROR-LEAK":               "ARGUS-A07",
		"TX-EXTERNAL-IO":           "ARGUS-A08",
		"TX-IO":                    "ARGUS-A08",
		"ADVISORY-LOCK":            "ARGUS-A09",
		"ISOLATION-LEVEL":          "ARGUS-A10",

		// A11 - A20
		"DESTRUCTIVE-MIGRATION":        "ARGUS-A11",
		"TIMEOUT-CONFIG":               "ARGUS-A12",
		"MIGRATION-TRANSACTION":        "ARGUS-A12",
		"MISSING-DOWN-MIGRATION":       "ARGUS-A13",
		"FORBIDDEN-SELECT-STAR":        "ARGUS-A14",
		"SELECT-STAR":                  "ARGUS-A14",
		"FORBIDDEN-DDL-APP-ROLE-GRANT": "ARGUS-A15",
		"DDL-GRANT":                    "ARGUS-A15",
		"MAX-CONNS-CONFIG":             "ARGUS-A16",
		"POOL-EXHAUSTION":              "ARGUS-A16",
		"FORBIDDEN-QUERY-IN-LOOP":      "ARGUS-A17",
		"N-PLUS-ONE":                   "ARGUS-A17",
		"MISSING-ROWS-ERR-CHECK":       "ARGUS-A18",
		"ROWS-ERR":                     "ARGUS-A18",
		"MISSING-INDEX-ON-FILTER":      "ARGUS-A18",
		"UNBOUNDED-QUERY-LIMIT":        "ARGUS-A19",
		"UNBOUNDED-LIMIT":              "ARGUS-A19",
		"PREPARED-STATEMENT-LEAK":      "ARGUS-A20",
		"PREPARED-STATEMENT":           "ARGUS-A20",

		// A21 - A30
		"LARGE-IN-CLAUSE":                   "ARGUS-A21",
		"IN-CLAUSE":                         "ARGUS-A21",
		"SERIALIZATION-FAILURE-RETRY":       "ARGUS-A22",
		"SERIALIZABLE-RETRY":                "ARGUS-A22",
		"TRANSIENT-ERROR-NO-RETRY":          "ARGUS-A22",
		"MISSING-STATEMENT-TIMEOUT":         "ARGUS-A23",
		"STATEMENT-TIMEOUT":                 "ARGUS-A23",
		"MULTI-TENANT-LEAK":                 "ARGUS-A24",
		"TENANT-LEAK":                       "ARGUS-A24",
		"UNSAFE-JSON-OPERATOR":              "ARGUS-A25",
		"JSON-INJECTION":                    "ARGUS-A25",
		"LOCKING-READ-OUTSIDE-TX":           "ARGUS-A26",
		"FOR-UPDATE-NO-TX":                  "ARGUS-A26",
		"NON-CONCURRENT-INDEX-CREATION":     "ARGUS-A27",
		"CONCURRENT-INDEX":                  "ARGUS-A27",
		"TABLE-LOCKING-CONSTRAINT-ADDITION": "ARGUS-A28",
		"CONSTRAINT-LOCK":                   "ARGUS-A28",
		"UNINDEXED-FOREIGN-KEY":             "ARGUS-A29",
		"UNINDEXED-FK":                      "ARGUS-A29",
		"TIMESTAMP-WITHOUT-TIMEZONE":        "ARGUS-A30",
		"TIMESTAMPTZ":                       "ARGUS-A30",
	}
)

// RegisterAlias allows rules to dynamically register aliases at init() or runtime,
// making the directive engine fully extensible to future rules without modifying core files.
func RegisterAlias(alias, ruleCode string) {
	aliasMu.Lock()
	defer aliasMu.Unlock()
	cleaned := cleanAliasKey(alias)
	canonical, _ := normalizeRule(ruleCode)
	ruleAliases[cleaned] = canonical
}

func cleanAliasKey(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	return strings.ReplaceAll(s, "_", "-")
}

// normalizeRule decomposes and canonicalizes rules like:
// "a07" -> ("ARGUS-A07", "ARGUS-A07")
// "a18.detail" -> ("ARGUS-A18.DETAIL", "ARGUS-A18")
// "n_plus_one" -> ("ARGUS-A17", "ARGUS-A17")
// "FORBIDDEN_QUERY_IN_LOOP" -> ("ARGUS-A17", "ARGUS-A17")
func normalizeRule(rule string) (canonical string, base string) {
	r := cleanAliasKey(rule)

	parts := strings.SplitN(r, ".", 2)
	basePart := parts[0]
	clause := ""
	if len(parts) > 1 {
		clause = parts[1]
	}

	normalizedBase := normalizeBaseCode(basePart)
	if clause != "" {
		return normalizedBase + "." + clause, normalizedBase
	}
	return normalizedBase, normalizedBase
}

// normalizeBaseCode evaluates rule identifiers:
//  1. Dynamic regex pattern recognition: Any A<num>, ARGUS-A<num>, ARGUS_A<num> is normalized
//     to standard canonical ARGUS-Axx format automatically.
//  2. Alias table lookup: Maps human-readable identifiers (e.g. FORBIDDEN_SELECT_STAR) to codes.
//  3. Fallback: Returns cleaned uppercase code as-is.
func normalizeBaseCode(raw string) string {
	if raw == "*" || raw == "ALL" {
		return "ALL"
	}

	// 1. Dynamic pattern recognition for numeric rule codes (ARGUS-A01 .. ARGUS-A999)
	if matches := codeRegex.FindStringSubmatch(raw); len(matches) == 2 {
		if num, err := strconv.Atoi(matches[1]); err == nil {
			if num < 100 {
				return fmt.Sprintf("ARGUS-A%02d", num)
			}
			return fmt.Sprintf("ARGUS-A%d", num)
		}
	}

	// 2. Thread-safe alias map lookup
	aliasMu.RLock()
	code, ok := ruleAliases[raw]
	aliasMu.RUnlock()
	if ok {
		return code
	}

	return raw
}
