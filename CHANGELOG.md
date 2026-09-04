# Changelog

All notable changes to this project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).

---

## [v1.3.0] - 2026-09-05

Argus v1.3.0 is a **MINOR** release delivering backward-compatible new features, rule extensions, and diagnostic enhancements.

---

### 🚀 New Features & Enhancements
* Support comma-separated rule codes in ignore directives and stacked comment lookback

---

### 🐛 Diagnostic Precision & Bug Fixes
* Resolve compile-time constant identifiers in concat expression inspection
* Extract CREATE TYPE from DO blocks/enum/composite stmts and resolve TypeName in DROP TYPE
* Recognize that DROP TABLE cascades to newly created indexes and columns
* Restrict parameter error tracking to actual error types and support package constants in IsCompileTimeString
* Audit pgx.Batch.Queue and exclude non-SQL methods to eliminate A05 false positives

---

### 🔧 Maintenance & Internal Hygiene
* Eliminate monorepo ceremony leakage, tighten directive lookback, and decompose large analyzers

---

### 📦 Installation & Upgrade

#### In-Place Self-Update (Existing Installations)
```bash
argus update # or: argus --update, argus -u
```

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
go install github.com/will2469/argus/cmd/argus@v1.3.0
```

_Or download pre-compiled binaries directly from [GitHub Releases](https://github.com/will2469/argus/releases/tag/v1.3.0)._
---

## [v1.2.0] - 2026-09-05

Argus v1.2.0 is a **MINOR** release delivering backward-compatible new features, rule extensions, and diagnostic enhancements.

---

### 🚀 New Features & Enhancements
* Implement non-blocking CLI update hint with 24h cache
* Increase DSN flow analysis depth to 64 and enforce strict object matching in call arguments
* Improve variable tracking accuracy by utilizing ast.Object references during flow analysis
* Implement variable shadowing detection and support for switch statement fallthrough in DSN flow analysis
* Improve DSN flow analysis with reaching definition logic for switches and variable overrides
* Support dsn reaching definitions through variable declarations and expression statements
* Upgrade constant evaluation to 64-bit precision and enhance flow-sensitive analysis for pgxpool connection limits
* Modularize pgxpool analysis with improved call identification and DSN resolution flows
* Add provenance verification to reject mock or non-DB implementations of database interfaces
* Implement AST-based type resolution for DDL analysis and update DBExecutor interfaces to use standard database types.
* Implement semantic type checking for advisory lock helpers and update corpus test expectations
* Implement fail-closed provenance tracking for audit immutability by introducing explicit Unknown state for unresolvable expressions
* Enforce proof-level database call identity and eliminate receiver naming heuristics
* Enforce semantic migration reversibility and AST inverse operation proof
* Enhance transaction IO detection by introducing dedicated type resolution, AST helpers, and expanded test coverage
* Prove db receivers, isolate transaction calls, and resolve query identifiers via lexical object identity
* Eliminate regex fallbacks, reject dot delimiter, and resolve identifiers via lexical object identity
* Eliminate naming heuristics and implement flow-sensitive transaction lifetime analysis
* Eliminate naming heuristics and enforce type-proven error provenance
* Implement fail-closed lattice join for branch DDL tracking and verify builder receiver type
* Enforce object identity, package scope resolution, and dynamic query checks
* Enforce bidirectional rollback symmetry and target schema op pairing
* Enforce semantic type identity, object flow resolution, and fail-closed lattice join for timeout configs
* Enforce authoritative multi-factor contract evidence and block-scoped metadata parsing
* Implement object-aware ddl privileges, rolespec resolution, and public pseudo-role separation
* Implement object-identity query provenance and scope-dominant variable resolution
* Implement semantic inverse rollback engine and object symmetry verification
* Implement semantic type identity, idle_in_transaction enforcement, and robust dsn resolution
* Implement 4-stage destructive migration pipeline with validated contract evidence
* Implement table-correlated row locking, schema identity, and safe dml target extraction
* Implement accurate sql argument extraction, 1-key vs 2-key advisory lock semantics, and go helper namespace hygiene
* Implement semantic transaction identity, type-safe external io classification, and path-sensitive overlap
* Implement call classification and database argument extraction to support advanced A06/A07 rule analysis
* Implement rule-specific YAML configuration support and enhance soundness of A01, A03, and A04 analysis rules
* Add rule catalog and enrich markdown report with metadata, documentation links, and standardized suppression instructions
* Add DisplayTag, RuleCode, and suppression instructions to issue reporting
* Introduce RuleAliases and update scanner meta parsing to resolve rule codes dynamically
* Implement rule-based configuration to enable or disable individual scanners and audit rules

---

### 🐛 Diagnostic Precision & Bug Fixes
* Enforce proof-level reversible migration semantics and qualified object identity
* Enforce proof-level db transaction contract and semantic ast package resolution
* Inspect file-level declarations in isIdentShadowed to resolve unused parameter
* Enforce proof-level advisory lock helper receiver identity and signature verification
* Enforce proof-level db pool signature checking and fail-closed transaction identity
* Enforce proof-level database call identity and semantic package resolution
* Expand expression resolution depth limit to prevent premature unknown fallback
* Synchronize Option 1 querier verification and add non-delegating wrapper negative tests
* Eliminate isStructWithDBField and enforce proof-level interface contracts
* Enforce proof-level driver type signatures and eliminate lexical name heuristics
* Preserve schema qualification and verify column type reversibility
* Enforce actual pgxpool.Config object identity and path-sensitive flow lattice
* Enforce semantic db receiver and transaction helper recognition
* Enforce semantic namespace contract and scope-hierarchy identifier resolution
* Implement abstract lattice join and semantic database error provenance
* Implement abstract lattice join and semantic method recognition
* Enforce object identity and path-sensitive query resolution
* Ensure error origin requires database or generic provenance in variable assignment tracking

---

### 🔧 Maintenance & Internal Hygiene
* Update DB interface definitions in tests and refine AST type resolver to enforce presence of both Exec and Query methods
* Centralize database type identification and tighten provenance checks for interface detection
* Introduce shared dbident package to consolidate database type and import identification logic
* Eliminate proprietary test package hacks and generalize config shape resolution
* Update type resolution to use declaration positions and simplify pointer dereferencing loop

---

### 📦 Installation & Upgrade

#### In-Place Self-Update (Existing Installations)
```bash
argus update # or: argus --update, argus -u
```

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
go install github.com/will2469/argus/cmd/argus@v1.2.0
```

