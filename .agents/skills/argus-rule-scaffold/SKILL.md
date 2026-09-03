---
name: argus-rule-scaffold
description: "MANDATORY SCAFFOLDING & EXTENSION HARNESS: Rapid, zero-defect scaffolding engine and exact code template generator for adding new Argus Checker rules (ARGUS-A31+). Enforces identical structure across all rules: analyzer.go, companion AST/SQL walkers, testdata fixtures, shared directive aliases, runner metadata, and unified dual-mode wiring (preventing the A01 divergent implementation bug). Auto-triggers whenever adding a new rule, implementing ARGUS-A31 to ARGUS-A35 (or beyond), scaffolding an analyzer, wiring standalone runners (scan_go.go/scan_migrations.go), or registering rule aliases."
compatibility: "Go 1.25+, PostgreSQL 18.x, golang.org/x/tools/go/analysis, github.com/pganalyze/pg_query_go/v6, bash"
metadata:
  version: "1.1.0"
  author: "Will (https://github.com/will2469)"
  license: "MIT"
  citations:
    - "Go Analysis Driver Design: golang.org/x/tools/go/analysis"
    - "ArXiv 2607.25032: Authoring Agent Skills: A Software-Engineering Approach"
    - "Argus Checker Architecture: AGENTS.md & argus-rule-workflow"
---

# Argus Rule Scaffolding Harness (`argus-rule-scaffold`)

> **Core Mandate:** Seluruh aturan baru (`ARGUS-A31` s/d `ARGUS-A35+`) wajib mengikuti struktur **100% identik** dengan 30 aturan Argus yang sudah ada. Dilarang keras melakukan re-derivation pola ad-hoc dari nol.
> **Anti-Divergence Invariant (Pelajaran A01):** Analyzer `multichecker` dan standalone CLI runner (`scan_go.go` / `scan_migrations.go`) **WAJIB** memanggil fungsi engine yang sama dari paket rule. Detail latar belakang: lihat [A01 Divergence Prevention](references/a01_divergence_prevention.md).

---

## 1. Quickstart Scaffolding (1 Command)

Gunakan skrip scaffolding otomatis untuk menghasilkan seluruh boilerplate awal:

```bash
# Rule Kode Go:
.agents/skills/argus-rule-scaffold/scripts/scaffold_rule.sh <RULE_NUM> <PKG_NAME> <IDENTIFIER> go
# Contoh:
.agents/skills/argus-rule-scaffold/scripts/scaffold_rule.sh 31 missing_partition_key MISSING_PARTITION_KEY go

# Rule SQL Migrasi:
.agents/skills/argus-rule-scaffold/scripts/scaffold_rule.sh <RULE_NUM> <PKG_NAME> <IDENTIFIER> sql
# Contoh:
.agents/skills/argus-rule-scaffold/scripts/scaffold_rule.sh 32 column_type_narrowing COLUMN_TYPE_NARROWING sql
```

---

## 2. Progressive Disclosure Architecture

Struktur skill ini memisahkan instruksi menjadi 3 tier modular:

```text
.agents/skills/argus-rule-scaffold/
├── SKILL.md                          # Alur kerja inti & checklist verifikasi (berkas ini)
├── assets/                           # Templat kode standar
│   ├── analyzer.go.tmpl              # Kerangka analyzer Go Analysis
│   ├── ast_visitor.go.tmpl           # Engine visitor AST Go (SSOT)
│   ├── sql_walker.go.tmpl            # Engine parser SQL AST (SSOT)
│   ├── analyzer_test.go.tmpl         # Test suite analysistest.Run
│   ├── testdata.go.tmpl              # Fixture 4 skenario testdata
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
- [ ] **Langkah 3: Lengkapi Fixture Laboratorium (`testdata/src/aXX/aXX.go`)**
  Wajib mencakup: (1) Kasus positif, (2) Kasus pelanggaran (`// want`), (3) Kasus supresi (`// argus:ignore-aXX <alasan >= 2 kata>`), dan (4) Kasus edge-case (CTE/alias).
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
