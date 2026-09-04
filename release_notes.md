# Release Notes — Argus v1.3.0 (2026-09-05)

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

