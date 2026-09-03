# Changelog Detection, Curation, and Redaction Guide
**Standard:** Keep a Changelog 1.1.0 & GitHub Releases Best Practices  
**Target Project:** Argus Checker (`github.com/will2469/argus`)

---

## 1. Taksonomi: Mana yang Harus Ditampilkan vs Dieliminasi

Release Notes ditulis untuk **manusia (software engineer, DevOps, operator)** yang menggunakan Argus, **bukan dump mentah git log**.

```
                           [ SEMUA COMMIT GIT DIFF ]
                                      │
                 ┌────────────────────┴────────────────────┐
                 ▼                                         ▼
     [ USER-FACING VALUE ]                       [ INTERNAL NOISE / SLOP ]
    (WAJIB DITAMPILKAN)                         (REDAKSI / GABUNG / DROP)
                 │                                         │
  • Breaking Changes (`feat!:`, `BREAKING`)      • In-flight bug fixes (fix fitur unreleased)
  • Fitur Baru & Rules Baru (`feat:`)            • Micro-typo commits (`docs: fix typo`)
  • Fix bug nyata di produksi (`fix:`)           • Lint / Formatting / Whitespace (`gofmt`)
  • Optimasi performa signifikan (`perf:`)       • Dependabot bump (`build(deps): bump...`)
  • Deprekasi fungsi / rule (`Deprecated:`)      • Minor refactor tanpa efek samping (`clean code`)
```

### Matriks Klasifikasi Tampilan

| Kategori | Status | Kriteria & Tindakan | Contoh Commit Argus |
| :--- | :--- | :--- | :--- |
| **🚨 Breaking Changes** | **WAJIB (Paling Atas)** | Perubahan API, CLI flag, format config yang memutus kode konsumen. Wajib sertakan panduan migrasi. | `feat!: change --format flag to accept enum only` |
| **🚀 New Features** | **WAJIB** | Rule baru (`A31+`), CLI subcommands, endpoint MCP baru. | `feat: implement core Model Context Protocol (MCP) framework` |
| **🐛 Bug Fixes** | **TERKURASI** | Memperbaiki false-positive (FP), false-negative (FN), crash, atau leak yang dialami pengguna. | `fix: remove strings.ReplaceAll from A26 sanitizer whitelist` |
| **⚡ Performance** | **TERKURASI** | Pengurangan latensi atau alokasi memori yang terukur. | `perf: memoize pg_query AST parsing across nested visitors` |
| **⚠️ Deprecations** | **WAJIB** | Fitur/rule yang ditandai deprecated untuk persiapan rilis Major berikutnya. | `feat: deprecate legacy rule flag in favor of .argus.yaml` |
| **🔧 Internal Hygiene** | **DIRINGKAS (1 Baris)** | Adopsi golden test, penambahan target Makefile, semgrep. | *Gabungkan 20+ commit test menjadi 1 bullet point.* |
| **❌ Micro-noise / Typo** | **DROP TOTAL** | Typos, whitespace, commit perbaikan coba-coba saat develop branch. | `fix: typo in comment`, `fix: test failing on CI` |

---

## 2. Aturan Redaksi Bug Kecil (*Minor Bugs Redaction*)

### Aturan 1: *The In-Flight Fix Elimination Rule*
> **Prinsip:** Jika sebuah bug muncul dan diperbaiki dalam siklus pengembangan fitur yang **belum pernah dirilis ke publik**, bug tersebut **DILARANG** dimasukkan ke dalam release notes sebagai "Bug Fix".

- *Kasus Nyata:*
  - Commit A: `feat: implement MCP server tool registry`
  - Commit B: `fix: fix nil pointer in MCP server tool registry` (dibuat 1 jam setelah commit A)
- *Tindakan:* Hapus Commit B dari release notes! Bagi publik, fitur MCP dirilis dalam keadaan langsung berfungsi, bukan "kami buat rusak lalu kami perbaiki sebelum rilis".

### Aturan 2: *Cohesive Synthesis (Penggabungan Commit Terdistribusi)*
> **Prinsip:** Jangan mengekspos 10 commit mikro berturut-turut. Sintesiskan menjadi satu kalimat teknis yang berbobot.

