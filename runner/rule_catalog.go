package runner

import (
	"strings"
)

// RuleMetadata holds static informational attributes for an Argus rule.
type RuleMetadata struct {
	ID          string
	Code        string
	Identifier  string
	Severity    string
	Category    string
	Description string
	WikiURL     string
}

type ruleDefinition struct {
	Identifier  string
	Severity    string
	Category    string
	Description string
}

var rawCatalog = map[string]ruleDefinition{
	"A01":  {"UNSAFE_SQL_CONCATENATION", "CRITICAL", "Security (CWE-89)", "Forbids raw string concatenation in queries; mandates $1, $2 bind parameters."},
	"A02":  {"MISSING_DEFER_CLOSE", "HIGH", "Reliability", "Mandates defer rows.Close() immediately after query execution to prevent pool leaks."},
	"A03":  {"UNBOUNDED_CONTEXT", "HIGH", "Resilience", "Prohibits raw context.Background() or context.TODO() in query calls."},
	"A04":  {"UNSAFE_ORDER_BY", "HIGH", "Security (CWE-89)", "Dynamic ORDER BY / GROUP BY must be validated against compile-time static allowlists."},
	"A05":  {"AUDIT_LOG_IMMUTABILITY", "CRITICAL", "Compliance", "Prohibits UPDATE, DELETE, TRUNCATE, or MERGE on append-only audit ledger tables."},
	"A06":  {"RUNTIME_DDL", "CRITICAL", "Security", "Blocks DDL execution in application runtime code; runtime roles must be DML-only."},
	"A07":  {"ERROR_LEAK", "HIGH", "Privacy (CWE-200)", "Forbids leaking raw database error messages or PII details to external clients."},
	"A08":  {"TX_EXTERNAL_IO", "HIGH", "Performance", "Forbids blocking network/disk I/O (HTTP, gRPC, disk) inside active database transactions."},
	"A09":  {"ADVISORY_LOCK", "HIGH", "Concurrency", "Mandates transaction-level advisory locks; forbids session locks in pooled connections."},
	"A10":  {"ISOLATION_LEVEL", "HIGH", "Integrity", "Critical financial/inventory mutations must declare explicit Serializable or FOR UPDATE."},
	"A11":  {"DESTRUCTIVE_MIGRATION", "CRITICAL", "Zero-Downtime", "Prohibits destructive DDL (DROP COLUMN, RENAME) in single releases without expand-contract."},
	"A12":  {"TIMEOUT_CONFIG", "HIGH", "Availability", "Mandates 4-tier timeout settings (statement_timeout, lock_timeout, idle timeouts)."},
	"A13":  {"MISSING_DOWN_MIGRATION", "HIGH", "Rollback Safety", "Every .up.sql migration must have a non-empty, deterministic symmetric .down.sql."},
	"A14":  {"FORBIDDEN_SELECT_STAR", "HIGH", "Performance", "Prohibits wildcard SELECT *; mandates explicit column projection to avoid TOAST bloat."},
	"A15":  {"FORBIDDEN_DDL_APP_ROLE_GRANT", "CRITICAL", "Security (CWE-250)", "Prohibits granting DDL/ALL privileges or table ownership to application runtime roles."},
	"A16":  {"MAX_CONNS_CONFIG", "HIGH", "Scalability", "Enforces mathematically bounded MaxConns on connection pools to prevent process thrashing."},
	"A17":  {"FORBIDDEN_QUERY_IN_LOOP", "HIGH", "Performance", "Eliminates N+1 query patterns inside loops in favor of WHERE id = ANY($1) or pgx.Batch."},
	"A18":  {"MISSING_ROWS_ERR_CHECK", "HIGH", "Integrity (CWE-391)", "Mandates rows.Err() check after rows.Next() loops to catch silent network truncations."},
	"A19":  {"UNBOUNDED_QUERY_LIMIT", "HIGH", "Resilience (CWE-400)", "Queries on high-cardinality tables must have an explicit LIMIT or keyset pagination."},
	"A20":  {"PARAM_LIMIT_65535", "HIGH", "Protocol Limits", "Prevents exceeding PostgreSQL's 65,535 wire parameter ceiling; recommends pgx.CopyFrom."},
	"A21":  {"UNBOUNDED_ROW_LOCK_BLOCKING", "HIGH", "Concurrency", "Queue queries (SELECT ... FOR UPDATE) must use SKIP LOCKED or NOWAIT."},
	"A22":  {"SERIALIZATION_FAILURE_RETRY", "HIGH", "Fault Tolerance", "Serializable transactions must be wrapped in automated retry loops catching SQLSTATE 40001."},
	"A23":  {"TRANSACTION_TIMEOUT_CONFIG", "HIGH", "Modern PG17/18", "Enforces transaction_timeout cap on connection pools to prevent XID horizon freezing."},
	"A24":  {"TENANT_ISOLATION_LEAK", "CRITICAL", "Multi-Tenancy", "Mandates explicit tenant predicates (WHERE tenant_id = $1) or verified RLS context."},
	"A25":  {"EXPENSIVE_CPU_IN_TRANSACTION", "HIGH", "Performance", "Prohibits CPU-heavy tasks (bcrypt, argon2, RSA keygen, PDF rendering) inside transactions."},
	"A26":  {"LIKE_WILDCARD_INJECTION", "HIGH", "Security (CWE-89)", "Mandates escaping wildcard characters (\\, %, _) on user input bound to LIKE/ILIKE."},
	"A27":  {"NON_CONCURRENT_INDEX_CREATION", "CRITICAL", "Zero-Downtime", "Indexes on existing tables must use CREATE INDEX CONCURRENTLY to avoid write lockouts."},
	"A28":  {"TABLE_LOCKING_CONSTRAINT_ADDITION", "CRITICAL", "Zero-Downtime", "FK and CHECK constraints must use 2-phase NOT VALID followed by VALIDATE CONSTRAINT."},
	"A29":  {"UNINDEXED_FOREIGN_KEY", "HIGH", "Performance", "Every foreign key on child tables must have a supporting B-tree index (anti-table scan)."},
	"A30":  {"TIMESTAMP_WITHOUT_TIMEZONE", "CRITICAL", "Temporal Hygiene", "Prohibits bare TIMESTAMP; mandates TIMESTAMPTZ (UTC-normalized) for temporal determinism."},
	"E001": {"UNABLE_TO_ANALYZE_MIGRATION", "HIGH", "Engine Reliability", "PostgreSQL migration syntax could not be parsed by libpg_query engine."},
}

// GetRuleMetadata returns the authoritative RuleMetadata for a given rule key, identifier, or code.
func GetRuleMetadata(ruleKey string) RuleMetadata {
	norm := NormalizeRuleCode(ruleKey)
	id := strings.TrimPrefix(norm, "ARGUS-")
	if def, ok := rawCatalog[id]; ok {
		wikiURL := "https://github.com/will2469/argus/wiki/" + norm
		if id == "E001" {
			wikiURL = "https://github.com/will2469/argus/wiki"
		}
		return RuleMetadata{
			ID:          id,
			Code:        norm,
			Identifier:  def.Identifier,
			Severity:    def.Severity,
			Category:    def.Category,
			Description: def.Description,
			WikiURL:     wikiURL,
		}
	}
	desc := GetRuleDescription(ruleKey)
	wikiURL := "https://github.com/will2469/argus/wiki"
	if strings.HasPrefix(norm, "ARGUS-") {
		wikiURL = "https://github.com/will2469/argus/wiki/" + norm
	}
	return RuleMetadata{
		ID:          id,
		Code:        norm,
		Identifier:  desc,
		Severity:    "HIGH",
		Category:    "Database Hygiene",
		Description: desc,
		WikiURL:     wikiURL,
	}
}