_Or download pre-compiled binaries directly from [GitHub Releases](https://github.com/will2469/argus/releases/tag/v1.2.0)._
---

## [v1.1.0] - 2026-09-04

Argus v1.1.0 is a **MINOR** release introducing Model Context Protocol (MCP 2026-07-28) tooling, kernel-enforced filesystem security containment, and comprehensive 1-SSOT golden corpus resilience testing across all rule analyzers.

---

### 🚀 New Features & Enhancements
* Standardize MCP _meta responses, upgrade to 2026-07-28 protocol, and implement permissive _meta key fallback
* Adopt descriptive tool policies, implement MCP 2026-07-28 protocol metadata, and enhance concurrency safety in tool registry
* Implement kernel-enforced filesystem containment with RootCapability for secure directory traversal
* Implement generation-based ABA protection in RequestTracker and modularize MCP component types
* Implement core Model Context Protocol (MCP) framework including transport, security, tools, and telemetry components
* Implement cross-rule isolation harness, enhance A26 sanitizer registry, and add A17 symbol resolution logic
* Add govulncheck to pre-commit and Makefile, update release action to v3.0.2, and improve PATH handling in hooks
* Expand Makefile with test-full, test-coverage, and format targets
* Refactor A08 transaction I/O detector to support external test suites and improve rule analysis logic
* Implement modular A07 database error leak analyzer with expanded test corpus and CLI support
* Refactor ARGUS-A06 analyzer to support modular inspection and migrate tests to standalone correctness suite
* Implement A05 audit table immutability rule to detect and report forbidden SQL mutations
* Refactor A04 ORDER BY analysis into reusable logic and add comprehensive adversarial test suite
* Migrate golden test data to new structure and add query analysis test cases
* Implement robust alias tracing for ARGUS-A03 unbounded context detection and migrate test suite structure
* Implement A02 unclosed rows analyzer and integrate it into the main scan pipeline with comprehensive test corpora
* Restructure A01 SQL injection analyzer and migrate legacy testdata to new correctness test suite
* Implement dynamic tenant table detection via AST analysis and add golden corpus integration tests
* Introduce strict mode and E001 error for unparseable SQL migrations
* Improve SQL injection detection accuracy by validating database receiver types and refining context-aware argument extraction
* Implement transitive call graph propagation and improved heuristics for N+1 query detection
* Enhance tenant leak analysis with table-specific isolation checks, operator validation, and multi-table join support
* Improve tenant leakage detection by refining AST inspection logic and adding support for complex SQL predicates
* Improve ARGUS-A26 detection logic with SQL fragment wrapping, enhanced regex scanning, and refactored AST reporting
* Implement AST-based taint analysis for SQL concatenation rule A01 to support non-type-checked code analysis
* Initialize agent skills for rule scaffolding, dual-path parity checking, and pgquery ast safety analysis
* 1-SSOT Golden Corpus Standard: Adopted standardized 17-pattern adversarial test corpus across 22 rules

---

### 🐛 Diagnostic Precision & Bug Fixes
* Remove strings.ReplaceAll from A26 sanitizer whitelist and add explicit test case for validation

---

### 🔧 Maintenance & Internal Hygiene
* Implement configurable sanitizer registry and add end-to-end verification for ARGUS-A26 wildcard sanitization
* Purge all legacy testdata references in favor of 1-SSOT golden corpus
* Implement control-flow dominance analysis for RLS session setup verification
* Replace heuristic database type matching with semantic type checking in IsPgxOrSQLType
* Implement flow-sensitive taint analysis for variable sanitization tracking

---

### 📦 Installation
```bash
go install github.com/will2469/argus/cmd/argus@v1.1.0
```
---

## [v1.0.0] - 2026-08-30

### Initial Public Release
* Production-grade compile-time static analyzer for Go & PostgreSQL 18.x.
* 30 Built-in database safety and hygiene rules covering Security, Connection Lifecycle, Performance, and Zero-Downtime Migrations.
* Dual execution modes: Official `go/analysis` multichecker driver and standalone CLI runner (`argus check`, `argus check-migrations`).
* Comprehensive 8-Pillars wiki documentation for all 30 rules.

---
