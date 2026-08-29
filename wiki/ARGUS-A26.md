# ARGUS-A26: LIKE_WILDCARD_INJECTION

| Meta Field            | Specification                                                                                                                                                                                     |
| :-------------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Rule Code**         | `ARGUS-A26`                                                                                                                                                                                       |
| **Identifier**        | `LIKE_WILDCARD_INJECTION`                                                                                                                                                                         |
| **Severity**          | **HIGH**                                                                                                                                                                                          |
| **Category**          | SQL Injection Prevention, Pattern Hygiene & DoS Prevention                                                                                                                                        |
| **Analysis Layer**    | Layer 4 - Interprocedural Taint & Pattern Analysis                                                                                                                                                |
| **CWE Mapping**       | [CWE-89: SQL Injection (Pattern Language Variant)](https://cwe.mitre.org/data/definitions/89.html), [CWE-400: Uncontrolled Resource Consumption](https://cwe.mitre.org/data/definitions/400.html) |
| **OWASP ASVS**        | OWASP ASVS v4.0.3/v5.0 §V5.3.1 (Input Validation & Output Encoding)                                                                                                                               |
| **PostgreSQL Target** | Pattern Matching Engine §9.7.1, Prefix Index B-tree Invalidation & Sequential Scan DoS Prevention                                                                                                 |
| **Default Status**    | `enabled`                                                                                                                                                                                         |

---

## 1. Executive Summary & Architectural Invariant

User-derived input strings bound to SQL **`LIKE`** or **`ILIKE`** pattern matching clauses **must be explicitly sanitized to escape SQL wildcard characters (`\`, `%`, and `_`)** prior to pattern assembly (e.g. `hecate.FormatLikeContains(userInput)`).

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            ARCHITECTURAL INVARIANT                          │
│                                                                             │
│  Binding unsanitized user inputs to `LIKE`/`ILIKE` parameter placeholders   │
│  is strictly prohibited.                                                    │
│                                                                             │
│  Standard parameterized queries (`$1`) protect against SQL syntax injection │
│  but DO NOT protect against Pattern Language Hijacking (§9.7.1).             │
│                                                                             │
│  Mandatory escaping order: (1) `\` -> `\\`, (2) `%` -> `\%`, (3) `_` -> `\_`│
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Threat Mechanics & Engine Reality (PostgreSQL 18)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    THE LIKE WILDCARD INJECTION EXPLOIT                      │
│                                                                             │
│  Search Endpoint:                                                           │
│  db.Query(ctx, "SELECT id, name FROM citizens WHERE name ILIKE $1", p)      │
│                                                                             │
│  Case A: Raw Unsanitized User Input (VULNERABLE):                           │
│  Attacker submits: "%"                                                      │
│  ├─► Assembled Pattern: "%" + "%" + "%" = "%%%"                             │
│  ├─► Engine matches EVERY RECORD in table (100,000 PII records leaked!)     │
│  ├─► B-tree prefix index INVALIDATED -> Full Table Scan & CPU 100% DoS      │
│  └─► PII EXPOSURE & DENIAL OF SERVICE (CWE-89, CWE-400)                     │
│                                                                             │
│  Case B: Sanitized via hecate.SanitizeLikePattern(input) (COMPLIANT):        │
│  Attacker submits: "%"                                                      │
│  ├─► SanitizeLikePattern transforms "%" into "\%"                           │
│  ├─► Assembled Pattern: "%\%%" ESCAPE '\'                                   │
│  ├─► Engine strictly searches for names literally containing "%"!           │
│  └─► Result: 0 matched rows. Zero CPU spike, zero data leakage!             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.1. Pattern Language Hijacking (§9.7.1)

While parameter binding (`$1`) prevents command injection like `' OR 1=1 --`, PostgreSQL's pattern matching engine evaluates `%` (match any sequence) and `_` (match any character) semantically within the bound string value. A single `%` input defeats intentional query filtering and exposes unauthorized records across the dataset.

### 2.2. Index Invalidation & DoS (CWE-400)

When tables use `text_pattern_ops` indexes for efficient prefix searches, leading wildcards force PostgreSQL to abandon index lookups and execute full sequential scans across millions of records.

---

## 3. Architecture & Execution Flow

```mermaid
flowchart TD
    A["SQL Query Parameterized CallSite"] --> B{"Contains LIKE / ILIKE Clause in SQL AST?"}
    B -- "No" --> C["PASS (Compliant)"]
    B -- "Yes" --> D{"Is Bound Argument Constant or Sanitized?"}
    D -- "Constant Literal or Sanitized via hecate" --> E["PASS (Compliant)"]
    D -- "Raw Variable / Unsanitized Concat" --> F["FAIL: ARGUS-A26 Wildcard Injection Risk (CWE-89)"]
```

---

## 4. Detection Logic & Rule Anatomy

1. **SQL AST Parser (`pg_query_go/v6`):** Identifies `LIKE`, `ILIKE`, `~~`, `~~*` expressions in `SELECT`, `UPDATE`, `DELETE`, and `JOIN` statements and extracts the 1-based parameter placeholder indices ($1, $2, ...).
2. **Taint Tracer:** Verifies if the corresponding argument in the Go call expression originates from:
   - Compile-time constant string literals (`*ast.BasicLit`).
   - Official sanitizers: `hecate.SanitizeLikePattern`, `hecate.FormatLikeContains`, `hecate.FormatLikePrefix`, `hecate.FormatLikeSuffix`, `SanitizeLike`.
   - `strings.ReplaceAll` chains.
3. **Exemptions:** Suppressed via `// argus:ignore ARGUS-A26 <reason>`.

---

## 5. Code Examples Matrix

### Non-Compliant (Unsanitized User Input in LIKE)

```go
// VIOLATION: Raw parameter concatenated directly with wildcards
func SearchUsers(ctx context.Context, db DB, keyword string) ([]User, error) {
    pattern := "%" + keyword + "%"
    const query = "SELECT id, name FROM users WHERE name ILIKE $1"
    return db.Query(ctx, query, pattern)
}
```

```go
// VIOLATION: Raw query parameter bound to SQL concat without sanitization
func SearchAuditLogs(ctx context.Context, db DB, q string) ([]Log, error) {
    const query = "SELECT id FROM logs WHERE message ILIKE '%' || $1 || '%'"
    return db.Query(ctx, query, q)
}
```

---

### Compliant (Sanitized User Input with ESCAPE)

```go
// COMPLIANT: Sanitized using helper function
func SearchUsers(ctx context.Context, db DB, keyword string) ([]User, error) {
    pattern := EscapeLikePattern(keyword)
    const query = "SELECT id, name FROM users WHERE name ILIKE $1 ESCAPE '\\'"
    return db.Query(ctx, query, pattern)
}
```

```go
// COMPLIANT: Explicit sanitization before query binding
func SearchAuditLogs(ctx context.Context, db DB, q string) ([]Log, error) {
    safeQ := EscapeLikePattern(q)
    const query = "SELECT id FROM logs WHERE message ILIKE '%' || $1 || '%' ESCAPE '\\'"
    return db.Query(ctx, query, safeQ)
}
```

---

## 6. Mitigation & Remediation Guide

1. **Use Sanitized Wildcard Escaper:**
   ```go
   // Escape user input before concatenating into LIKE query pattern
   func EscapeLikePattern(s string) string {
       s = strings.ReplaceAll(s, `\`, `\\`)
       s = strings.ReplaceAll(s, `%`, `\%`)
       s = strings.ReplaceAll(s, `_`, `\_`)
       return "%" + s + "%"
   }
   ```
2. **Explicit Escape Clause:**
   Always include `ESCAPE '\\'` in SQL statements when using wildcards to establish unambiguous delimiter semantics across PostgreSQL versions.

---

## 7. Configuration & Suppression Directives

### Configuration in `.argus.yaml`

```yaml
rules:
  ARGUS-A26:
    enabled: true
```

### Inline Ignore Directives

```go
// argus:ignore ARGUS-A26 internal administrative regex-style pattern search
rows, err := db.Query(ctx, adminQuery, rawWildcardPattern)
```