- *Mentah (AI Slop / Raw Git Log):*
  ```text
  - feat(a09): adopt 1-SSOT golden corpus
  - feat(a10): adopt 1-SSOT golden corpus
  - ... (22 commit berulang)
  - feat(a30): adopt 1-SSOT golden corpus
  ```
- *Redaksi Bersih (Curated):*
  ```markdown
  * **1-SSOT Golden Corpus Adoption:** Migrated all 30 Argus rules (ARGUS-A01 through ARGUS-A30) to the single-source-of-truth adversarial test suite with 17-pattern resilience validation.
  ```

---

## 3. Siklus Hidup Dokumen: `release_notes.md` vs `CHANGELOG.md`

Pertanyaan krusial: *"Terus rilis lama gimana?"*

### Arsitektur Dua Dokumen (Two-Document Architecture)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           ARSITEKTUR DOKUMEN RILIS                          │
│                                                                             │
│  [release_notes.md]  ──► HANYA BERISI DRAF RILIS SAAT INI (Active Target)   │
│                          Digunakan untuk GitHub Release body, copy-paste    │
│                          pengumuman rilis, atau release automation bot.     │
│                                      │                                      │
│                                      ▼ (Saat rilis disahkan / git tag)      │
│  [CHANGELOG.md]      ──► BUKU BESAR AKUMULATIF HISTORIS (Living Ledger)     │
│                          Rilis baru di-PREPEND di paling atas.              │
│                          Semua rilis terdahulu (v1.0.0, v0.9.0) TETAP ADA   │
│                          dan tersimpan utuh di bawahnya.                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Prosedur "Prepend" Rilis Baru:
1. Ketika rilis baru (misal `v1.1.0`) siap diterbitkan:
2. Baca draf dari `release_notes.md`.
3. Buka `CHANGELOG.md`. Sisipkan blok `## [v1.1.0] - YYYY-MM-DD` tepat di bawah judul `# Changelog` dan di atas entri rilis lama `## [v1.0.0]`.
4. Rilis `v1.0.0` tidak pernah ditimpa atau dihapus!
5. Perbarui `release_notes.md` agar mencerminkan ringkasan rilis `v1.1.0`.

---

## 4. Format Standar Release Notes (`release_notes.md`)

```markdown
# Argus v1.1.0 — Model Context Protocol & 1-SSOT Hardening (2026-09-04)

Argus v1.1.0 introduces native Model Context Protocol (MCP) server support, kernel-enforced filesystem security containment, and complete 1-SSOT golden corpus test adoption across all 30 database safety rules.

---

### 🚀 New Features & Capabilities
* **Model Context Protocol (MCP 2026-07-28):** Native stateless JSON-RPC / stdio server exposing `argus_scan`, `argus_check_migration`, `argus_explain_rule`, and `argus_report_issue` tools.
* **Kernel-Enforced Filesystem Containment:** Sandboxed directory traversal using `RootCapability` to prevent path traversal vulnerabilities.
* **Adoption of 1-SSOT Golden Corpus:** Completed full migration of rules `ARGUS-A01` through `ARGUS-A30` to the standardized adversarial test harness.

---

### 🐛 Bug Fixes & Diagnostic Precision
* **ARGUS-A26 (LIKE Wildcard Injection):** Removed `strings.ReplaceAll` from sanitizer allowlist to eliminate false-negative bypasses.
* **SQL Migration Scanner:** Introduced strict parsing mode (`E001`) for graceful reporting of unparseable SQL migrations.
* **ARGUS-A01 (SQL Concatenation):** Hardened AST-based taint analysis to handle untyped receiver calls and dynamic arguments.

---

### ⚡ Performance & Internal Hygiene
* Refactored `shared/sqlparser` AST normalizer to avoid redundant query parsing.
* Added `govulncheck` and Semgrep static analysis into pre-commit validation.

---

### 📦 Installation & Verification
\`\`\`bash
go install github.com/will2469/argus/cmd/argus@v1.1.0
\`\`\`
```
