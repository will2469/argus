# ARGUS-A23: TRANSACTION_TIMEOUT_CONFIG

| Meta Field            | Specification                                                                                 |
| :-------------------- | :-------------------------------------------------------------------------------------------- |
| **Rule Code**         | `ARGUS-A23`                                                                                   |
| **Identifier**        | `TRANSACTION_TIMEOUT_CONFIG`                                                                  |
| **Severity**          | **HIGH**                                                                                      |
| **Category**          | Database Availability, Transaction Governance & Engine Modernization                          |
| **Analysis Layer**    | Layer 2 - Go-AST Structural & Configuration Scope                                             |
| **CWE Mapping**       | [CWE-400: Uncontrolled Resource Consumption](https://cwe.mitre.org/data/definitions/400.html) |
| **OWASP ASVS**        | OWASP ASVS v4.0.3/v5.0 §V1.4.3, §V12 (Denial of Service & Connection Pool Governance)         |
| **PostgreSQL Target** | PostgreSQL 17/18+ (Cumulative Wall-Clock `transaction_timeout` GUC Parameter)                 |
| **Default Status**    | `enabled`                                                                                     |

---

## 1. Executive Summary & Architectural Invariant

Database connection pool configurations (**`pgxpool.Config`**) targeting PostgreSQL 17/18+ **must explicitly configure the `transaction_timeout` GUC parameter** in `RuntimeParams` or DSN connection strings (recommended 30,000ms - 60,000ms for OLTP web applications).

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            ARCHITECTURAL INVARIANT                          │
│                                                                             │
│  PostgreSQL 17/18+ connection pool initializations MUST configure a        │
│  positive `transaction_timeout` GUC parameter in `RuntimeParams` or DSN.    │
│                                                                             │
│  Standard Solutions:                                                        │
│  1. `cfg.ConnConfig.RuntimeParams["transaction_timeout"] = "30000"` (30s)   │
│  2. Append `&transaction_timeout=30000` to DSN connection URI               │
│  3. Use central repository pool configuration helper                        │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. PostgreSQL 18 Engine Internals & Threat Mechanics

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    THE 4-TIER TIMEOUT DEFENSE MATRIX                        │
│                                                                             │
│  [ BEGIN Transaction ]                                                      │
│    │                                                                        │
│    ├─► statement_timeout (10s-15s)  : Bounds single query execution          │
│    ├─► lock_timeout (3s-5s)        : Bounds lock acquisition wait           │
│    ├─► idle_in_tx_timeout (10s)    : Bounds idle wait between statements    │
│    │                                                                        │
│    └─► transaction_timeout (30s)   : CAPSTONE CUMULATIVE WALL-CLOCK CAP     │
│                                      (Prevents 10 x 8s chained queries 80s!)│
│    │                                                                        │
│  [ COMMIT Transaction ]                                                     │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.1. The Blind Spot of Classic Timeouts

- **`statement_timeout`:** Only limits individual statement runtime. A transaction issuing 10 sequential 8-second queries easily evades a 10-second `statement_timeout` while hogging database resources for **80 seconds**.
- **`idle_in_transaction_session_timeout`:** Only measures the gap between statement completions and new arrivals. Rapid continuous queries bypass idle timeouts completely.

### 2.2. The Capstone: `transaction_timeout` in PG17/18

Introduced in PostgreSQL 17 and standardized in PostgreSQL 18, `transaction_timeout` measures total cumulative wall-clock time from `BEGIN` to `COMMIT`/`ROLLBACK`. If the threshold is breached, the PostgreSQL backend terminates the transaction with:

```
ERROR: canceling statement due to transaction timeout (SQLSTATE 25P03)
```

### 2.3. XID Horizon Freezing & Anti-Wraparound Crisis (CWE-400)

Zombie transactions left open by orphaned client connections hold back the oldest transaction ID (`relfrozenxid` in `ProcGlobal->xids`). This prevents autovacuum from vacuuming dead tuples, leading to table bloat and eventually triggering an emergency read-only shutdown. Client-level `transaction_timeout` ensures zombie transactions are terminated deterministically.

---

## 3. Architecture & Execution Flow

```mermaid
flowchart TD
    A["pgxpool Configuration Initialization"] --> B{"Is transaction_timeout Configured in RuntimeParams or DSN?"}
    B -- "No" --> C["FAIL: ARGUS-A23 Missing transaction_timeout Configuration"]
    B -- "Yes" --> D{"Is transaction_timeout == 0 (Disabled)?"}
    D -- "Yes" --> E["FAIL: ARGUS-A23 Unbounded transaction_timeout Violation"]
    D -- "No" --> F["PASS (4-Tier Timeout Matrix Complete)"]
```

---

## 4. Detection Logic & Rule Anatomy

Argus AST visitor inspects:

1. **`RuntimeParams` Map Analysis:** Evaluates composite literals and map index assignments for `"transaction_timeout"`.
2. **DSN Parsing:** Checks connection strings passed to `pgxpool.New` for `transaction_timeout=<non-zero>`.
3. **Pool Initialization Checks:** Verifies that `pgxpool.NewWithConfig` invocations have a corresponding `transaction_timeout` setup in `RuntimeParams`.
4. **Exemptions:**
   - Explicit setting of `transaction_timeout` to positive integer duration.
   - Declarations with `// argus:ignore ARGUS-A23 <reason>`.

---

## 5. Code Examples Matrix

### Non-Compliant (Missing or Disabled `transaction_timeout`)

```go
// VIOLATION: Missing transaction_timeout allows long-running cumulative transactions
cfg, _ := pgxpool.ParseConfig(connString)
cfg.ConnConfig.RuntimeParams = map[string]string{
    "statement_timeout":                   "10000",
    "lock_timeout":                        "3000",
    "idle_in_transaction_session_timeout": "10000",
    // Missing: "transaction_timeout"
}
```

```go
// VIOLATION: Disabling transaction_timeout (set to 0)
cfg.ConnConfig.RuntimeParams["transaction_timeout"] = "0"
```

---

### Compliant (Complete 4-Tier Timeout Matrix)

```go
// COMPLIANT: Explicit 4-tier timeout matrix protecting PostgreSQL 18
cfg, err := pgxpool.ParseConfig(connString)
if err != nil {
    return nil, err
}

if cfg.ConnConfig.RuntimeParams == nil {
    cfg.ConnConfig.RuntimeParams = make(map[string]string)
}

cfg.ConnConfig.RuntimeParams["statement_timeout"] = "15000"                    // 15s
cfg.ConnConfig.RuntimeParams["lock_timeout"] = "5000"                            // 5s
cfg.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "10000" // 10s
cfg.ConnConfig.RuntimeParams["transaction_timeout"] = "30000"                 // 30s Capstone

return pgxpool.NewWithConfig(ctx, cfg)
```

```go
// COMPLIANT: Configured via DSN connection string
const dsn = "postgres://user:pass@localhost:5432/app_db?sslmode=disable&transaction_timeout=30000"
pool, err := pgxpool.New(ctx, dsn)
```

---

## 6. Mitigation & Remediation Guide

1. **Configure in `RuntimeParams`:** Set `RuntimeParams["transaction_timeout"] = "30000"` (30 seconds) on pool startup.
2. **Do Not Set in `postgresql.conf`:** Avoid setting `transaction_timeout` globally in cluster-wide configs to prevent aborting backup utilities (`pg_dump`) or data migrations.
3. **Align with HTTP Deadlines:** Ensure `transaction_timeout` is shorter than the ingress gateway / HTTP request timeout (e.g. 30s DB transaction timeout under a 60s HTTP timeout).

---

## 7. Configuration & Suppression Directives

### Configuration in `.argus.yaml`

```yaml
rules:
  ARGUS-A23:
    enabled: true
    recommended_timeout_sec: 30
```

### Inline Ignore Directives

```go
// argus:ignore ARGUS-A23 administrative migration runner requires unbounded transaction
cfg.ConnConfig.RuntimeParams["transaction_timeout"] = "0"

// argus:ignore TRANSACTION_TIMEOUT_CONFIG legacy postgres 16 compatibility mode
pool, err := pgxpool.New(ctx, legacyDSN)
```
