# Changelog

All notable changes to this project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).

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
