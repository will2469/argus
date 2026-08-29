# ARGUS-A12: Database Connection Pool Timeout Configurations

> **Rule Code:** `ARGUS-A12`
> **Identifier:** `TIMEOUT_CONFIG`
> **Severity:** `HIGH / CRITICAL` (Runaway Query & Connection Starvation Blocker)
> **Category:** `Operational Stability, Resource Bounds & DoS Prevention`
> **Target Standards:** CWE-400 (Uncontrolled Resource Consumption), OWASP ASVS v4.0.3/v5.0 §V1.4.3, §V11.1.4

---

## 1. Overview & Core Invariant

Every PostgreSQL connection pool initialization (`pgxpool.Config`, `pgxpool.ParseConfig`, or DSN connection string) **must explicitly configure server-side and client-side timeout boundaries**, including:

1. **`statement_timeout`**: Cancels individual queries exceeding safe execution limits (e.g. 5,000-15,000 ms).
2. **`lock_timeout`**: Cancels queries waiting in lock acquisition queues beyond a strict threshold (e.g. 2,000-5,000 ms).
3. **`idle_in_transaction_session_timeout`**: Terminates connections abandoned while holding an open transaction (e.g. 10,000-15,000 ms).
4. **`MaxConnIdleTime` & `MaxConnLifetime`**: Enforces driver-side connection cycling to prevent memory bloat.

Timeout parameters must not be omitted or explicitly disabled (`0` / unlimited).

---

## 2. Technical Grounding & PostgreSQL Engine Realities

### 2.1. Why Server-Level `postgresql.conf` is Insufficient for OLTP

Server-wide timeout defaults in `postgresql.conf` are frequently configured loosely (or disabled at `0`) to accommodate administrative tasks, batch ETL, or long-running database migrations. Application connection pools serving user traffic must not inherit loose timeouts: a single runaway query can exhaust connection pool slots and cascade into a cluster freeze.

### 2.2. The Role of `lock_timeout` in Preventing Starvation

When a transaction attempts to modify a row locked by another worker, it enters an unbuffered wait queue in PostgreSQL shared memory. Without `lock_timeout`, the blocked query **hangs indefinitely** until `statement_timeout` expires or the pool runs out of connections. `lock_timeout` forces a fast failure (2-3 seconds), allowing the application to retry or return a human-friendly response.

### 2.3. Autovacuum Protection (`idle_in_transaction_session_timeout`)

If an application worker crashes or blocks on external I/O inside an open transaction, the connection remains `idle in transaction`. This state **pins the PostgreSQL global `xmin` transaction horizon**, preventing autovacuum from reclaiming dead tuples cluster-wide and causing catastrophic disk bloat (CWE-400).

```mermaid
flowchart TD
    subgraph HIERARCHY ["4-Tier PostgreSQL Timeout Hierarchy"]
        direction TB
        L["1. lock_timeout (e.g. 2s - 5s)<br/>Fails fast if blocked in row/table lock queue"]
        S["2. statement_timeout (e.g. 5s - 15s)<br/>Aborts query execution if compute exceeds threshold"]
        I["3. idle_in_transaction_session_timeout (e.g. 10s - 15s)<br/>Terminates abandoned open transactions & protects autovacuum xmin"]
        T["4. transaction_timeout (e.g. 30s) [PostgreSQL 17+]<br/>Cumulative timeout from BEGIN to COMMIT"]
        L --> S --> I --> T
    end

    subgraph DSN ["DSN Connection String Example"]
        direction TB
        DSN_STR["postgres://user:pass@host:5432/db?<br/>statement_timeout=5000&<br/>lock_timeout=2000&<br/>idle_in_transaction_session_timeout=10000"]
    end
```

---

## 3. How Argus Detects Violations (Static Analysis Architecture)

Argus inspects connection pool initialization via DSN query parameters and Go struct assignment flows:

```mermaid
flowchart LR
    Scan["Inspect Pool Init Calls<br/>(Exclude _test.go)"] --> CallType{"pgxpool.New or<br/>pgxpool.NewWithConfig?"}
    CallType -->|pgxpool.New| CheckDSN["dsn_checker.go:<br/>Parse URL Query / KV Params"]
    CallType -->|pgxpool.NewWithConfig| CheckFlow["config_flow.go:<br/>Track AST Assignments on Config Var"]
    CheckDSN --> DSNMissing{"Missing statement_timeout,<br/>lock_timeout, or idle timeout?"}
    CheckFlow --> StructMissing{"Missing RuntimeParams or<br/>MaxConnIdleTime / Lifetime?"}
    DSNMissing -->|Yes| ReportDSN["Report HIGH Violation:<br/>Missing DSN Timeout Parameters"]
    StructMissing -->|Yes| ReportStruct["Report HIGH Violation:<br/>Incomplete pgxpool.Config Timeouts"]
    DSNMissing -->|No| Pass["Pass (Complete Timeout Config)"]
    StructMissing -->|No| Pass
```

