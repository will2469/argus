# Semantic Versioning 2.0.0 Official Specification Clauses
**Source:** [semver.org/spec/v2.0.0.html](https://semver.org/spec/v2.0.0.html)  
**Author:** Tom Preston-Werner (co-founder of GitHub)  
**Scope:** Core release triplets `MAJOR.MINOR.PATCH`. Pre-release (`-alpha`, `-beta`, `-rc`) and build metadata (`+build`) are excluded per production simplicity doctrine.

---

## 1. Summary & Core Specification Clauses

### Summary
> *"Given a version number MAJOR.MINOR.PATCH, increment the:*  
> *1. **MAJOR** version when you make incompatible API changes*  
> *2. **MINOR** version when you add functionality in a backward compatible manner*  
> *3. **PATCH** version when you make backward compatible bug fixes"*

---

### Clause 1: Format Standard
> *"A normal version number MUST take the form X.Y.Z where X, Y, and Z are non-negative integers, and MUST NOT contain leading zeroes. X is the major version, Y is the minor version, and Z is the patch version. Each element MUST increase numerically. For instance: 1.9.0 -> 1.10.0 -> 1.11.0."*

---

### Clause 2: Immutability of Released Versions
> *"Once a versioned package has been released, the contents of that version MUST NOT be modified. Any modifications MUST be released as a new version."*

- **Invariant:** Never overwrite or re-tag a release in Git. If a release has a critical bug after publishing, tag a new PATCH (`1.0.1`), never force-push tag `1.0.0`.

---

### Clause 3: Initial Development Phase (0.y.z)
> *"Major version zero (0.y.z) is for initial development. Anything MAY change at any time. The public API SHOULD NOT be considered stable."*

---

### Clause 4: Public API Declaration
> *"Version 1.0.0 defines the public API. The way in which the number is incremented after this release is dependent on this public API and how it changes."*

- **Invariant:** SemVer operates strictly on the **Public API Contract**. Private implementation details do NOT dictate breaking changes.

---

### Clause 5: PATCH Version Increments (x.y.Z)
> *"Patch version Z (x.y.Z | x > 0) MUST be incremented if only backward compatible bug fixes are introduced. A bug fix is defined as an internal change that fixes incorrect behavior."*

- **Trigger Conditions:**
  - Fixing unintended runtime behavior (bugs, race conditions, memory leaks, panics).
  - Security vulnerability patches (CVE remediations) preserving signature contracts.
  - Performance optimizations that do not alter the input/output signature.
  - Internal refactoring within non-exported packages or private function bodies.

---

### Clause 6: MINOR Version Increments (x.Y.z)
> *"Minor version Y (x.Y.z | x > 0) MUST be incremented if new, backward compatible functionality is introduced to the public API. It MUST be incremented if any public API functionality is marked as deprecated. It MAY be incremented if substantial new functionality or improvements are introduced within the private code. It MAY include patch level changes. Patch version MUST be reset to 0 when minor version is incremented."*

- **Trigger Conditions:**
  - New public functions, methods, structs, types, or endpoints added.
  - Existing functions extended with backward-compatible options (e.g. variadic options, optional query params).
  - Deprecation annotations added (e.g., `// Deprecated:`, `@deprecated`). Marking an API as deprecated warns users *before* removal and **MUST** trigger a MINOR bump, not a MAJOR bump.
  - Substantial internal subsystem improvements.
- **Reset Invariant:** `PATCH` version MUST reset to `0` (e.g. `1.4.2` -> `1.5.0`).

---

### Clause 7: MAJOR Version Increments (X.y.z)
> *"Major version X (X.y.z | X > 0) MUST be incremented if any backward incompatible changes are introduced to the public API. It MAY also include minor and patch level changes. Patch and minor version MUST be reset to 0 when major version is incremented."*

- **Trigger Conditions:**
  - Any removal or renaming of public functions, methods, fields, endpoints, or flags.
  - Any signature mutation that breaks existing callers (adding non-default parameters, changing types).
  - Any behavioral modification where previously valid caller code now errors or behaves incompatibly.
  - Removal of previously deprecated features.
  - Stricter input validation that rejects previously valid inputs.
- **Reset Invariant:** Both `MINOR` and `PATCH` versions MUST reset to `0` (e.g. `1.4.2` -> `2.0.0`).

---

### Clause 10: Precedence and Ordering
> *"Precedence is determined by comparing each dot separated identifier from left to right: Major, Minor, and Patch are compared numerically.*  
> *Example: 1.0.0 < 2.0.0 < 2.1.0 < 2.1.1."*
