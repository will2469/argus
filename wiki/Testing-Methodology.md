# Argus 1-SSOT Golden Corpus & Adversarial Testing Methodology

> _"A static analyzer that only tests whether obvious violations trigger suffers from a dangerous illusion of security. Real-world integrity requires proving that safe idioms survive without noise, and subtle obfuscations cannot evade."_

The **Argus 1-SSOT Golden Corpus** is the testing methodology and architectural harness powering the reliability of the Argus static analyzer. It enforces compile-time static analysis guarantees across Go code and PostgreSQL 18.x migrations with a **Zero False-Positive Target** and **Zero-Divergence Parity**.

---

## 1. The Core Philosophy & The 4-Layer Quality Pyramid

Testing static analysis tools is fundamentally harder than testing standard application logic. A typical unit test asserts `f(x) == y`. However, static analyzers operate on abstract syntax trees (AST) across arbitrary developer expressions, dialects, and architectural wrappers. 

> [!IMPORTANT]
> **"100% Adoption" ≠ "100% Correctness"**
>
> An adoption test (such as checking file existence) only verifies **Structural Presence Gate**—that fixture files exist on disk. It does NOT prove semantic soundness, resilience against logic inversion, or resistance to subtle evasion techniques.
>
> To deliver true compiler-grade guarantees with zero false alarms and zero silent escapes, Argus organizes validation into the **4-Layer Continuous Quality Pyramid**:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    THE 4-LAYER COMPILER-GRADE QUALITY GATES                 │
├─────────────────────────────────────────────────────────────────────────────┤
│  Layer 1: Golden Corpus Adoption (Structural Presence Gate)                 │
│  ├─► Verifies 1-SSOT directory structure, fixtures, and runner tests exist  │
│  └─► Gate: TestGoldenCorpus_AdoptionMatrix (Status: 100% File Presence)     │
│                                                                             │
│  Layer 2: Golden Corpus Correctness (Semantic Execution Gate)               │
│  ├─► Positive Gate: P1–P5+ trigger exact diagnostics on expected lines      │
│  ├─► Negative Gate: N1–N5+ produce zero false positives on idioms/consts    │
│  ├─► Adversarial Gate: A1–A7 / M1–M7 anti-obfuscation vectors caught        │
│  └─► Dual-Path Parity Gate: Analysis Driver (go vet) == Standalone CLI      │
│                                                                             │
│  Layer 3: Mutation & Evasion Testing (Resilience Gate)                      │
│  ├─► Operator Inversion: AND ↔ OR, = ↔ !=, NOT, IS NOT NULL                 │
│  ├─► Identity & Scope Evasion: Method collision, unverified receivers       │
│  ├─► Lexical Spoofing: Fake SQL comments (-- tenant_id = 1), string quotes │
│  ├─► Fail-Closed AST Invariant: Unparseable queries rejected, not bypassed  │
│  └─► Target: Mutation Kill Rate = 100% (Zero Surviving Mutants)             │
│                                                                             │
│  Layer 4: Cross-Rule Regression & Interaction Matrix (Isolation Gate)       │
│  ├─► Multi-checker concurrent execution (all 30 rules active concurrently)  │
│  ├─► Shared infrastructure isolation (Zero cache poisoning in sqlparser)    │
│  ├─► Directive scoping isolation (// argus:ignore-a24 does not mute a26)    │
│  └─► Whole-Program Golden Corpus (tests/golden/golden.go)                   │
└─────────────────────────────────────────────────────────────────────────────┘
```

1. **Zero False-Positive Target (Layer 2):** If a linter emits warnings on valid, standard idioms (such as `COUNT(*)`, `pgx.CollectRows`, or verified prefix constants), developers lose trust and disable the tool with `--no-verify`. Compliant code **must survive with zero diagnostics**.
2. **Zero False-Negative Target (Layer 2 & 3):** Unsafe queries rarely appear as naive string concatenations in enterprise codebases. They hide inside helper functions, closures, conditional branches, repository wrappers, or aliased types. Analyzers must be resilient against structural obfuscation and mutation evasion.

---

## 2. 1-SSOT Architecture (Single Source of Truth)

In traditional static analysis setups, developers often maintain disparate test directories for the analysis driver (`analysistest`) and integration tests for the CLI runner. This duplication frequently leads to **silent divergence**—where an analyzer passes unit tests but fails when executed via the CLI binary.

Argus permanently resolves this through the **1-SSOT (Single Source of Truth) architecture**:

```text
tests/
├── golden_corpus_status_test.go     # Automated 30-Rule Adoption Matrix Enforcement
├── correctness/                     # 24 Go AST Rules (A01–A10, A12, A14, A16–A26)
│   ├── a01/                         # Rule ARGUS-A01
│   │   ├── positive/                # P1–P5 Violations (annotated with // want)
│   │   │   └── positive.go
│   │   ├── negative/                # N1–N5 Compliant & Suppressed Code (0 diagnostics)
│   │   │   └── negative.go
│   │   ├── adversarial/             # A1–A7 Structural Obfuscation & Evasion Matrix
│   │   │   └── adversarial.go
│   │   └── a01_corpus_test.go       # Dual-Path Parity Harness & Direct Engine Tests
│   └── a02/ ... a26/
│
└── migration/                       # 6 SQL Migration Rules (A11, A13, A15, A27–A30)
    ├── a11/                         # Rule ARGUS-A11
    │   ├── positive/                # Violating migration DDLs (DROP TABLE, DROP COLUMN)
    │   │   ├── positive.go          # Go test runner harness
    │   │   └── migrations/          # Raw SQL migration fixtures (*.up.sql)
    │   ├── negative/                # Safe migration DDLs (expand-contract, -- argus:ignore)
    │   │   ├── negative.go
    │   │   └── migrations/
    │   ├── adversarial/             # M1–M7 SQL AST Evasion Vectors (casing, quotes, chains)
    │   │   ├── adversarial.go
    │   │   └── migrations/
    │   └── a11_corpus_test.go       # Migration Engine & Standalone Parity Harness
    └── a13, a15, a27, a28, a29, a30
```

### Key Architectural Invariants:
- **100% Structural Symmetry:** Both Go AST rules and SQL migration rules share the exact same tripartite hierarchy (`positive/`, `negative/`, `adversarial/`).
- **Module Root Driver Wiring:** `rules/aXX/analyzer_test.go` leverages Go's module root resolution to execute `analysistest.Run` directly against `./tests/correctness/aXX/positive` and `./tests/correctness/aXX/negative`.
- **Zero Fixture Duplication:** The standalone CLI runner (`runner.RunAuditWithConfig`) targets the exact same files evaluated by `analysistest.Run`.

---

## 3. The 17-Pattern Corpus Taxonomy (Go AST Rules)

Every Go source code analyzer is evaluated against a canonical **17-Pattern Matrix**:

| Category | Code | Pattern Name | Test Objective & Description |
| :--- | :--- | :--- | :--- |
| **Positive** | `P1` | **Obvious Violation** | Direct raw function call or straightforward illegal AST expression. |
| | `P2` | **Indirect Violation** | Multi-step assignment, intermediate variables, or reassigned values. |
| | `P3` | **Helper Violation** | Violation embedded inside private helper functions or secondary methods. |
| | `P4` | **Nested Violation** | Violation nested deeply inside conditional blocks, loops, or select statements. |
| | `P5` | **Alias Violation** | Package imported with an alias or receiver referenced through a type alias. |
| **Negative** | `N1` | **Obvious Safe** | Idiomatic, compliant implementation using standard parameter bindings. |
| | `N2` | **Legitimate Idiom** | Compiler-recognized safe idioms (e.g. `COUNT(*)`, `EXISTS (SELECT 1)`). |
| | `N3` | **Unrelated API** | Non-database structs sharing identical method names (e.g. custom `Logger.Query`). |
| | `N4` | **Sanitized Input** | Input verified through static allowlists or approved sanitization routines. |
| | `N5` | **Static / Constant** | Untainted compile-time string constants, typed numeric constants, or math literals. |
| **Adversarial** | `A1` | **Branching** | Conditional branches (`if/else`, `switch/case`) where one path is tainted. |
| | `A2` | **Reassignment** | Clean variables reassigned with dirty values, or dirty variables overwritten cleanly. |
| | `A3` | **Aliasing** | Struct embedding, pointer indirection, and local interface references. |
| | `A4` | **Wrapper** | Custom database repositories, transaction managers, or logging decorators. |
| | `A5` | **Nested Closure** | Violations dispatched inside anonymous closures, `defer` statements, or goroutines. |
| | `A6` | **Generics** | Generic repository abstractions (`[T any]`) parameterized with database models. |
| | `A7` | **Interface Dispatch** | Dynamic method dispatch through `any.(Querier)` or untyped interface parameters. |

---

## 4. The M1–M7 Adversarial Matrix (SQL Migration Rules)

PostgreSQL DDL migrations are analyzed using native `libpg_query` C-AST parser trees. To ensure SQL migrations cannot bypass checks through lexical or grammatical variations, migration rules are stress-tested against the **M1–M7 Adversarial Matrix**:

```mermaid
mindmap
  root((M1–M7 Adversarial Matrix))
    M1["M1: Multi-Statement Chaining<br/>Semi-colon separated batches"]
    M2["M2: Case-Insensitivity<br/>Mixed casing (CrEaTe InDeX)"]
    M3["M3: Quoted Identifiers<br/>Escaped table/col names (&quot;users&quot;)"]
    M4["M4: Schema Qualification<br/>Namespace prefix (public.users)"]
    M5["M5: Structural Variation<br/>Inline vs out-of-line DDL constraints"]
    M6["M6: Non-Leading Indexes<br/>Composite index column ordering"]
    M7["M7: Multi-Target Batching<br/>Comma-delimited statement targets"]
```

1. **M1: Multi-Statement Chaining:** Semicolon-delimited queries within a single `.up.sql` file to ensure the parser traverses all statement nodes in the parse tree without truncating on the first node.
2. **M2: Case-Insensitivity:** Arbitrary casing (`cReAtE iNdEx`, `tImEsTaMp`, `dRoP tAbLe`) to verify AST node inspection rather than fragile string-matching.
3. **M3: Quoted Identifiers:** SQL identifiers wrapped in double quotes (`"orders"`, `"created_at"`), preserving exact PostgreSQL identifier semantics.
4. **M4: Schema Qualification:** Explicit namespace prefixes (`public.accounts`, `billing.invoices`) ensuring table names are matched regardless of schema scoping.
5. **M5: Structural Variation:** Table constraints declared inline during `CREATE TABLE` versus added later via `ALTER TABLE ADD CONSTRAINT`.
6. **M6: Precision & Non-Leading Index Variants:** Testing subtle SQL variations, such as `TIMESTAMP(6)` precision specifiers or composite indexes where the inspected column is not the leading key.
7. **M7: Multi-Target Batching:** Statements operating on multiple targets in a single clause (e.g. `DROP TABLE users, orders CASCADE`).

---

## 5. Dual-Path Parity Invariant

Argus can be run in two distinct environments:
- **Path 1 (Driver Mode):** Run via the official Go analysis driver (`go vet -vettool=$(which argus) ./...`).
- **Path 2 (Standalone CLI Mode):** Run directly in terminal or CI pipelines (`argus --dirs=. --migrations=migrations`).

A known pitfall in static analysis tooling is **dual-path divergence** (where the standalone runner reimplements inspection logic with regex or disparate walkers, creating dead code and missed detections).

Argus enforces the **Dual-Path Parity Invariant**:

> **The Parity Rule:** Standalone runners (`runner/scan_go.go` and `runner/scan_migrations.go`) MUST invoke the exact same exported AST inspection engine as the analysis driver package (`rules/aXX/`). Reimplementing rule heuristics using regular expressions is strictly prohibited.

Every rule package includes an automated dual-path parity harness:

```go
func TestDualPathParity(t *testing.T) {
    // 1. Verify Path 1: Official Analysis Driver (analysistest)
    analysistest.Run(t, rootDir, Analyzer, "./tests/correctness/aXX/positive")

    // 2. Verify Path 2: Standalone CLI Runner (runner.RunAuditWithConfig)
    result, err := runner.RunAuditWithConfig(runner.AuditConfig{
        RootDir:  fixtureDir,
        ScanDirs: []string{fixtureDir},
    })
    if len(result.Issues) == 0 {
        t.Fatalf("PARITY FAILURE: Standalone runner reported 0 violations for identical fixture!")
    }
}
```

---

## 6. The 4 Continuous Quality Gates

Argus enforces continuous reliability through **four progressive quality gates**:

### Gate 1: Structural Adoption Matrix (Presence Gate)
Verifies that all rules maintain canonical 1-SSOT directory structures and fixtures:
```bash
go test -v -run TestGoldenCorpus_AdoptionMatrix ./tests
```
- **Status:** 30 / 30 rules adopted (100.0%).
- **Verification Scope:** File existence on disk (`positive.go`, `negative.go`, `adversarial.go`, `<rule>_corpus_test.go`).

### Gate 2: Semantic Correctness & Dual-Path Parity (Execution Gate)
Executes all rule test suites against the Go analysis driver (`analysistest`) and standalone runner (`runner.RunAuditWithConfig`):
```bash
go test -v ./tests/correctness/... ./tests/migration/...
```
- **Positive Verification:** P1–P5+ violations trigger exact expected diagnostics at expected line positions.
- **Negative Verification:** N1–N5+ safe idioms and compiler constants produce **0 false positives**.
- **Adversarial Verification:** A1–A7 and M1–M7 obfuscated code patterns are caught.
- **Dual-Path Parity:** Standalone CLI scanner produces 100% identical diagnostics as `analysistest`.

### Gate 3: Mutation & Evasion Resilience (Resilience Gate)
Stress-tests rules against intentional mutation and syntax evasion:
```bash
go test -v -run "Mutation|Adversarial" ./...
```
- **Logic Inversion:** Conjunction tampering (`AND` ↔ `OR`, `=` ↔ `!=`, `NOT`, `IS NOT NULL`).
- **Identity Integrity:** Receiver collisions (e.g. `(MemoryCache).Get` vs `(DBRepo).Get`) and fake sanitizer stubs (e.g. `evil.SanitizeLikePattern`).
- **Lexical Spoofing:** Verifying that SQL comments (e.g. `-- tenant_id = 1`) or string literals cannot bypass AST rules.
- **Fail-Closed Parser Invariant:** Unparseable queries on domain tables fail with explicit errors rather than falling back to weak heuristics.
- **Target:** **100% Mutation Kill Rate** (zero surviving mutants).

### Gate 4: Cross-Rule Regression & Interaction Matrix (Isolation Gate)
Executes concurrent multi-checker audits across multi-rule files (`tests/golden/golden.go`):
```bash
go test ./... && go vet ./...
```
- **Concurrency & Cache Isolation:** Multi-threaded analysis with shared AST caching (`sqlparser.astCache`) without cache poisoning or race conditions.
- **Zero Shadowing:** Multiple active rules on the same file evaluate independently without masking or suppression.
- **Directive Scoping:** Inline suppression (e.g. `// argus:ignore-a24`) strictly isolates its effect without muting unrelated rules.

Whenever new rules are introduced (**ARGUS-A31+**), they are required to pass all four quality gates before merging.
