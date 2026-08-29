---
name: argus-rule-workflow
description: "Standard operational procedure (SOP) and execution protocol for implementing and modularizing Argus Checker rules (A01-A30) with the strict Zero-Defect Rule Gate."
compatibility: "Go 1.24+, PostgreSQL 18.x, golang.org/x/tools/go/analysis, pg_query_go"
metadata:
  version: "2.0.0"
  author: "Will (https://github.com/will2469)"
  citations:
    - "Go Analysis Driver Design: golang.org/x/tools/go/analysis"
    - "https://github.com/will2469/argus/wiki"
---

# Argus Checker Rule Execution Workflow & Zero-Defect Gate

> **Core Mandate:** Setiap aturan Argus Checker (`ARGUS-A01` s/d `ARGUS-A30` atau ekstensi aturan baru di masa depan) wajib diimplementasikan secara **modular, plug-and-play, terisolasi, dan tervalidasi 100%**.
> **Larangan Sisir Ulang:** Setiap rule yang dikerjakan harus tuntas sekali jalan dengan cakupan tes 100%, seluruh berkas Go $\le 250$ baris, dan Public Wiki lengkap tanpa kehilangan substansi ilmu (_Zero Loss of Technical Wisdom_). Dilarang keras melompat ke aturan berikutnya sebelum seluruh gerbang kualitas terpenuhi.

---

## 1. The Strict 8-Phase Rule Execution Pipeline

Setiap pengerjaan satu aturan Argus Checker wajib mengikuti siklus state-machine 8 fase berikut tanpa jalan pintas:

````
┌─────────────────────────────────────────────────────────────────────────────┐
│                 STRICT RULE-BY-RULE EXECUTION PIPELINE                      │
├─────────────────────────────────────────────────────────────────────────────┤
│  1. SPECIFICATION GROUNDING                                                 │
│     Baca tuntas spesifikasi teknis dan invarian aturan ARGUS-AXX.           │
│     Identifikasi Invarian, target AST, CWE/ASVS, dan PostgreSQL 18 internals.│
│                                  │                                          │
│                                  ▼                                          │
│  2. MODULAR IMPLEMENTATION (ANTI-FAT CODE <= 250 LINES)                     │
│     Implementasikan subpackage rules/aXX_<name>/:                           │
│     - Batas tegas: maksimal ~250 baris per berkas Go.                       │
│     - Dekomposisi tugas (analyzer.go, ast_visitor.go, exceptions.go, dll.)  │
│     - Import path WAJIB: github.com/will2469/argus/shared/...               │
│     - Gunakan Go docstrings standar yang mandiri (self-contained).          │
│                                  │                                          │
│                                  ▼                                          │
│  3. LAB HARNESS & TESTDATA (100% COVERAGE SCOPE)                            │
│     Buat fixture testdata sentral di testdata/src/aXX/aXX.go.               │
│     Buat analyzer_test.go (analysistest + unit test).                       │
│     Pastikan SEMUA skenario (positif, negatif, ignored) teruji.             │
│                                  │                                          │
│                                  ▼                                          │
│  4. DYNAMIC PLUG-AND-PLAY REGISTRATION                                      │
│     Daftarkan Analyzer ke multichecker di cmd/argus/main.go & rules/rules.go│
│     Tanpa merombak arsitektur inti package shared/.                         │
│                                  │                                          │
│                                  ▼                                          │
│  5. TEST VERIFICATION & ZERO-DEFECT GATE                                    │
│     Jalankan go test pada rule (100% PASS) dan seluruh test suite Argus.    │
│     Verifikasi go vet ./... dan gofmt bersih tanpa peringatan.              │
│                                  │                                          │
│                                  ▼                                          │
│  6. PUBLIC WIKI PUBLICATION (ZERO LOSS OF WISDOM)                           │
│     Tulis wiki/ARGUS-AXX.md sesuai 8-Pillars Matrix:                        │
│     - Pertahankan seluruh grounding engine PostgreSQL 18 & analisis risiko. │
│     - Gunakan visual diagram ```mermaid.                                    │
│     - Jaga perimeter sanitasi: bebas path/istilah privat monorepo.          │
│                                  │                                          │
│                                  ▼                                          │
│  7. CATALOG SYNCHRONIZATION                                                 │
│     Tautkan ARGUS-AXX di wiki/Home.md.                                      │
│     Pastikan daftar aturan di README.md selalu sinkron.                     │
│                                  │                                          │
│                                  ▼                                          │
│  8. QUALITY GATE PASS                                                       │
│     Jalankan `make lint` dan `make test` -> 100% Hijau.                     │
│     BARU DIIZINKAN LANJUT KE RULE BERIKUTNYA.                               │
└─────────────────────────────────────────────────────────────────────────────┘
````

---

## 2. Directory & Module Anatomy

Setiap aturan diisolasi penuh di bawah `rules/aXX_<name>/`:

```
rules/
└── aXX_<name>/
    ├── analyzer.go          # Exported var Analyzer (*analysis.Analyzer, <= 250 baris)
    ├── <sub_domain_1>.go    # Logika spesifik AST/Engine (<= 250 baris)
    ├── <sub_domain_2>.go    # Logika evaluator/registry/exceptions (<= 250 baris)
    └── analyzer_test.go     # Test suite analysistest + unit test (<= 250 baris)
