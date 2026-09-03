# Shared Engine Design Pattern for Dual-Path Parity

Pola perancangan kode Go standar untuk memastikan logika inspeksi static analyzer dapat dieksekusi secara identik oleh Go Analysis Driver (`pass *analysis.Pass`) dan Standalone CLI Runner (`runner`).

---

## 1. Pola Engine Bersama untuk Rule Kode Go

Struktur file:
```text
rules/aXX_<name>/
├── analyzer.go       # Coordinator untuk go vet / multichecker
├── ast_visitor.go    # Single Source of Truth (SSOT) inspection engine
└── analyzer_test.go  # Unit test analysistest & parity test
```

### Implementasi di `ast_visitor.go`:
```go
package aXX_<name>

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/directives"
)

type Issue struct {
	Pos     token.Pos
	Message string
}

// InspectFile adalah SSOT: dipanggil oleh analyzer.go DAN runner/scan_go.go.
// Jika dipanggil dari runner, parameter pass bernilai nil.
func InspectFile(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap) []Issue {
	var issues []Issue

	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		if isViolation(n) {
			if dm != nil && fset != nil && dm.IsIgnored(fset, n.Pos(), RuleCode) {
				return true
			}
			issues = append(issues, Issue{
				Pos:     n.Pos(),
				Message: "Penjelasan pelanggaran (CWE-XXX)",
			})
		}
		return true
	})

	return issues
}
```

### Pemanggilan di `analyzer.go` (Mode Vettool):
```go
func run(pass *analysis.Pass) (interface{}, error) {
    // ...
    for _, file := range pass.Files {
        issues := InspectFile(pass, pass.Fset, file, dm)
        for _, issue := range issues {
            pass.Reportf(issue.Pos, "[%s] %s", RuleCode, issue.Message)
        }
    }
    return nil, nil
}
```

### Pemanggilan di `runner/scan_go.go` (Mode Standalone CLI):
```go
issues := aXX_<name>.InspectFile(nil, fset, node, dm)
for _, issue := range issues {
    pos := fset.Position(issue.Pos)
    tracker.AddIssue(Issue{
        File:     relPath,
        Line:     pos.Line,
        Rule:     "CANONICAL_NAME",
        Message:  issue.Message,
        Category: "security",
    })
}
```

---

## 2. Pola Engine Bersama untuk Rule SQL Migrasi

### Implementasi di `rules/aXX_<name>/analyzer.go` atau `standalone_runner.go`:
```go
// CheckMigration adalah SSOT: dipanggil oleh runner/scan_migrations.go DAN analyzer.go run().
func CheckMigration(filename, content string, dm *directives.DirectiveMap) []migration.Issue {
    tree, err := sqlparser.Parse(content)
    if err != nil {
        return nil
    }
    return InspectStatements(filename, content, tree, dm)
}
```

### Pemanggilan di `runner/scan_migrations.go`:
```go
for _, issue := range aXX_<name>.CheckMigration(file, content, fileDm) {
    addMigrationIssue(issue, "ARGUS-AXX", rootDir, tracker)
}
```
Hasil: Tidak ada kode regex ad-hoc di runner; 100% konsisten.
