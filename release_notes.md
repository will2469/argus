## Argus v1.0.0 — Production-Grade Go & PostgreSQL 18 Static Analyzer

We are proud to announce the first official public release of **Argus (v1.0.0)**! 

Argus is an advanced compile-time static analyzer and pre-commit database safety linter for Go applications and PostgreSQL migrations. Built on top of the official Go analysis framework (`go/analysis`) and PostgreSQL's native C query parser (`libpg_query` via `pg_query_go`), Argus bridges the gap between Go application code and PostgreSQL engine internals.

---

### 📦 Quick Installation

#### Linux & macOS (One-Line Installer)
```bash
curl -fsSL https://raw.githubusercontent.com/will2469/argus/main/install.sh | bash
```

#### Windows (PowerShell)
```powershell
irm https://raw.githubusercontent.com/will2469/argus/main/install.ps1 | iex
```

#### Go Toolchain
```bash
go install github.com/will2469/argus/cmd/argus@latest
```

---

### 🛡️ 30 Production-Grade Database Invariants

Every rule is thoroughly documented in the official [Argus Wiki](https://github.com/will2469/argus/wiki). Click on any rule below to open its dedicated specification, failure modes, and code fix examples:

#### 🔒 Security & Data Integrity
* [`ARGUS-A01`](https://github.com/will2469/argus/wiki/ARGUS-A01) — **UNSAFE_SQL_CONCATENATION**: Forbids raw string concatenation in queries; mandates `$1, $2` bind parameters.
* [`ARGUS-A05`](https://github.com/will2469/argus/wiki/ARGUS-A05) — **AUDIT_LOG_IMMUTABILITY**: Enforces append-only immutable audit logs (prohibits `UPDATE`, `DELETE`, `TRUNCATE`, `MERGE`).
* [`ARGUS-A06`](https://github.com/will2469/argus/wiki/ARGUS-A06) — **RUNTIME_DDL**: Blocks DDL execution in application runtime code; runtime roles must be DML-only.
* [`ARGUS-A07`](https://github.com/will2469/argus/wiki/ARGUS-A07) — **ERROR_LEAK**: Prevents leaking raw internal database error strings or PII to external clients.
* [`ARGUS-A15`](https://github.com/will2469/argus/wiki/ARGUS-A15) — **FORBIDDEN_DDL_APP_ROLE_GRANT**: Blocks runtime application roles from receiving DDL privileges or table ownership.
* [`ARGUS-A18`](https://github.com/will2469/argus/wiki/ARGUS-A18) — **MISSING_ROWS_ERR_CHECK**: Mandates `rows.Err()` check after `rows.Next()` loops to catch silent network truncations.
* [`ARGUS-A24`](https://github.com/will2469/argus/wiki/ARGUS-A24) — **TENANT_ISOLATION_LEAK**: Mandates explicit tenant isolation filter checks on multi-tenant tables.
* [`ARGUS-A26`](https://github.com/will2469/argus/wiki/ARGUS-A26) — **LIKE_WILDCARD_INJECTION**: Enforces explicit escaping of SQL wildcards (`%`, `_`, `\`) in LIKE queries.

#### ⚡ Resource & Connection Pool Lifecycle
* [`ARGUS-A02`](https://github.com/will2469/argus/wiki/ARGUS-A02) — **MISSING_DEFER_CLOSE**: Mandates `defer rows.Close()` immediately after query execution to prevent pool leaks.
* [`ARGUS-A03`](https://github.com/will2469/argus/wiki/ARGUS-A03) — **UNBOUNDED_CONTEXT**: Prohibits raw `context.Background()` or `context.TODO()` in query calls.
* [`ARGUS-A08`](https://github.com/will2469/argus/wiki/ARGUS-A08) — **TX_EXTERNAL_IO**: Forbids blocking network/disk I/O (HTTP, gRPC, disk) inside active database transactions.
* [`ARGUS-A12`](https://github.com/will2469/argus/wiki/ARGUS-A12) — **TIMEOUT_CONFIG**: Mandates 4-tier timeout settings (`statement_timeout`, `lock_timeout`, idle timeouts).
* [`ARGUS-A16`](https://github.com/will2469/argus/wiki/ARGUS-A16) — **MAX_CONNS_CONFIG**: Enforces mathematically bounded `MaxConns` on connection pools to prevent process thrashing.
* [`ARGUS-A23`](https://github.com/will2469/argus/wiki/ARGUS-A23) — **TRANSACTION_TIMEOUT_CONFIG**: Enforces `transaction_timeout` cap on connection pools to prevent XID horizon freezing.
* [`ARGUS-A25`](https://github.com/will2469/argus/wiki/ARGUS-A25) — **EXPENSIVE_CPU_IN_TRANSACTION**: Prohibits CPU-heavy tasks (`bcrypt`, `argon2`, RSA keygen) inside transactions.

#### 🚀 Performance & Concurrency
* [`ARGUS-A04`](https://github.com/will2469/argus/wiki/ARGUS-A04) — **UNSAFE_ORDER_BY**: Dynamic `ORDER BY` / `GROUP BY` must be validated against compile-time static allowlists.
* [`ARGUS-A09`](https://github.com/will2469/argus/wiki/ARGUS-A09) — **ADVISORY_LOCK**: Mandates transaction-level advisory locks; forbids session locks in pooled connections.
* [`ARGUS-A10`](https://github.com/will2469/argus/wiki/ARGUS-A10) — **ISOLATION_LEVEL**: Critical financial/inventory mutations must declare explicit `Serializable` or `FOR UPDATE`.
* [`ARGUS-A14`](https://github.com/will2469/argus/wiki/ARGUS-A14) — **FORBIDDEN_SELECT_STAR**: Prohibits wildcard `SELECT *`; mandates explicit column projection to avoid TOAST bloat.
* [`ARGUS-A17`](https://github.com/will2469/argus/wiki/ARGUS-A17) — **FORBIDDEN_QUERY_IN_LOOP**: Eliminates N+1 query patterns inside loops in favor of `WHERE id = ANY($1)` or `pgx.Batch`.
* [`ARGUS-A19`](https://github.com/will2469/argus/wiki/ARGUS-A19) — **UNBOUNDED_QUERY_LIMIT**: Queries on high-cardinality tables must have an explicit `LIMIT` or keyset pagination.
* [`ARGUS-A20`](https://github.com/will2469/argus/wiki/ARGUS-A20) — **PARAM_LIMIT_65535**: Prevents exceeding PostgreSQL's 65,535 wire parameter ceiling; recommends `pgx.CopyFrom`.
* [`ARGUS-A21`](https://github.com/will2469/argus/wiki/ARGUS-A21) — **UNBOUNDED_ROW_LOCK_BLOCKING**: Queue queries (`SELECT ... FOR UPDATE`) must use `SKIP LOCKED` or `NOWAIT`.
* [`ARGUS-A22`](https://github.com/will2469/argus/wiki/ARGUS-A22) — **SERIALIZATION_FAILURE_RETRY**: `Serializable` transactions must be wrapped in automated retry loops catching SQLSTATE `40001`.

#### 🔄 Zero-Downtime Migration Hygiene
* [`ARGUS-A11`](https://github.com/will2469/argus/wiki/ARGUS-A11) — **DESTRUCTIVE_MIGRATION**: Prohibits destructive DDL (`DROP COLUMN`, `RENAME`) in single releases without expand-contract.
* [`ARGUS-A13`](https://github.com/will2469/argus/wiki/ARGUS-A13) — **MISSING_DOWN_MIGRATION**: Every `.up.sql` migration must have a non-empty, deterministic symmetric `.down.sql`.
* [`ARGUS-A27`](https://github.com/will2469/argus/wiki/ARGUS-A27) — **NON_CONCURRENT_INDEX_CREATION**: Indexes on existing tables must use `CREATE INDEX CONCURRENTLY` to avoid write lockouts.
* [`ARGUS-A28`](https://github.com/will2469/argus/wiki/ARGUS-A28) — **TABLE_LOCKING_CONSTRAINT_ADDITION**: FK and CHECK constraints must use 2-phase `NOT VALID` followed by `VALIDATE CONSTRAINT`.
* [`ARGUS-A29`](https://github.com/will2469/argus/wiki/ARGUS-A29) — **UNINDEXED_FOREIGN_KEY**: Every foreign key on child tables must have a supporting B-tree index (anti-table scan).
* [`ARGUS-A30`](https://github.com/will2469/argus/wiki/ARGUS-A30) — **TIMESTAMP_WITHOUT_TIMEZONE**: Prohibits bare `TIMESTAMP`; mandates `TIMESTAMPTZ` (UTC-normalized) for temporal determinism.

---

### 💻 Basic Usage

```bash
# 1. Run static analysis across Go codebase and SQL migrations
argus --dirs=. --migrations=migrations

# 2. Generate comprehensive markdown audit report
argus --output=argus-report.md

# 3. Integrate directly with go vet
go vet -vettool=$(which argus) ./...
```
