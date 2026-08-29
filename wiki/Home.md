# Argus (Ἄργος Πανόπτης) Documentation & Rule Reference

> _"In Greek mythology, **Argus Panoptes** (Argos the All-Seeing) was a hundred-eyed giant endowed with perpetual vigilance. Because only a few of his eyes ever slept at any given moment while the rest remained wide open, Argus served as the supreme, infallible watchman entrusted to guard sacred treasures and maintain uninterrupted observation."_

Welcome to the official **Argus Static Analyzer Wiki**.

Like the hundred-eyed guardian of mythology, **Argus** serves as an infallible compile-time watchman for Go applications and PostgreSQL database migrations. By keeping a vigilant eye across every query site, connection lifecycle, lock hierarchy, and schema migration, Argus ensures that database invariants remain unbroken and production databases stay resilient, consistent, and secure.

---

## Rule Reference Directory

All 30 inspection rules are categorized by architectural concerns:

### Security & Data Integrity

| Rule Code                   | Identifier                     | Severity     | Description                                                            | Default   |
| :-------------------------- | :----------------------------- | :----------- | :--------------------------------------------------------------------- | :-------- |
| [`ARGUS-A01`](ARGUS-A01.md) | `UNSAFE_SQL_CONCATENATION`     | **CRITICAL** | Prohibits raw runtime string concatenation in SQL queries              | `enabled` |
| [`ARGUS-A05`](ARGUS-A05.md) | `AUDIT_LOG_IMMUTABILITY`       | **CRITICAL** | Enforces append-only immutable audit logs (no UPDATE/DELETE/TRUNCATE)  | `enabled` |
| [`ARGUS-A06`](ARGUS-A06.md) | `RUNTIME_DDL`                  | **CRITICAL** | Forbids runtime application code from executing DDL statements         | `enabled` |
| [`ARGUS-A07`](ARGUS-A07.md) | `ERROR_LEAK`                   | **HIGH**     | Prevents internal database error strings leaking to API responses      | `enabled` |
| [`ARGUS-A15`](ARGUS-A15.md) | `FORBIDDEN_DDL_APP_ROLE_GRANT` | **CRITICAL** | Blocks runtime application roles from receiving DDL privileges         | `enabled` |
| [`ARGUS-A18`](ARGUS-A18.md) | `MISSING_ROWS_ERR_CHECK`       | **HIGH**     | Enforces mandatory `rows.Err()` checks immediately after cursor loop   | `enabled` |
| [`ARGUS-A24`](ARGUS-A24.md) | `TENANT_ISOLATION_LEAK`        | **CRITICAL** | Mandates tenant isolation filter checks on multi-tenant tables         | `enabled` |
| [`ARGUS-A26`](ARGUS-A26.md) | `LIKE_WILDCARD_INJECTION`      | **HIGH**     | Enforces explicit escaping of SQL wildcards (%, _, \\) in LIKE queries | `enabled` |

### Resource & Connection Lifecycle

| Rule Code                   | Identifier                     | Severity | Description                                                                | Default   |
| :-------------------------- | :----------------------------- | :------- | :------------------------------------------------------------------------- | :-------- |
| [`ARGUS-A02`](ARGUS-A02.md) | `MISSING_DEFER_CLOSE`          | **HIGH** | Ensures database query rows are properly closed via `defer rows.Close()`   | `enabled` |
| [`ARGUS-A03`](ARGUS-A03.md) | `UNBOUNDED_CONTEXT`            | **HIGH** | Disallows unbounded `context.Background()` or `context.TODO()` in DB calls | `enabled` |
| [`ARGUS-A08`](ARGUS-A08.md) | `TX_EXTERNAL_IO`               | **HIGH** | Detects external network I/O executed inside active database transactions  | `enabled` |
| [`ARGUS-A12`](ARGUS-A12.md) | `TIMEOUT_CONFIG`               | **HIGH** | Mandates explicit timeout configuration for database pools                 | `enabled` |
| [`ARGUS-A16`](ARGUS-A16.md) | `MAX_CONNS_CONFIG`             | **HIGH** | Requires safe maximum pool connection limits to prevent exhaustion         | `enabled` |
| [`ARGUS-A25`](ARGUS-A25.md) | `EXPENSIVE_CPU_IN_TRANSACTION` | **HIGH** | Prohibits CPU-expensive operations (bcrypt, argon2, RSA) in transactions   | `enabled` |

### Performance & Concurrency

