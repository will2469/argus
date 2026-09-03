---
name: argus-dual-path-parity-check
description: "MANDATORY DUAL-PATH PARITY GUARDIAN: Rigorous parity verification protocol and anti-divergence guardrail for Argus rules executing across both the official Analysis Driver (go vet / multichecker) and the standalone CLI scanner (runner/scan_go.go / scan_migrations.go). Enforces the strict zero-duplication rule ('never reimplement heuristics using regex in standalone runner if rule package uses AST'), mandates output comparison against identical fixtures, and eliminates silent divergences (the root cause of the A01 dead-code and shallow-migration scan bugs). Auto-triggers after creating, modifying, or reviewing any Argus rule with dual execution paths."
compatibility: "Go 1.25+, PostgreSQL 18.x, golang.org/x/tools/go/analysis, bash"
metadata:
  version: "1.0.0"
  author: "Will (https://github.com/will2469)"
  license: "MIT"
  citations:
    - "Go Analysis Driver Design: golang.org/x/tools/go/analysis"
    - "Argus Post-Mortem: Dual-Path Divergence (A01 Dead Code & Shallow Scans)"
    - "ArXiv 2607.25032: Authoring Agent Skills: A Software-Engineering Approach"
---

# Argus Dual-Path Parity Check (`argus-dual-path-parity-check`)

> **The Iron Parity Invariant:**
> *"Dilarang keras mengimplementasikan ulang heuristik deteksi menggunakan regex baru di jalur standalone runner (`runner/`) jika paket rule aslinya sudah menggunakan AST — REUSE, JANGAN DUPLIKAT."*
> Logika deteksi adalah **Single Source of Truth (SSOT)**. Jika `go vet` dan `argus check` menghasilkan output berbeda pada berkas yang sama, linter kehilangan integritasnya.

---

## 1. The 4-Step Mandatory Parity Verification Protocol

Setiap kali menambah atau memodifikasi rule, jalankan protokol 4 tahap ini:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                 DUAL-PATH PARITY VERIFICATION WORKFLOW                      │
├─────────────────────────────────────────────────────────────────────────────┤
│ 1. [INSPECT ENGINE]      Pastikan rules/aXX/ mengekspor fungsi evaluasi     │
│                          (InspectFile atau CheckMigration).                 │
│                                  │                                          │
│ 2. [AUDIT RUNNER]        Periksa runner/scan_go.go atau scan_migrations.go: │
│                          Apakah memanggil fungsi engine? Dilarang ada regex!│
│                                  │                                          │
│ 3. [RUN PARITY SCRIPT]   Eksekusi pengujian otomatis pada fixture sentral:   │
│                          .agents/skills/argus-dual-path-parity-check/assets/│
│                          run_parity_check.sh <XX> <name>                    │
│                                  │                                          │
│ 4. [ASSERT EQUALITY]     Verifikasi: Jumlah issue, nomor baris, dan nama     │
│                          rule di kedua mode 100% IDENTIK.                   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Checklist Verifikasi Paritas:
- [ ] **Zero-Regex Check:** Tidak ada ekspresi reguler tiruan yang ditambahkan ke `runner/scan_go.go` atau `runner/scan_migrations.go`.
- [ ] **Engine Export Check:** Paket `rules/aXX_<name>/` mengekspor fungsi evaluasi yang menerima `*analysis.Pass` (yang aman jika bernilai `nil`).
- [ ] **Fixture Test Parity:** Menjalankan `run_parity_check.sh` membuktikan bahwa `argus check` mendeteksi jumlah pelanggaran yang sama dengan `analysistest`.
- [ ] **Recursive Directory Check:** Pemindaian migrasi SQL tidak menggunakan `os.ReadDir` dangkal jika subdirektori didukung.

---

## 2. Progressive Disclosure Map

Struktur skill ini memisahkan pengujian paritas menjadi aset dan referensi terarah:

```text
.agents/skills/argus-dual-path-parity-check/
├── SKILL.md                          # Prosedur paritas & checklist (berkas ini)
├── assets/                           # Perkakas pengujian otomatis
│   ├── dual_path_harness_test.go.tmpl# Unit test paritas driver vs runner
│   └── run_parity_check.sh           # Skrip CLI pembanding kedua mode eksekusi
└── references/                       # Analisis mendalam & post-mortem
    ├── a01_and_migration_divergence.md # Post-mortem kegagalan A01 & migrasi dangkal
    └── shared_engine_design_pattern.md # Pola desain kode Go pass == nil vs pass != nil
```

---

## 3. Parity Failure Triage Matrix

Jika pengujian paritas gagal, identifikasi gejalanya di bawah ini:

| Gejala Paritas | Kemungkinan Akar Masalah | Solusi Cepat |
| :--- | :--- | :--- |
| `analysistest` PASS, tapi standalone `argus check` melaporkan **0 issues** (Kasus A01). | Standalone runner menggunakan regex terpisah yang tidak cocok dengan AST, atau rule lupa didaftarkan ke `scan_go.go`. | Hapus regex ad-hoc di `runner/`. Panggil fungsi `InspectFile` yang diekspor paket rule. |
| Standalone CLI melewatkan berkas migrasi di dalam subfolder (misal `migrations/v1/`). | Runner menggunakan `os.ReadDir` dangkal alih-alih penelusuran rekursif `filepath.Walk`. | Gunakan fungsi penelusuran berkas rekursif konsisten seperti `findFilesWithExt`. |
| Nama rule di laporan audit berbeda dengan di terminal (`Reportf`). | Terdapat ketidakcocokan antara string format `Reportf` dan `CanonicalDescriptions` di `rules_meta.go`. | Samakan identifier kanonikal di `rules_meta.go` dan konstanta `RuleCode`. |

---

## 4. Quick Verification Command

```bash
# Jalankan verifikasi paritas otomatis pada rule yang baru diubah:
.agents/skills/argus-dual-path-parity-check/assets/run_parity_check.sh <RULE_NUM> <RULE_NAME>
# Contoh:
.agents/skills/argus-dual-path-parity-check/assets/run_parity_check.sh 14 select_star
```
Semua rule wajib lolos verifikasi paritas sebelum dinyatakan selesai.
