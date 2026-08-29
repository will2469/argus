# Argus: Production-Grade Go & PostgreSQL 18 Static Analyzer

[![Go Reference](https://pkg.go.dev/badge/github.com/will2469/argus.svg)](https://pkg.go.dev/github.com/will2469/argus)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![PostgreSQL 18 Ready](https://img.shields.io/badge/PostgreSQL-18%2B%20Ready-blue.svg)](https://www.postgresql.org/)
[![Wiki Documentation](https://img.shields.io/badge/docs-Wiki-blue.svg)](https://github.com/will2469/argus/wiki)

**Argus** is an advanced compile-time static analyzer and pre-commit database safety linter for Go applications and PostgreSQL migrations. Built on top of the official Go analysis framework (`go/analysis`) and PostgreSQL's native C query parser (`libpg_query` via `pg_query_go`), Argus bridges the gap between Go application code and PostgreSQL engine internals.

It enforces **30 production-grade database invariants**, eliminating SQL injections, N+1 query latency collapses, connection pool starvation, cross-tenant data leaks, and catastrophic production table locking during zero-downtime schema migrations. Complete specifications and remediation examples are available in the [official Argus Wiki](https://github.com/will2469/argus/wiki).

---

## Why Argus?

Traditional linters only inspect Go syntax, while schema linters only look at migration syntax in isolation. Argus combines **both worlds**:

1. **Dual-Engine Precision:** Analyzes Go AST control-flow, scopes, and taint tracking alongside native PostgreSQL C-parser SQL AST (`SelectStmt`, `FromClause`, `IndexStmt`, etc.).
2. **PostgreSQL 18.x Internals Awareness:** Understands MVCC, `transaction_timeout` GUC, table lock modes (`SHARE` vs `SHARE UPDATE EXCLUSIVE`), and partition pruning.
3. **Zero False-Positive Target:** Built-in heuristics for idiomatic Go and `pgx/v5` constructs (e.g., `COUNT(*)`, `pgx.CollectRows`, keyset pagination).
4. **Dual Execution Modes:** Runs seamlessly as a standard `go vet` tool or as a standalone CLI for CI/CD pipelines.

---

## Quickstart

### Installation

#### Linux & macOS (One-Line Installer)

```bash
curl -fsSL https://raw.githubusercontent.com/will2469/argus/main/install.sh | bash
```

#### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/will2469/argus/main/install.ps1 | iex
```

#### Via Go Toolchain

```bash
go install github.com/will2469/argus/cmd/argus@latest
```

_Or download pre-compiled binaries directly from [GitHub Releases](https://github.com/will2469/argus/releases)._

### Basic Usage

```bash
# 1. Run static analysis across current directory and migrations
argus --dirs=. --migrations=migrations

# 2. Generate a comprehensive markdown audit report
argus --output=argus-report.md

# 3. Integrate directly into go vet
go vet -vettool=$(which argus) ./...

# 4. Self-update to the latest release
argus update # or: argus --update, argus -u
```

---

## 🤖 AI Agent Integration (MCP)

Argus ships with a built-in [Model Context Protocol](https://modelcontextprotocol.io) (MCP) server. AI coding agents like **Cursor**, **Claude Desktop**, **VS Code Copilot**, and **Antigravity** can automatically invoke Argus to audit database queries in real-time — no manual tagging required.

**Setup** — Add to your AI editor's MCP configuration:

```json
{
  "mcpServers": {
    "argus": {
      "command": "argus",
      "args": ["mcp"]
    }
  }
}
```

**Exposed Tools:**

| Tool | Description |
|:-----|:------------|
| `argus_scan` | Full audit of Go source files and SQL migrations against all 30 rules |
| `argus_check_migration` | Instant safety check for raw SQL DDL/DML snippets |
| `argus_explain_rule` | Retrieve documentation and fix patterns for any rule (A01–A30) |
| `argus_report_issue` | Two-phase Human-in-the-Loop (HITL) reporter for false positives & feedback |

> 💡 The `argus_scan` tool description instructs AI models to **automatically invoke it** after writing or modifying database queries — no `.cursorrules` or prompt engineering needed.

### 🔒 Enterprise Privacy & Telemetry Kill-Switch

For corporate, banking, or air-gapped environments where outbound issue reporting must be unconditionally disabled, set `telemetry: false` in `.argus.yaml` or export `ARGUS_TELEMETRY=false`:

```yaml
# .argus.yaml
version: "1"
options:
  telemetry: false # Blocks all outbound issue submission
```

```bash
export ARGUS_TELEMETRY=false
```

📖 **Detailed Guide:** Read the full [Argus MCP Server Specification & Architecture Wiki](https://github.com/will2469/argus/wiki/MCP-Server) for HITL protocols, trust-mode security advisories, and client setup guides.

---

## The 30 Argus Rules Matrix

> 📖 **Full Documentation:** Every rule is thoroughly documented in the [Argus Wiki](https://github.com/will2469/argus/wiki). Click on any rule code or identifier below to open its dedicated specification, failure modes, and code fix examples.

| Rule | Identifier | Severity | Category | Description |
| :--- | :--- | :--- | :--- | :--- |
| [`A01`](https://github.com/will2469/argus/wiki/ARGUS-A01) | [`UNSAFE_SQL_CONCATENATION`](https://github.com/will2469/argus/wiki/ARGUS-A01) | **CRITICAL** | Security (CWE-89) | Forbids raw string concatenation in queries; mandates `$1, $2` bind parameters. |
| [`A02`](https://github.com/will2469/argus/wiki/ARGUS-A02) | [`MISSING_DEFER_CLOSE`](https://github.com/will2469/argus/wiki/ARGUS-A02) | **HIGH** | Reliability | Mandates `defer rows.Close()` immediately after query execution to prevent pool leaks. |
| [`A03`](https://github.com/will2469/argus/wiki/ARGUS-A03) | [`UNBOUNDED_CONTEXT`](https://github.com/will2469/argus/wiki/ARGUS-A03) | **HIGH** | Resilience | Prohibits raw `context.Background()` or `context.TODO()` in query calls. |
| [`A04`](https://github.com/will2469/argus/wiki/ARGUS-A04) | [`UNSAFE_ORDER_BY`](https://github.com/will2469/argus/wiki/ARGUS-A04) | **HIGH** | Security (CWE-89) | Dynamic `ORDER BY` / `GROUP BY` must be validated against compile-time static allowlists. |
| [`A05`](https://github.com/will2469/argus/wiki/ARGUS-A05) | [`AUDIT_LOG_IMMUTABILITY`](https://github.com/will2469/argus/wiki/ARGUS-A05) | **CRITICAL** | Compliance | Prohibits `UPDATE`, `DELETE`, `TRUNCATE`, or `MERGE` on append-only audit ledger tables. |
| [`A06`](https://github.com/will2469/argus/wiki/ARGUS-A06) | [`RUNTIME_DDL`](https://github.com/will2469/argus/wiki/ARGUS-A06) | **CRITICAL** | Security | Blocks DDL execution in application runtime code; runtime roles must be DML-only. |
| [`A07`](https://github.com/will2469/argus/wiki/ARGUS-A07) | [`ERROR_LEAK`](https://github.com/will2469/argus/wiki/ARGUS-A07) | **HIGH** | Privacy (CWE-200) | Forbids leaking raw database error messages or PII details to external clients. |
| [`A08`](https://github.com/will2469/argus/wiki/ARGUS-A08) | [`TX_EXTERNAL_IO`](https://github.com/will2469/argus/wiki/ARGUS-A08) | **HIGH** | Performance | Forbids blocking network/disk I/O (HTTP, gRPC, disk) inside active database transactions. |
| [`A09`](https://github.com/will2469/argus/wiki/ARGUS-A09) | [`ADVISORY_LOCK`](https://github.com/will2469/argus/wiki/ARGUS-A09) | **HIGH** | Concurrency | Mandates transaction-level advisory locks; forbids session locks in pooled connections. |
| [`A10`](https://github.com/will2469/argus/wiki/ARGUS-A10) | [`ISOLATION_LEVEL`](https://github.com/will2469/argus/wiki/ARGUS-A10) | **HIGH** | Integrity | Critical financial/inventory mutations must declare explicit `Serializable` or `FOR UPDATE`. |
| [`A11`](https://github.com/will2469/argus/wiki/ARGUS-A11) | [`DESTRUCTIVE_MIGRATION`](https://github.com/will2469/argus/wiki/ARGUS-A11) | **CRITICAL** | Zero-Downtime | Prohibits destructive DDL (`DROP COLUMN`, `RENAME`) in single releases without expand-contract. |
| [`A12`](https://github.com/will2469/argus/wiki/ARGUS-A12) | [`TIMEOUT_CONFIG`](https://github.com/will2469/argus/wiki/ARGUS-A12) | **HIGH** | Availability | Mandates 4-tier timeout settings (`statement_timeout`, `lock_timeout`, idle timeouts). |
| [`A13`](https://github.com/will2469/argus/wiki/ARGUS-A13) | [`MISSING_DOWN_MIGRATION`](https://github.com/will2469/argus/wiki/ARGUS-A13) | **HIGH** | Rollback Safety | Every `.up.sql` migration must have a non-empty, deterministic symmetric `.down.sql`. |
| [`A14`](https://github.com/will2469/argus/wiki/ARGUS-A14) | [`FORBIDDEN_SELECT_STAR`](https://github.com/will2469/argus/wiki/ARGUS-A14) | **HIGH** | Performance | Prohibits wildcard `SELECT *`; mandates explicit column projection to avoid TOAST bloat. |
| [`A15`](https://github.com/will2469/argus/wiki/ARGUS-A15) | [`FORBIDDEN_DDL_APP_ROLE_GRANT`](https://github.com/will2469/argus/wiki/ARGUS-A15) | **CRITICAL** | Security (CWE-250) | Prohibits granting DDL/ALL privileges or table ownership to application runtime roles. |
| [`A16`](https://github.com/will2469/argus/wiki/ARGUS-A16) | [`MAX_CONNS_CONFIG`](https://github.com/will2469/argus/wiki/ARGUS-A16) | **HIGH** | Scalability | Enforces mathematically bounded `MaxConns` on connection pools to prevent process thrashing. |
| [`A17`](https://github.com/will2469/argus/wiki/ARGUS-A17) | [`FORBIDDEN_QUERY_IN_LOOP`](https://github.com/will2469/argus/wiki/ARGUS-A17) | **HIGH** | Performance | Eliminates N+1 query patterns inside loops in favor of `WHERE id = ANY($1)` or `pgx.Batch`. |
| [`A18`](https://github.com/will2469/argus/wiki/ARGUS-A18) | [`MISSING_ROWS_ERR_CHECK`](https://github.com/will2469/argus/wiki/ARGUS-A18) | **HIGH** | Integrity (CWE-391) | Mandates `rows.Err()` check after `rows.Next()` loops to catch silent network truncations. |
| [`A19`](https://github.com/will2469/argus/wiki/ARGUS-A19) | [`UNBOUNDED_QUERY_LIMIT`](https://github.com/will2469/argus/wiki/ARGUS-A19) | **HIGH** | Resilience (CWE-400) | Queries on high-cardinality tables must have an explicit `LIMIT` or keyset pagination. |
| [`A20`](https://github.com/will2469/argus/wiki/ARGUS-A20) | [`PARAM_LIMIT_65535`](https://github.com/will2469/argus/wiki/ARGUS-A20) | **HIGH** | Protocol Limits | Prevents exceeding PostgreSQL's 65,535 wire parameter ceiling; recommends `pgx.CopyFrom`. |
| [`A21`](https://github.com/will2469/argus/wiki/ARGUS-A21) | [`UNBOUNDED_ROW_LOCK_BLOCKING`](https://github.com/will2469/argus/wiki/ARGUS-A21) | **HIGH** | Concurrency | Queue queries (`SELECT ... FOR UPDATE`) must use `SKIP LOCKED` or `NOWAIT`. |
| [`A22`](https://github.com/will2469/argus/wiki/ARGUS-A22) | [`SERIALIZATION_FAILURE_RETRY`](https://github.com/will2469/argus/wiki/ARGUS-A22) | **HIGH** | Fault Tolerance | `Serializable` transactions must be wrapped in automated retry loops catching SQLSTATE `40001`. |
| [`A23`](https://github.com/will2469/argus/wiki/ARGUS-A23) | [`TRANSACTION_TIMEOUT_CONFIG`](https://github.com/will2469/argus/wiki/ARGUS-A23) | **HIGH** | Modern PG17/18 | Enforces `transaction_timeout` cap on connection pools to prevent XID horizon freezing. |
| [`A24`](https://github.com/will2469/argus/wiki/ARGUS-A24) | [`TENANT_ISOLATION_LEAK`](https://github.com/will2469/argus/wiki/ARGUS-A24) | **CRITICAL** | Multi-Tenancy | Mandates explicit tenant predicates (`WHERE tenant_id = $1`) or verified RLS context. |
| [`A25`](https://github.com/will2469/argus/wiki/ARGUS-A25) | [`EXPENSIVE_CPU_IN_TRANSACTION`](https://github.com/will2469/argus/wiki/ARGUS-A25) | **HIGH** | Performance | Prohibits CPU-heavy tasks (`bcrypt`, `argon2`, RSA keygen, PDF rendering) inside transactions. |
| [`A26`](https://github.com/will2469/argus/wiki/ARGUS-A26) | [`LIKE_WILDCARD_INJECTION`](https://github.com/will2469/argus/wiki/ARGUS-A26) | **HIGH** | Security (CWE-89) | Mandates escaping wildcard characters (`\`, `%`, `_`) on user input bound to `LIKE`/`ILIKE`. |
| [`A27`](https://github.com/will2469/argus/wiki/ARGUS-A27) | [`NON_CONCURRENT_INDEX_CREATION`](https://github.com/will2469/argus/wiki/ARGUS-A27) | **CRITICAL** | Zero-Downtime | Indexes on existing tables must use `CREATE INDEX CONCURRENTLY` to avoid write lockouts. |
| [`A28`](https://github.com/will2469/argus/wiki/ARGUS-A28) | [`TABLE_LOCKING_CONSTRAINT_ADDITION`](https://github.com/will2469/argus/wiki/ARGUS-A28) | **CRITICAL** | Zero-Downtime | FK and CHECK constraints must use 2-phase `NOT VALID` followed by `VALIDATE CONSTRAINT`. |
| [`A29`](https://github.com/will2469/argus/wiki/ARGUS-A29) | [`UNINDEXED_FOREIGN_KEY`](https://github.com/will2469/argus/wiki/ARGUS-A29) | **HIGH** | Performance | Every foreign key on child tables must have a supporting B-tree index (anti-table scan). |
| [`A30`](https://github.com/will2469/argus/wiki/ARGUS-A30) | [`TIMESTAMP_WITHOUT_TIMEZONE`](https://github.com/will2469/argus/wiki/ARGUS-A30) | **CRITICAL** | Temporal Hygiene | Prohibits bare `TIMESTAMP`; mandates `TIMESTAMPTZ` (UTC-normalized) for temporal determinism. |

---

## Suppressing Findings (Ignore Directives)

When a rule violation is deliberate and reviewed, suppress it with an inline directive:

```go
// In Go code:
// argus:ignore ARGUS-A14 export worker requires full row dump
rows, err := pool.Query(ctx, "SELECT * FROM historical_archive")
```

```sql
-- In SQL migrations:
-- argus:ignore ARGUS-A29 static dictionary table with under 50 rows never deleted
status_code VARCHAR(20) REFERENCES ref_status(code)
```

Directives require an explanatory reason of at least two words.

---

## Configuration (`.argus.yaml`)

Initialize a `.argus.yaml` file in your repository root to configure project-specific settings:

```yaml
version: "1"

rules:
  ARGUS-A14:
    enabled: true
  ARGUS-A16:
    enabled: true
    max_conns_limit: 50
  ARGUS-A19:
    enabled: true
    default_max_limit: 1000
    high_growth_tables:
      - "orders"
      - "audit_logs"
  ARGUS-A24:
    enabled: true
    tenant_column: "tenant_id"
    tenant_tables:
      - "customers"
      - "invoices"
```

---

## GitHub Actions CI Integration

```yaml
name: Database Safety Audit
on: [push, pull_request]

jobs:
  argus:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.24"
      - name: Install Argus
        run: go install github.com/will2469/argus/cmd/argus@latest
      - name: Audit Go Packages & Migrations
        run: argus --no-report --dirs=. --migrations=migrations
```

---

## License

Argus is open-source software licensed under the [MIT License](LICENSE).