| Rule Code                   | Identifier                    | Severity   | Description                                                                        | Default   |
| :-------------------------- | :---------------------------- | :--------- | :--------------------------------------------------------------------------------- | :-------- |
| [`ARGUS-A04`](ARGUS-A04.md) | `UNSAFE_ORDER_BY`             | **HIGH**   | Enforces closed-set allowlists or switch-case mapping for dynamic `ORDER BY`       | `enabled` |
| [`ARGUS-A09`](ARGUS-A09.md) | `ADVISORY_LOCK`               | **MEDIUM** | Enforces proper acquisition and release pairing of PostgreSQL advisory locks       | `enabled` |
| [`ARGUS-A10`](ARGUS-A10.md) | `ISOLATION_LEVEL`             | **MEDIUM** | Validates appropriate transaction isolation levels                                 | `enabled` |
| [`ARGUS-A14`](ARGUS-A14.md) | `FORBIDDEN_SELECT_STAR`       | **HIGH**   | Forbids `SELECT *` over-fetching to minimize network and memory bloat              | `enabled` |
| [`ARGUS-A17`](ARGUS-A17.md) | `FORBIDDEN_QUERY_IN_LOOP`     | **HIGH**   | Detects N+1 query antipatterns inside loop bodies                                  | `enabled` |
| [`ARGUS-A19`](ARGUS-A19.md) | `UNBOUNDED_QUERY_LIMIT`       | **HIGH**   | Mandates explicit `LIMIT` or keyset pagination on high-cardinality tables          | `enabled` |
| [`ARGUS-A20`](ARGUS-A20.md) | `PARAM_LIMIT_65535`           | **HIGH**   | Prevents exceeding 65,535 wire parameter limits in dynamic multi-row batching      | `enabled` |
| [`ARGUS-A21`](ARGUS-A21.md) | `UNBOUNDED_ROW_LOCK_BLOCKING` | **HIGH**   | Mandates `SKIP LOCKED` or `NOWAIT` on multi-row `FOR UPDATE` queue queries         | `enabled` |
| [`ARGUS-A22`](ARGUS-A22.md) | `SERIALIZATION_FAILURE_RETRY` | **HIGH**   | Mandates automatic retry loops on `Serializable` and `RepeatableRead` transactions | `enabled` |
| [`ARGUS-A23`](ARGUS-A23.md) | `TRANSACTION_TIMEOUT_CONFIG`  | **HIGH**   | Enforces explicit `transaction_timeout` GUC configuration for PG 17/18+ targets    | `enabled` |

### Schema & Migration Safety

| Rule Code                   | Identifier                          | Severity     | Description                                                               | Default   |
| :-------------------------- | :---------------------------------- | :----------- | :------------------------------------------------------------------------ | :-------- |
| [`ARGUS-A11`](ARGUS-A11.md) | `DESTRUCTIVE_MIGRATION`             | **CRITICAL** | Blocks destructive schema operations (DROP COLUMN, DROP TABLE) in .up.sql | `enabled` |
| [`ARGUS-A13`](ARGUS-A13.md) | `MISSING_DOWN_MIGRATION`            | **HIGH**     | Enforces that every .up.sql migration has a corresponding valid .down.sql | `enabled` |
| [`ARGUS-A27`](ARGUS-A27.md) | `NON_CONCURRENT_INDEX_CREATION`     | **CRITICAL** | Requires `CREATE INDEX CONCURRENTLY` on existing tables                   | `enabled` |
| [`ARGUS-A28`](ARGUS-A28.md) | `TABLE_LOCKING_CONSTRAINT_ADDITION` | **CRITICAL** | Prohibits exclusive table-locking constraint additions in migrations      | `enabled` |
| [`ARGUS-A29`](ARGUS-A29.md) | `UNINDEXED_FOREIGN_KEY`             | **HIGH**     | Detects foreign keys lacking supporting indexes                           | `enabled` |
| [`ARGUS-A30`](ARGUS-A30.md) | `TIMESTAMP_WITHOUT_TIMEZONE`        | **CRITICAL** | Enforces `TIMESTAMPTZ` instead of bare `TIMESTAMP` in column definitions  | `enabled` |

---

## Suppression & Ignore Directives

Argus supports granular per-line ignore directives:

```go
// argus:ignore <RULE_CODE> <mandatory reason with at least 2 words>
```

Wildcard suppression:

```go
// argus:ignore ALL <reason>
// argus:ignore * <reason>
```

---

## CLI & Runner Modes

Argus Checker operates in dual-mode architecture:

### 1. Dual-Mode Execution

| Mode                  | Command                                        | Use Case                                                       |
| :-------------------- | :--------------------------------------------- | :------------------------------------------------------------- |
| **Go Vet Tool**       | `go vet -vettool=$(which argus-checker) ./...` | Standard compiler vettool integration & CI type-checked passes |
| **Standalone Runner** | `argus-checker [options] [directories...]`     | Comprehensive static analysis & SQL migration scanner          |

### 2. Standalone CLI Flags

| Flag                   | Description                                                                            | Default / Example                                               |
| :--------------------- | :------------------------------------------------------------------------------------- | :-------------------------------------------------------------- |
| `--no-report`          | Suppresses markdown report file creation; runs in-memory and outputs exit code `0`/`1` | Ideal for Git pre-commit hooks & fast CI checks                 |
| `--output=<path.md>`   | Path where the generated markdown audit report will be saved                           | Overrides `report_file` from `.argus.yaml`                      |
| `<path.md>`            | Positional path argument to output markdown report                                     | `argus-checker argus-report.md`                                 |
| `--dirs=<d1,d2>`       | Comma-separated list of Go directories or files to inspect                             | `--dirs=.,cmd,pkg`                                              |
| `--migrations=<d1,d2>` | Comma-separated list of SQL migration directories                                      | `--migrations=migrations`                                       |
| `-h`, `--help`         | Display usage help and available options                                               | `argus-checker --help`                                          |

### 3. Configuration via `.argus.yaml`

Argus automatically searches for `.argus.yaml` from the current working directory up to the repository root:

```yaml
version: "1"

options:
  report_format: "markdown" # "text" | "json" | "markdown"
  report_file: "argus-report.md" # Default report file destination
  fail_on: "HIGH" # "CRITICAL" | "HIGH" | "MEDIUM" | "LOW"
  scan_dirs:
    - "."
  migration_dirs:
    - "migrations"
```

- When `report_file` is defined in `.argus.yaml`, running `argus-checker` automatically generates the markdown report.
- Passing `--no-report` explicitly disables file generation, keeping git working trees clean during automated hook executions.
