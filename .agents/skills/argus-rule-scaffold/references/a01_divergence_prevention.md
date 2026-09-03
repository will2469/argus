# Post-Mortem & Prevention: The A01 Dual-Implementation Trap

Dokumentasi arsitektural mengenai penyebab dan pencegahan bug divergensi logika (Dual-Implementation Bug) pada Argus Checker.

---

## 1. Anatomi Bug Divergensi A01

Argus dirancang mendukung dua mode eksekusi:
1. **Mode Vettool (`golang.org/x/tools/go/analysis`)**: Digunakan saat integrasi `go vet` atau multichecker.
2. **Mode Standalone CLI Runner (`runner/`)**: Digunakan saat eksekusi CLI langsung (`argus check`, `argus audit`).

### Apa yang Terjadi pada ARGUS-A01?
- Di `rules/a01_sql_concat/analyzer.go`, implementasi berjalan presisi: memeriksa `ast.CallExpr`, mengecek apakah receiver adalah method database (`callsite.IsDBQueryMethod`), lalu menjalankan **TaintTracker** mendalam pada `*ast.BinaryExpr` (operator `+`) dan builder check.
- Namun di `runner/scan_go.go`, dibuat pengecekan tiruan terpisah:
  ```go
  var sqlUnsafeConcatRegex = regexp.MustCompile(`(?i)(?:["` + "`" + `]\s*(?:SELECT|INSERT...)[^"` + "`" + `]*["` + "`" + `]\s*\+\s*(?:req\.|input\...))`)
  // ...
  case *ast.BasicLit:
      if sqlUnsafeConcatRegex.MatchString(val) { ... }
  ```
- **Kegagalan Total:** Sebuah `*ast.BasicLit` adalah string literal murni (`"SELECT * FROM users"`). Ekspresi penggabungan `+ req.ID` berada di tingkat `*ast.BinaryExpr`, bukan di dalam string literal `BasicLit`! Akibatnya, regex di `scan_go.go` menjadi **kode mati (dead code)**, sedangkan di `analyzer.go` logika berjalan normal.

---

## 2. Invarian SSOT (Single Source of Truth)

Untuk mencegah divergensi implementasi pada aturan A31+, Argus menetapkan aturan baku:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                       THE UNIFIED SSOT ENGINE MODEL                         │
│                                                                             │
│                        [ rules/aXX_<name>/engine.go ]                       │
│                                      │                                      │
│                  ┌───────────────────┴───────────────────┐                  │
│                  ▼                                       ▼                  │
│        [ Mode 1: Analyzer ]                    [ Mode 2: Standalone ]       │
│     rules/aXX_<name>/analyzer.go                 runner/scan_go.go          │
│       pass.Reportf(issue.Pos)               tracker.AddIssue(issue)         │
└─────────────────────────────────────────────────────────────────────────────┘
```

1. **Dilarang Menulis Logika Deteksi di `runner/`:** Berkas di `runner/` hanya bertindak sebagai adapter I/O dan pelapor metrik.
2. **Paket Rule Wajib Mengekspor Fungsi Bersama:**
   - **Rule Go:** Mengekspor `InspectFile(pass *analysis.Pass, file *ast.File, dm *directives.DirectiveMap) []Issue`. Parameter `pass` boleh bernilai `nil` saat dipanggil dari standalone runner.
   - **Rule SQL Migrasi:** Mengekspor `CheckMigration(filename, content string, dm *directives.DirectiveMap) []migration.Issue`.
3. **Identitas Diagnostik Konsisten:** Pesan pelanggaran dan identifier rule antara mode multichecker dan standalone harus 100% identik.
