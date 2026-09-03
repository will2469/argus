---
name: argus-rule-scaffold
description: "MANDATORY SCAFFOLDING & EXTENSION HARNESS: Rapid, zero-defect scaffolding engine and exact code template generator for adding new Argus Checker rules (ARGUS-A31+). Enforces identical structure across all rules: analyzer.go, companion AST/SQL walkers, 1-SSOT golden corpus fixtures (positive, negative, adversarial), shared directive aliases, runner metadata, and unified dual-mode wiring (preventing the A01 divergent implementation bug). Auto-triggers whenever adding a new rule, implementing ARGUS-A31 to ARGUS-A35 (or beyond), scaffolding an analyzer, wiring standalone runners (scan_go.go/scan_migrations.go), or registering rule aliases."
compatibility: "Go 1.25+, golang.org/x/tools/go/analysis, pg_query_go/v6"
metadata:
  version: "2.0.0"
  author: "Will (https://github.com/will2469)"
  license: "MIT"
---

# Argus Checker Rule Scaffolding Harness (`argus-rule-scaffold`)

Ekstensi dan pembuatan aturan baru untuk **Argus Checker** (Go 1.25+ & PostgreSQL 18.x) wajib mematuhi arsitektur **Zero-Divergence Dual Execution Mode** dan **1-SSOT Golden Corpus Standard**.

---

## 1. Quick Start Generator

Untuk men-generate kerangka aturan baru secara otomatis:

```bash
# Men-generate rule analisis Go AST (misal: ARGUS-A31)
./.agents/skills/argus-rule-scaffold/scripts/scaffold_rule.sh 31 missing_partition_key MISSING_PARTITION_KEY go

# Men-generate rule analisis SQL Migration (misal: ARGUS-A32)
./.agents/skills/argus-rule-scaffold/scripts/scaffold_rule.sh 32 generated_column_indexing GENERATED_COLUMN_INDEXING sql
```

Skrip ini akan secara otomatis membuat:
1. Paket analyzer di `rules/a${NUM}_${NAME}/` lengkap dengan `analyzer.go`, `analyzer_test.go`, dan companion walker.
2. 1-SSOT Golden Corpus di `tests/correctness/a${NUM}/` atau `tests/migration/a${NUM}/` (`positive/`, `negative/`, `adversarial/`).
3. Draf dokumentasi wiki di `wiki/ARGUS-A${NUM}.md`.

---

## 2. Directory & Asset Topology

```text
.agents/skills/argus-rule-scaffold/
├── SKILL.md                          # Alur kerja inti & checklist verifikasi (berkas ini)
├── assets/                           # Templat kode standar
│   ├── analyzer.go.tmpl              # Kerangka analyzer Go Analysis
│   ├── ast_visitor.go.tmpl           # Engine visitor AST Go (SSOT)
│   ├── sql_walker.go.tmpl            # Engine parser SQL AST (SSOT - per file)
│   ├── migration_dir_scanner.go.tmpl # Scanner migrasi rekursif multi-file/subfolder
│   ├── analyzer_test.go.tmpl         # Test suite analysistest.Run (1-SSOT module root)
│   └── wiki_rule.md.tmpl             # Templat dokumentasi 8-Pillars Matrix
├── references/                       # Dokumentasi teknis mendalam
│   ├── wiring_guide.md               # Snippet lengkap 6 titik registrasi sentral
│   └── a01_divergence_prevention.md  # Post-mortem bug A01 & invarian pencegahan
└── scripts/
    └── scaffold_rule.sh              # Generator scaffolding bash otomatis
```

---

## 3. The 7-Step Atomic Scaffolding Checklist

Setiap penambahan rule baru wajib menyelesaikan 7 langkah atomik berikut:

- [ ] **Langkah 1: Scaffolding Boilerplate**
  Jalankan skrip generator di atas atau salin templat dari [`assets/`](assets/).
- [ ] **Langkah 2: Implementasi Logika Deteksi ($\le 250$ Baris/Berkas)**
  Tulis logika pencocokan AST di `ast_visitor.go` (Go) atau `sql_walker.go` (SQL). Sediakan whitelist idiom valid di `exceptions.go` untuk menjamin _Zero False-Positive Target_.
- [ ] **Langkah 3: Lengkapi Fixture 1-SSOT Golden Corpus (`tests/correctness/aXX/` atau `tests/migration/aXX/`)**
  Wajib mencakup matriks 17-pola / M1–M7: (1) `positive/` kasus pelanggaran (`// want`), (2) `negative/` kasus patuh & supresi (`// argus:ignore`), dan (3) `adversarial/` stress-test (closures/interfaces atau AST evasion).
- [ ] **Langkah 4: Daftarkan ke MultiChecker (`rules/rules.go`)**
  Impor paket rule baru dan daftarkan analyzer ke slice `AllAnalyzers`. Lihat [Wiring Guide §1](references/wiring_guide.md#1-multichecker-registration-rulesrulesgo).
- [ ] **Langkah 5: Daftarkan Alias Direktif (`shared/directives/alias.go`)**
  Tambahkan mapping alias ke `ruleAliases`. Lihat [Wiring Guide §2](references/wiring_guide.md#2-directives-alias-registration-shareddirectivesaliasgo).
- [ ] **Langkah 6: Daftarkan Runner Meta & Wiring Standalone**
  - Tambahkan deskripsi ke `CanonicalDescriptions` di [`runner/rules_meta.go`](references/wiring_guide.md#3-runner-metadata-registration-runnerrules_metago).
  - Hubungkan fungsi engine diekspor ke [`runner/scan_go.go`](references/wiring_guide.md#kasus-a-rule-memeriksa-kode-go-runnerscan_gogo) (Go) atau [`runner/scan_migrations.go`](references/wiring_guide.md#kasus-b-rule-memeriksa-migrasi-sql-runnerscan_migrationsgo) (SQL). Dilarang menulis ulang regex!
  - Jika total rule > 30, perbarui batas loop default di [`shared/config/config.go`](references/wiring_guide.md#5-configuration-defaults-sharedconfigconfiggo).
- [ ] **Langkah 7: Publikasi Wiki & Sinkronisasi Katalog**
  Lengkapi [`wiki/ARGUS-AXX.md`](file:///home/will/Monorepo/argus/wiki/) (8-Pillars Matrix), daftarkan di [`wiki/Home.md`](file:///home/will/Monorepo/argus/wiki/Home.md), dan sinkronkan tabel di [`README.md`](file:///home/will/Monorepo/argus/README.md).

---

## 4. Verification Commands (Quality Gate)

Jalankan rangkaian pengujian berikut sebelum commit:

```bash
# 1. Test unit laboratorium rule (Wajib 100% PASS):
go test -v ./rules/aXX_<name>/...

# 2. Test seluruh test suite Argus (Memastikan zero-regression):
go test ./...

# 3. Linter & formatting check:
make lint

# 4. Standalone CLI check (Verifikasi wiring runner):
./bin/argus check --no-report
```
