## Description

Please provide a clear and concise summary of the changes in this pull request and the rationale behind them.

Closes #(issue)

---

## Type of Change

- [ ] `feat`: New analyzer rule or CLI feature
- [ ] `fix`: Bug fix or false-positive reduction in an existing rule
- [ ] `docs`: Documentation or Wiki update
- [ ] `refactor`: Internal refactoring with no behavior changes
- [ ] `test`: Additional test fixtures or test harness improvements
- [ ] `ci`: Build, release, or CI workflow updates

---

## Quality & Architecture Checklist

- [ ] **Anti-Fat Code (~250 lines/file):** Files are cohesively scoped and adhere to the repository size boundaries.
- [ ] **Zero False-Positive Target:** Standard idioms (`COUNT(*)`, `CollectRows`, static constants) have appropriate allowlists.
- [ ] **Directive Support:** Rules properly respect `// argus:ignore` / `-- argus:ignore`.
- [ ] **Hermetic Testing:** Rule test suites use `golang.org/x/tools/go/analysis/analysistest` with compliant, non-compliant, and ignored fixtures.
- [ ] **All Tests Pass:** `go test -race ./...` passes with zero failures.
- [ ] **Code Hygiene Clean:** `make lint` (`go vet` and `gofmt`) passes with zero warnings.
- [ ] **Clean Conventional Commit:** Git commit messages follow the Conventional Commits specification.