testdata/
└── src/
    └── aXX/
        └── aXX.go           # Central testdata fixture (compliant, violations, ignore)
```

### Standar Pembagian Berkas Sub-Domain:

- **`analyzer.go`**: Definisi metadata `analysis.Analyzer`, registrasi dependensi (`directives.Analyzer`, `config.Analyzer`), dan entry point traversal `run`.
- **`ast_visitor.go` / `grant_ast_walker.go` / `loop_walker.go`**: Penelusuran AST Go atau SQL AST (`pg_query_go`).
- **`exceptions.go` / `size_evaluator.go` / `role_registry.go`**: Evaluator batasan, whitelist pengecualian, atau integrasi konfigurasi `.argus.yaml`.
- **`standalone_runner.go`**: Runner mandiri jika aturan memeriksa file migrasi `.sql` di luar Go AST.
- **`analyzer_test.go`**: Memuat:
  1. `TestAnalyzer`: Menjalankan `analysistest.Run` pada `testdata/src/aXX/aXX.go`.
  2. Unit test domain: Menguji fungsi parser/evaluator secara presisi.

---

## 3. Dynamic Plug-and-Play Rules Registration

Arsitektur Argus Checker dirancang agar **skalabel hingga puluhan atau ratusan aturan (A01-A100+)** tanpa memerlukan refactor pada engine inti (`shared/`):

1. **Inti Terisolasi (`shared/`):**
   Modul `shared/callsite`, `shared/config`, `shared/directives`, `shared/migration`, dan `shared/sqlparser` bersifat agnostik terhadap rule. Dilarang memasukkan logika khusus satu rule ke dalam package `shared/`.

2. **Registrasi Rule Baru di `rules/rules.go` dan CLI:**
   Cukup impor rule baru dan daftarkan analyzer-nya ke daftar `AllAnalyzers`:

   ```go
   import (
       "github.com/will2469/argus/rules/aXX_name"
   )

   var AllAnalyzers = []*analysis.Analyzer{
       // ...
       aXX_name.Analyzer,
   }
   ```

3. **Registrasi Rule Migrasi di `runner/` (Khusus Aturan File SQL):**
   Jika rule memeriksa berkas `.sql`, panggil fungsi scanner mandiri rule tersebut di `runner/scan_migrations.go` tanpa mengubah struktur pipeline runner.

---

## 4. Standard Analyzer Implementation Template

> [!TIP]
> Seluruh dokumentasi kode Argus harus murni dan mandiri (_self-contained Go docstrings_) yang informatif bagi komunitas open-source.

```go
// Package aXX_name implements the ARGUS-AXX static analyzer.
package aXX_name

import (
	"go/ast"
	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

// RuleCode is the official identifier for ARGUS-AXX.
const RuleCode = "ARGUS-AXX"

// Analyzer defines the analysis.Analyzer for ARGUS-AXX.
var Analyzer = &analysis.Analyzer{
	Name: "argus_aXX_identifier",
	Doc:  "Human-readable brief description of invariant",
	Run:  run,
	Requires: []*analysis.Analyzer{
		directives.Analyzer,
		config.Analyzer,
	},
}

func run(pass *analysis.Pass) (interface{}, error) {
	cfg := pass.ResultOf[config.Analyzer].(*config.Config)
	if !cfg.IsRuleEnabled(RuleCode) {
		return nil, nil
	}

	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)

	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			// Evaluasi pelanggaran AST Go / SQL
			// Jika melanggar:
			// if dm != nil && dm.IsIgnored(pass.Fset, n.Pos(), RuleCode) { return true }
			// pass.Reportf(n.Pos(), "[%s] Diagnostic message (CWE-XXX)", RuleCode)
			return true
		})
	}
	return nil, nil
}
```

---

## 5. Public Wiki Standard (Zero Loss of Technical Wisdom)

Saat menerbitkan berkas `wiki/ARGUS-AXX.md`, gunakan standar **8-Pillars Matrix** (Skill: `argus-wikis-builder`).

### Invarian Kualitas Wiki:

- **Zero Loss of Wisdom:** Seluruh penjelasan teknis mendalam (perilaku engine PostgreSQL 18, subsistem buffer cache, lock conflict matrix, TOAST, SSI, plan invalidation, CWE/ASVS) **wajib dituangkan secara utuh** ke dalam wiki publik. Dilarang menyunat penjelasan menjadi sekadar ringkasan dangkal.
- **Diagram Mermaid Wajib:** Wajib memuat minimal 1 flowchart visual perbandingan (Hazardous vs Compliant) dan diagram arsitektur deteksi AST.
- **Nama Domain Netral:** Seluruh nama entitas menggunakan nama domain umum/netral (`accounts`, `orders`, `users`, `audit_logs`).
- **Katalog Terhubung:** Setiap wiki yang terbit wajib langsung ditautkan di `wiki/Home.md`.

---

## 6. Verification Commands

Jalankan perintah verifikasi ini pada setiap tahap siklus kerja:

```bash
# 1. Menjalankan pengujian laboratorium rule tertentu (Wajib 100% PASS):
go test -v ./rules/aXX_<name>/...

# 2. Menjalankan seluruh test suite Argus Checker:
go test ./...

# 3. Menjalankan linting dan formatting check:
make lint

# 4. Melakukan kompilasi binary CLI:
make build
```
