# Dual-Path Divergence Post-Mortem: A01 Dead Code & Shallow Migration Scans

Dokumentasi historis dari dua insiden divergensi arsitektur nyata di Argus, mengapa hal tersebut terjadi, dan bagaimana aturan paritas mencegahnya.

---

## 1. Kasus 1: ARGUS-A01 (Mati Total di Mode Standalone)

### Gejala Insiden:
Pada pengujian rule via `go test ./rules/a01_sql_concat/...`, seluruh test fixture di `testdata/src/a01/a01.go` lulus 100%. Namun saat pengguna menjalankan binary CLI standalone:
```bash
argus check --dirs ./testdata/src/a01
```
Hasil scan melaporkan **0 issues (PASS)** padahal berkas tersebut memuat pelanggaran SQL concatenation terang-terangan!

### Akar Penyebab:
Di `runner/scan_go.go`, pengembang tidak memanggil fungsi engine dari `rules/a01_sql_concat`, melainkan membuat regex tiruan baru:
```go
// KODE RUSAK DI runner/scan_go.go:
var sqlUnsafeConcatRegex = regexp.MustCompile(`(?i)(?:["` + "`" + `]\s*(?:SELECT|INSERT...)[^"` + "`" + `]*["` + "`" + `]\s*\+\s*(?:req\.|input\...))`)
// ...
case *ast.BasicLit:
    if sqlUnsafeConcatRegex.MatchString(val) { ... }
```
Dalam Go AST:
- `*ast.BasicLit` mewakili string literal tunggal murni, misalnya `"SELECT * FROM users WHERE id = "`.
- Penggabungan dengan variabel `+ req.ID` berada di node induk bertipe `*ast.BinaryExpr` dengan `Op == token.ADD`.
- Nilai `val` dari `BasicLit` **tidak akan pernah mengandung string `+ req.`**!
- Akibatnya regex di atas menjadi **kode mati (dead code)**, dan deteksi standalone A01 mati total secara diam-diam.

---

## 2. Kasus 2: Shallow Migration Directory Scanning

### Gejala Insiden:
Pada pengerjaan rule migrasi SQL (`A11`, `A15`, `A27`, `A28`, `A30`), file migrasi yang berada di dalam subdirektori (misal `migrations/2026/01_init.up.sql` atau `migrations/v1/02_users.up.sql`) terdeteksi saat ditargetkan secara langsung, namun terlewatkan saat pemindaian folder induk.

### Akar Penyebab:
Di `runner/scan_migrations.go` dan `shared/migration/scanner.go`:
```go
entries, err := os.ReadDir(targetDir)
for _, entry := range entries {
    if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
        continue // Subdirektori dilewati tanpa penelusuran rekursif!
    }
}
```
Sedangkan pada scanner file Go (`runner/runner.go`), pemindaian dilakukan secara rekursif via `filepath.Walk` (`findFilesWithExt`).
Perbedaan paradigma pemindaian antara kode Go (rekursif) dan migrasi SQL (shallow `os.ReadDir`) menyebabkan berkas migrasi bertingkat terabaikan secara diam-diam.

---

## 3. Invarian Paritas Jalur Ganda (Dual-Path Parity Rule)

1. **Strict Zero-Duplication:** Dilarang keras menulis ulang ekspresi reguler (regex) di `runner/scan_go.go` atau `runner/scan_migrations.go` untuk menduplikasi logika yang sudah ada di paket rule `rules/aXX/`.
2. **Re-use Exported Engine:** Setiap paket rule wajib mengekspor fungsi evaluasi yang menerima `*analysis.Pass` (yang boleh `nil`) atau signature input murni (`ast.Node`, `content string`).
3. **Automated Parity Test:** Setiap rule baru wajib memverifikasi bahwa pengujian terhadap `testdata/src/aXX/aXX.go` menghasilkan deteksi yang identik baik pada mode analysistest maupun mode runner standalone.
