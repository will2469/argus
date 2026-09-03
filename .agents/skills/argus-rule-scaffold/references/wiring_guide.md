# Argus Rule Wiring & Registration Guide

Panduan teknis mendalam untuk menghubungkan aturan baru (`ARGUS-AXX`) ke seluruh subsistem registri Argus.

---

## 1. MultiChecker Registration (`rules/rules.go`)

Daftarkan analyzer agar dieksekusi saat Argus berjalan sebagai vettool (`argus -flags` atau `go vet -vettool=argus`).

```go
import (
    "github.com/will2469/argus/rules/aXX_<name>"
)

var AllAnalyzers = []*analysis.Analyzer{
    // ...
    aXX_<name>.Analyzer,
}
```

---

## 2. Directives Alias Registration (`shared/directives/alias.go`)

Daftarkan alias identifier ke dalam map `ruleAliases` agar parser supresi `// argus:ignore-<alias>` mengenali shortcode:

```go
var ruleAliases = map[string]string{
    // ...
    "<CANONICAL-IDENTIFIER-KEBAB>": "ARGUS-AXX",
    "<SHORT-ALIAS-KEBAB>":          "ARGUS-AXX",
}
```
*Catatan:* Huruf besar dengan tanda hubung (`-`), misalnya `"MISSING-PARTITION-KEY": "ARGUS-A31"`.

---

## 3. Runner Metadata Registration (`runner/rules_meta.go`)

### A. Entri Deskripsi Kanonikal
Tambahkan deskripsi singkat ke map `CanonicalDescriptions`:

```go
var CanonicalDescriptions = map[string]string{
    // ...
    "AXX": "CANONICAL_IDENTIFIER_NAME",
}
```

### B. Kalkulasi Komponen Terperiksa
Jika aturan memeriksa berkas migrasi SQL (`.sql`), tambahkan `"AXX"` ke daftar ID migrasi di `CalculateCheckedComponents`:

```go
func CalculateCheckedComponents(id string, querySites, migrationFiles, totalFiles int) int {
	switch id {
	case "A11", "A13", "A15", "A27", "A28", "A29", "A30", "AXX":
		if migrationFiles > 0 {
			return migrationFiles
		}
		return 156
    // ...
```

---

## 4. Standalone Runner Wiring (Dual-Mode Execution)

### Kasus A: Rule Memeriksa Kode Go (`runner/scan_go.go`)

Panggil fungsi engine bersama di dalam fungsi `scanGoSourceFile`:

```go
import "github.com/will2469/argus/rules/aXX_<name>"

// Di dalam fungsi scanGoSourceFile:
issues := aXX_<name>.InspectFile(nil, fset, node, dm)
for _, issue := range issues {
    pos := fset.Position(issue.Pos)
    tracker.AddIssue(Issue{
        File:     relPath,
        Line:     pos.Line,
        Rule:     "CANONICAL_IDENTIFIER_NAME",
        Message:  issue.Message,
        Category: "security", // atau "performance" / "hygiene"
    })
}
```

### Kasus B: Rule Memeriksa Migrasi SQL (`runner/scan_migrations.go`)

Panggil `CheckMigration` di dalam loop evaluasi berkas `.up.sql`:

```go
import "github.com/will2469/argus/rules/aXX_<name>"

// Di dalam loop sqlFiles pada scanMigrationDirectories:
for _, issue := range aXX_<name>.CheckMigration(file, content, fileDm) {
    addMigrationIssue(issue, "ARGUS-AXX", rootDir, tracker)
}
```

---

## 5. Configuration Defaults (`shared/config/config.go`)

Jika nomor aturan melampaui 30 (misal A31–A35), perbarui batas loop inisialisasi default pada fungsi `DefaultConfig`:

```go
// Enable all standard rules by default
for i := 1; i <= 35; i++ {
    code := formatRuleCode(i)
    cfg.Rules[code] = RuleConfig{
        Enabled: true,
        Options: make(map[string]interface{}),
    }
}
```

---

## 6. Catalog Synchronization (`wiki/Home.md` & `README.md`)

Tambahkan baris dokumentasi ke tabel kategori yang sesuai:

```markdown
| [`ARGUS-AXX`](ARGUS-AXX.md) | `CANONICAL_IDENTIFIER` | **HIGH** | Penjelasan singkat invariant aturan | `enabled` |
```
