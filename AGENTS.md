# Argus AI Agent Guidelines

Core engineering rules and quality guardrails for AI coding agents operating on the **Argus** codebase.
Priorities: **safe · sound · unbloated · zero-hallucination · test-backed**.

---

## 1. Project Architecture & Environment

**Domain:** Compile-time static analysis and database hygiene checker for Go & PostgreSQL 18.x.

**Core Technology Stack & Module Path:**

- **Module Path:** `github.com/will2469/argus` (Target Public Repo: `https://github.com/will2469/argus`).
- **Language:** Go 1.25+ (Standard Go toolchain).
- **Static Analysis Framework:** `golang.org/x/tools/go/analysis` (Official Go driver).
- **SQL Parser Engine:** `github.com/pganalyze/pg_query_go/v6` (Native C `libpg_query` AST parser).
- **Configuration Parser:** `gopkg.in/yaml.v3` (for `.argus.yaml`).
- **Target Driver Idioms:** `github.com/jackc/pgx/v5` and `database/sql`.

**Structural Mapping:**

| Path                   | Purpose                                                              |
| :--------------------- | :------------------------------------------------------------------- |
| `cmd/argus/`           | Standalone CLI entrypoint (`argus check`, `argus check-migrations`)  |
| `rules/`               | Individual modular analyzers (`a01_.../`, `a02_.../` s/d `a30_.../`) |
| `testdata/src/<rule>/` | Central test fixtures evaluated via `analysistest.Run`               |
| `shared/callsite/`     | Common AST call-expression and interface resolution utilities        |
| `shared/directives/`   | Parsing and enforcement of `// argus:ignore` comments                |
| `shared/config/`       | Config loader and validation for `.argus.yaml`                       |
| `shared/migration/`    | Standard issue structure and line calculation utilities              |
| `shared/sqlparser/`    | Wrapper and AST helpers around `pg_query_go`                         |

---

## 2. Universal Engineering & Code Constraints

1. **Anti-Fat Code (~250 Lines/File):**
   - Strictly limit Go source files to **~250 lines**.
   - If an analyzer grows beyond this, decompose cohesively:
     - `analyzer.go`: analyzer declaration and `Run` coordinator.
     - `ast_visitor.go`: AST node inspection logic.
     - `call_matcher.go`: library/function symbol matching.
2. **Strict Public Isolation (No Internal Doc-Linking):**
   - Prohibit internal monorepo doc-linking or proprietary project terminology.
   - Use self-contained, descriptive Go docstrings suitable for open-source consumers.
3. **Never Break AST Determinism:**
   - Prohibit fragile string regexes where an AST node (`pg_query.Node` or `ast.Node`) can be evaluated deterministically.
   - SQL queries must be parsed using `pg_query_go.Parse()` to inspect the real query tree (`SelectStmt`, `FromClause`, `WhereClause`, `IndexStmt`, etc.).
4. **Zero False-Positive Target:**
   - A static analyzer that emits false positives damages developer trust.
   - Always provide explicit whitelists for valid standard idioms:
     - `COUNT(*)` in select-star checks.
     - `pgx.CollectRows` in missing-error checks.
     - Static constants in dynamic order-by checks.
5. **Directives Support (`argus:ignore`):**
   - Every analyzer MUST check for suppression comments before emitting diagnostics:
     - `// argus:ignore <RULE_CODE> <reason>` (Go source)
     - `-- argus:ignore <RULE_CODE> <reason>` (SQL migrations)
   - Reasons must contain at least 2 words to ensure accountability.
6. **No Blind Agreement / Anti-Sycophancy:**
   - Challenge insecure shortcuts, incomplete error handling, or skipped test cases.
   - Never bypass validation rules or omit edge-case assertions.

---

## 3. Testing Guardrails (`analysistest`)

- Every rule under `rules/` MUST have a complete test suite using `golang.org/x/tools/go/analysis/analysistest`.
- Test fixture requirements:
  - Positive tests (compliant code producing zero diagnostics).
  - Negative tests (non-compliant code annotated with `// want "pattern"`).
  - Ignored tests (annotated with `// argus:ignore` producing zero diagnostics).
  - Edge cases (subqueries, CTEs, aliased packages, struct method wrappers).
- Run test suites before any commit:
  ```bash
  go test ./...
  go vet ./...
  ```

---

## 4. Git & Commit Guidelines

- Strictly adhere to **Conventional Commits**:
  - `feat(rule): implement ARGUS-A24 tenant isolation leak analyzer`
  - `fix(a14): support EXISTS subqueries in targetList traversal`
  - `docs: update rules matrix in README.md`
  - `refactor(shared): optimize AST caching in sqlparser`
