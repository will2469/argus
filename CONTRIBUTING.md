# Contributing to Argus

Thank you for your interest in improving Argus! We welcome contributions from developers passionate about database reliability, static analysis, and PostgreSQL engine internals.

---

## Code of Conduct

We are committed to providing a welcoming, inclusive, and harassment-free environment for everyone. Please treat fellow contributors with respect and constructive feedback.

---

## Development Setup

### Prerequisites

- Go 1.24 or higher
- Git

### Getting Started

```bash
# Clone the repository
git clone https://github.com/will2469/argus.git
cd argus

# Download dependencies
go mod download

# Run all test suites
go test ./...

# Run linters
go vet ./...
```

---

## How to Add a New Analyzer Rule

1. **Package Placement:** Create a new directory under `rules/aXX_<name>/` (e.g., `rules/a31_unpinned_search_path/`).
2. **Analyzer Definition:** Implement an exported `*analysis.Analyzer` in `analyzer.go`:
   ```go
   package a31_unpinned_search_path

   import (
       "golang.org/x/tools/go/analysis"
       "github.com/will2469/argus/shared/directives"
   )

   var Analyzer = &analysis.Analyzer{
       Name:     "argus_a31_unpinned_search_path",
       Doc:      "Enforce explicit search_path pinning on SECURITY DEFINER functions and database sessions",
       Run:      run,
       Requires: []*analysis.Analyzer{directives.Analyzer},
   }
   ```
3. **Write Fixture Tests:** Add test cases in `rules/aXX_<name>/testdata/src/a/`:
   - Compliant code without diagnostics.
   - Non-compliant code with expected diagnostics using `// want "pattern"`.
   - Ignored violations with `// argus:ignore`.
4. **Register the Rule:** Register your new analyzer in `rules/rules.go`.
5. **Run Verification:**
   ```bash
   go test -v ./rules/aXX_<name>/...
   ```

---

## Code Style & Best Practices

- **Anti-Fat Code:** Limit file length to **~250 lines**. Keep analyzers modular and cohesive.
- **Zero In-Memory ORMs:** Argus analyzers strictly target native Go code, standard libraries, and `pgx/v5`.
- **Zero False-Positive Target:** Always evaluate real-world idioms to avoid noisy false alerts.
- **Conventional Commits:** All commit messages must follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:
  - `feat(...)`: A new analyzer or feature
  - `fix(...)`: A bug fix or false-positive reduction
  - `docs(...)`: Documentation changes
  - `test(...)`: Adding or refactoring test fixtures
  - `refactor(...)`: Code changes that neither fix bugs nor add features

---

## Submitting Pull Requests

1. Fork the repository and create your branch from `main`:
   ```bash
   git checkout -b feat/my-new-rule
   ```
2. Ensure all tests and linters pass:
   ```bash
   go test ./...
   go vet ./...
   ```
3. Push your branch and open a Pull Request with a clear description of the problem and your solution.