1. **DSN Parameter Checker (`dsn_checker.go`):** Parses URL query parameters and key-value connection strings for `statement_timeout`, `lock_timeout`, and `idle_in_transaction_session_timeout`.
2. **Configuration Flow Evaluator (`config_flow.go`):** Tracks `pgxpool.Config` composite literals, struct assignments (`AssignStmt`), and helper initializers (`configurePostgresPool`).

---

## 4. Vulnerability & Risk Taxonomy

| Failure Mode                      | Technical Impact                                                                        | Risk Severity |
| :-------------------------------- | :-------------------------------------------------------------------------------------- | :------------ |
| **Missing `statement_timeout`**   | Runaway complex queries consume CPU/memory and hang connection slots indefinitely.      | **CRITICAL**  |
| **Missing `lock_timeout`**        | Lock contention starves connection pool workers, blocking incoming traffic.             | **HIGH**      |
| **Missing `idle_in_transaction`** | Rogue open transactions stall autovacuum `xmin` horizon, triggering severe table bloat. | **CRITICAL**  |
| **Zero Timeout (`"0"`)**          | Explicitly disables timeout protection, exposing server to denial of service.           | **HIGH**      |

---

## 5. Non-Compliant Code Patterns (Bad Examples)

### Example 1: Plain DSN Without Timeouts

```go
// VIOLATION: DSN missing statement_timeout, lock_timeout, and idle_in_transaction
func InitPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
    // Flagged: pgxpool DSN missing required timeout parameters
    return pgxpool.New(ctx, "postgres://app:secret@localhost:5432/app_db")
}
```

### Example 2: Incomplete `pgxpool.Config`

```go
// VIOLATION: Config missing lock_timeout and MaxConnLifetime
func InitCustomPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
    cfg, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return nil, err
    }
    // Flagged: missing lock_timeout and MaxConnLifetime
    cfg.ConnConfig.RuntimeParams = map[string]string{
        "statement_timeout": "5000",
    }
    cfg.MaxConnIdleTime = 5 * time.Minute
    return pgxpool.NewWithConfig(ctx, cfg)
}
```

---

## 6. Compliant Implementation Patterns (Good Examples)

### Solution 1: Fully Configured DSN

```go
// COMPLIANT: All required timeout parameters present in connection string
const dsn = "postgres://app:secret@localhost:5432/app_db?" +
    "statement_timeout=5000&" +
    "lock_timeout=2000&" +
    "idle_in_transaction_session_timeout=10000"

func InitPool(ctx context.Context) (*pgxpool.Pool, error) {
    return pgxpool.New(ctx, dsn)
}
```

### Solution 2: Explicit `pgxpool.Config` with Helper

```go
// COMPLIANT: Complete RuntimeParams and pool lifecycle configuration
func NewDatabasePool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
    cfg, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return nil, err
    }

    cfg.ConnConfig.RuntimeParams = map[string]string{
        "statement_timeout":                   "10000",
        "lock_timeout":                        "3000",
        "idle_in_transaction_session_timeout": "15000",
    }
    cfg.MaxConnIdleTime = 5 * time.Minute
    cfg.MaxConnLifetime = 1 * time.Hour
    cfg.MaxConns = 25

    return pgxpool.NewWithConfig(ctx, cfg)
}
```

---

## 7. How to Suppress (Ignore Directives)

For offline database maintenance utilities, analytical dump jobs, or batch ETL scripts:

```go
// argus:ignore ARGUS-A12 analytical batch worker dedicated pool
pool, err := pgxpool.NewWithConfig(ctx, batchCfg)
```

Alternatively, use the canonical identifier alias:

```go
// argus:ignore TIMEOUT_CONFIG offline database dump utility
pool, err := pgxpool.New(ctx, dumpDSN)
```

---

## 8. Configuration Reference (`.argus.yaml`)

Enable or configure timeout defaults in `.argus.yaml`:

```yaml
rules:
  ARGUS-A12:
    enabled: true
```
