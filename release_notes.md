# Release Notes — Argus v1.2.0 (2026-09-05)

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

