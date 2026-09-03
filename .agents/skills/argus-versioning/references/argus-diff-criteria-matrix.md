# Argus Diff Criteria Matrix: Classifying Latest Tag to Current HEAD
**Purpose:** Exact criteria matrix to evaluate `git diff <latest_tag>...HEAD` specifically for the **Argus Checker** repository (`github.com/will2469/argus`) and its Go / PostgreSQL ecosystem.

---

## 1. Argus Component Diff Classification Matrix

| Component / Subsystem | **MAJOR (Breaking)** `1.x.x` → `2.0.0` | **MINOR (Additive)** `1.4.0` → `1.5.0` | **PATCH (Fix/Hygiene)** `1.4.1` → `1.4.2` |
| :--- | :--- | :--- | :--- |
| **CLI & Commands** (`cmd/argus/`) | • Removed subcommand (`check`, `check-migrations`)<br>• Removed/renamed CLI flag (`--format`, `--config`)<br>• Changed default behavior of a CLI flag<br>• Altered JSON/SARIF stdout machine-readable schema<br>• Changed process exit codes | • Added new subcommand or new optional CLI flag<br>• Added new supported output format (e.g. `--format=github`)<br>• Added new environment variable override | • Fixed CLI flag parsing crash / nil panic<br>• Improved CLI terminal human-readable output styling<br>• Fixed `--help` text typos or bash completions |
| **Analyzer Rules** (`rules/aXX_.../`) | • Removed an existing rule analyzer<br>• Changed rule code ID (e.g. `ARGUS-A01` renamed)<br>• Incompatible diagnostic position reporting break | • Added new rule analyzer (e.g. `ARGUS-A31+`)<br>• Added new companion AST/SQL walker<br>• Added `// Deprecated:` notice to an existing rule | • Fixed false-positive (FP) in rule AST traversal<br>• Fixed false-negative (FN) in edge-case handling<br>• Added missing standard idiom whitelist (e.g. `COUNT(*)`) |
| **Configuration** (`.argus.yaml`, `shared/config/`) | • Removed existing config key or renamed section<br>• Made previously optional YAML key required<br>• Changed default severity of an existing rule<br>• Made validation reject previously valid `.argus.yaml` | • Added new optional configuration key or section<br>• Added support for new directive alias in YAML<br>• Added non-breaking default overrides | • Fixed YAML parsing error message clarity<br>• Fixed unhandled nil pointer in empty config file<br>• Corrected config file search path precedence bug |
| **Shared Libraries** (`shared/callsite/`, `shared/sqlparser/`, etc.) | • Removed exported function, struct, or method<br>• Added required param to exported function<br>• Changed function signature / parameter type<br>• Added method to exported interface without default | • Added new exported helper function or struct<br>• Added optional variadic option to helper<br>• Added `// Deprecated:` doc comment | • Fixed internal regex/AST caching bug<br>• Optimized `pg_query_go` AST memoization<br>• Fixed memory leak in SQL parser wrapper |
| **MCP Server** (Model Context Protocol) | • Removed an MCP tool (`argus_scan`, etc.)<br>• Renamed tool or removed required argument<br>• Changed tool JSON schema incompatibly<br>• Changed MCP protocol wire version incompatibly | • Added new MCP tool or resource<br>• Added optional property to existing tool input schema<br>• Added new diagnostic metadata in response | • Fixed MCP session lifecycle or pipe EOF bug<br>• Fixed `_meta` protocol version fallback<br>• Concurrency race condition fix in tool registry |
| **Testing Harness** (`tests/correctness/`, `tests/migration/`) | • Deleted golden corpus fixture categories | • Added new golden corpus test cases (P1-P5, N1-N5, A1-A7) | • Fixed flaky test or updated `analysistest` assertions |
| **Runtime & Dependencies** (`go.mod`) | • Raised minimum Go version (e.g. Go 1.25 → Go 1.27)<br>• Upgraded `pg_query_go` with breaking C ABI | • Added new development or linting dependency<br>• Bumped minor dependency version | • Bumped patch dependency for security vulnerability (CVE) |

---

## 2. Git Diff Evaluation Pipeline

### Step 1: Detect Base Tag
```bash
LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || git tag -l --sort=-v:refname | head -n 1)
```

### Step 2: Three-Dot Diff Inspection
```bash
# 1. Overview of changed files
git diff --stat "${LATEST_TAG}...HEAD"

# 2. Check for breaking deletions or signature changes
git diff "${LATEST_TAG}...HEAD" -- 'cmd/' 'rules/' 'shared/'
```

### Step 3: Conventional Commit Scan
```bash
# Check for breaking change markers
git log "${LATEST_TAG}..HEAD" --format="%s%n%b"
```
- **MAJOR:** `type!:` (e.g. `feat!:`, `fix!:`) or body containing `BREAKING CHANGE:`.
- **MINOR:** `feat:` (without `!`).
- **PATCH:** `fix:`, `perf:`, `refactor:`, `docs:`, `test:`, `chore:`.

---

## 3. Presedensi & Invariant Reset

$$\mathbf{MAJOR > MINOR > PATCH}$$

1. **MAJOR Triggered:**  
   `1.4.2` → **`2.0.0`** *(Reset Minor & Patch ke 0)*.
2. **MINOR Triggered (No Major):**  
   `1.4.2` → **`1.5.0`** *(Reset Patch ke 0)*.
3. **PATCH Triggered (No Major & No Minor):**  
   `1.4.2` → **`1.4.3`** *(Hanya Patch naik)*.
