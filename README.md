# Argus: Production-Grade Go & PostgreSQL 18 Static Analyzer

[![Go Reference](https://pkg.go.dev/badge/github.com/will2469/argus.svg)](https://pkg.go.dev/github.com/will2469/argus)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![PostgreSQL 18 Ready](https://img.shields.io/badge/PostgreSQL-18%2B%20Ready-blue.svg)](https://www.postgresql.org/)

**Argus** is an advanced compile-time static analyzer and pre-commit database safety linter for Go applications and PostgreSQL migrations. Built on top of the official Go analysis framework (`go/analysis`) and PostgreSQL's native C query parser (`libpg_query` via `pg_query_go`), Argus bridges the gap between Go application code and PostgreSQL engine internals.

It enforces **30 production-grade database invariants**, eliminating SQL injections, N+1 query latency collapses, connection pool starvation, cross-tenant data leaks, and catastrophic production table locking during zero-downtime schema migrations.

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

*Or download pre-compiled binaries directly from [GitHub Releases](https://github.com/will2469/argus/releases).*

### Basic Usage

```bash
# 1. Run static analysis across current directory and migrations
argus --dirs=. --migrations=migrations

# 2. Generate a comprehensive markdown audit report
argus --output=argus-report.md

# 3. Integrate directly into go vet
go vet -vettool=$(which argus) ./...
```

---

## The 30 Argus Rules Matrix

| Rule  | Identifier                          | Severity     | Category             | Description                                                                                     |
| :---- | :---------------------------------- | :----------- | :------------------- | :---------------------------------------------------------------------------------------------- |
| `A01` | `UNSAFE_SQL_CONCATENATION`          | **CRITICAL** | Security (CWE-89)    | Forbids raw string concatenation in queries; mandates `$1, $2` bind parameters.                 |
| `A02` | `MISSING_DEFER_CLOSE`               | **HIGH**     | Reliability          | Mandates `defer rows.Close()` immediately after query execution to prevent pool leaks.          |
| `A03` | `UNBOUNDED_CONTEXT`                 | **HIGH**     | Resilience           | Prohibits raw `context.Background()` or `context.TODO()` in query calls.                        |
| `A04` | `UNSAFE_ORDER_BY`                   | **HIGH**     | Security (CWE-89)    | Dynamic `ORDER BY` / `GROUP BY` must be validated against compile-time static allowlists.       |
| `A05` | `AUDIT_LOG_IMMUTABILITY`            | **CRITICAL** | Compliance           | Prohibits `UPDATE`, `DELETE`, `TRUNCATE`, or `MERGE` on append-only audit ledger tables.        |
| `A06` | `RUNTIME_DDL`                       | **CRITICAL** | Security             | Blocks DDL execution in application runtime code; runtime roles must be DML-only.               |
| `A07` | `ERROR_LEAK`                        | **HIGH**     | Privacy (CWE-200)    | Forbids leaking raw database error messages or PII details to external clients.                 |
| `A08` | `TX_EXTERNAL_IO`                    | **HIGH**     | Performance          | Forbids blocking network/disk I/O (HTTP, gRPC, disk) inside active database transactions.       |
| `A09` | `ADVISORY_LOCK`                     | **HIGH**     | Concurrency          | Mandates transaction-level advisory locks; forbids session locks in pooled connections.         |
| `A10` | `ISOLATION_LEVEL`                   | **HIGH**     | Integrity            | Critical financial/inventory mutations must declare explicit `Serializable` or `FOR UPDATE`.    |
| `A11` | `DESTRUCTIVE_MIGRATION`             | **CRITICAL** | Zero-Downtime        | Prohibits destructive DDL (`DROP COLUMN`, `RENAME`) in single releases without expand-contract. |
| `A12` | `TIMEOUT_CONFIG`                    | **HIGH**     | Availability         | Mandates 4-tier timeout settings (`statement_timeout`, `lock_timeout`, idle timeouts).          |
| `A13` | `MISSING_DOWN_MIGRATION`            | **HIGH**     | Rollback Safety      | Every `.up.sql` migration must have a non-empty, deterministic symmetric `.down.sql`.           |
| `A14` | `FORBIDDEN_SELECT_STAR`             | **HIGH**     | Performance          | Prohibits wildcard `SELECT *`; mandates explicit column projection to avoid TOAST bloat.        |
| `A15` | `FORBIDDEN_DDL_APP_ROLE_GRANT`      | **CRITICAL** | Security (CWE-250)   | Prohibits granting DDL/ALL privileges or table ownership to application runtime roles.          |
| `A16` | `MAX_CONNS_CONFIG`                  | **HIGH**     | Scalability          | Enforces mathematically bounded `MaxConns` on connection pools to prevent process thrashing.    |
| `A17` | `FORBIDDEN_QUERY_IN_LOOP`           | **HIGH**     | Performance          | Eliminates N+1 query patterns inside loops in favor of `WHERE id = ANY($1)` or `pgx.Batch`.     |
| `A18` | `MISSING_ROWS_ERR_CHECK`            | **HIGH**     | Integrity (CWE-391)  | Mandates `rows.Err()` check after `rows.Next()` loops to catch silent network truncations.      |
| `A19` | `UNBOUNDED_QUERY_LIMIT`             | **HIGH**     | Resilience (CWE-400) | Queries on high-cardinality tables must have an explicit `LIMIT` or keyset pagination.          |
| `A20` | `PARAM_LIMIT_65535`                 | **HIGH**     | Protocol Limits      | Prevents exceeding PostgreSQL's 65,535 wire parameter ceiling; recommends `pgx.CopyFrom`.       |
| `A21` | `UNBOUNDED_ROW_LOCK_BLOCKING`       | **HIGH**     | Concurrency          | Queue queries (`SELECT ... FOR UPDATE`) must use `SKIP LOCKED` or `NOWAIT`.                     |
| `A22` | `SERIALIZATION_FAILURE_RETRY`       | **HIGH**     | Fault Tolerance      | `Serializable` transactions must be wrapped in automated retry loops catching SQLSTATE `40001`. |
| `A23` | `TRANSACTION_TIMEOUT_CONFIG`        | **HIGH**     | Modern PG17/18       | Enforces `transaction_timeout` cap on connection pools to prevent XID horizon freezing.         |
| `A24` | `TENANT_ISOLATION_LEAK`             | **CRITICAL** | Multi-Tenancy        | Mandates explicit tenant predicates (`WHERE tenant_id = $1`) or verified RLS context.           |
| `A25` | `EXPENSIVE_CPU_IN_TRANSACTION`      | **HIGH**     | Performance          | Prohibits CPU-heavy tasks (`bcrypt`, `argon2`, RSA keygen, PDF rendering) inside transactions.  |
| `A26` | `LIKE_WILDCARD_INJECTION`           | **HIGH**     | Security (CWE-89)    | Mandates escaping wildcard characters (`\`, `%`, `_`) on user input bound to `LIKE`/`ILIKE`.    |
| `A27` | `NON_CONCURRENT_INDEX_CREATION`     | **CRITICAL** | Zero-Downtime        | Indexes on existing tables must use `CREATE INDEX CONCURRENTLY` to avoid write lockouts.        |
| `A28` | `TABLE_LOCKING_CONSTRAINT_ADDITION` | **CRITICAL** | Zero-Downtime        | FK and CHECK constraints must use 2-phase `NOT VALID` followed by `VALIDATE CONSTRAINT`.        |
| `A29` | `UNINDEXED_FOREIGN_KEY`             | **HIGH**     | Performance          | Every foreign key on child tables must have a supporting B-tree index (anti-table scan).        |
| `A30` | `TIMESTAMP_WITHOUT_TIMEZONE`        | **CRITICAL** | Temporal Hygiene     | Prohibits bare `TIMESTAMP`; mandates `TIMESTAMPTZ` (UTC-normalized) for temporal determinism.   |

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

```

```
