---
name: argus-versioning
description: "MANDATORY SEMVER 2.0.0 RELEASE & DIFF GUARDIAN FOR ARGUS: Deterministic version increment evaluator and breaking change inspector. Auto-triggers when evaluating version bumps, checking git diff from latest tag to HEAD, auditing API backward compatibility, reviewing breaking changes in CLI/rules/config/MCP, or preparing release tags for Argus Checker ('argus versioning', 'semver', 'version bump', 'major minor patch', 'tag diff', 'breaking change check', 'release check')."
compatibility: "Go 1.25+, git, bash, golang.org/x/tools/go/analysis"
metadata:
  version: "1.0.0"
  author: "Will (https://github.com/will2469)"
  license: "MIT"
  citations:
    - "Semantic Versioning 2.0.0 Specification (https://semver.org/spec/v2.0.0.html) by Tom Preston-Werner"
    - "Conventional Commits 1.0.0 (https://www.conventionalcommits.org/)"
    - "ArXiv 2607.25032: Authoring Agent Skills: A Software-Engineering Approach"
---

# Argus Semantic Versioning & Diff Guardian (`argus-versioning`)

> **Core Thesis:** Penentuan nomor rilis pada **Argus Checker** (`github.com/will2469/argus`) tunduk sepenuhnya pada **Semantic Versioning 2.0.0 (SemVer)**. Evaluasi kenaikan versi (`MAJOR.MINOR.PATCH`) wajib dihitung secara deterministik dari perubahan nyata (*ground-truth diff*) antara **tag rilis terakhir (`LATEST_TAG`)** dan **state saat ini (`HEAD`)**, menjaga integritas kontrak API publik tanpa asumsi spekulatif.

---

## 1. Fondasi Spesifikasi Resmi SemVer 2.0.0

Format versi stabil Argus adalah triplet numerik **`MAJOR.MINOR.PATCH`** (contoh: `1.4.2`). Tanpa kompleksitas pra-rilis (`alpha`/`beta`/`rc`), setiap angka memiliki mandat klausul resmi:

```
      X   .   Y   .   Z
    MAJOR   MINOR   PATCH
      │       │       │
      │       │       └─► Clause 5: Backward-compatible bug/security fix
      │       └─────────► Clause 6: Backward-compatible new features / deprecations
      └─────────────────► Clause 7: Incompatible API breaking changes
```

### Tabel Sitasi Klausul Otoritatif (semver.org)

| Komponen | Klausul | Kutipan Dokumen Resmi Spesifikasi SemVer 2.0.0 | Dampak Penomoran |
| :--- | :--- | :--- | :--- |
| **MAJOR (X)** | **Clause 7** | *"Major version X (X.y.z \| X > 0) MUST be incremented if any backwards incompatible changes are introduced to the public API. It MAY also include minor and patch level changes. Patch and minor version MUST be reset to 0 when major version is incremented."* | `1.4.2` → **`2.0.0`** *(Reset Minor & Patch ke 0)* |
| **MINOR (Y)** | **Clause 6** | *"Minor version Y (x.Y.z \| x > 0) MUST be incremented if new, backwards compatible functionality is introduced to the public API. It MUST be incremented if any public API functionality is marked as deprecated... Patch version MUST be reset to 0 when minor version is incremented."* | `1.4.2` → **`1.5.0`** *(Reset Patch ke 0)* |
| **PATCH (Z)** | **Clause 5** | *"Patch version Z (x.y.Z \| x > 0) MUST be incremented if only backwards compatible bug fixes are introduced. A bug fix is defined as an internal change that fixes incorrect behavior."* | `1.4.2` → **`1.4.3`** *(Hanya Patch naik)* |

> Rujukan teks klausul lengkap: [semver-2.0.0-spec-clauses.md](file:///home/will/Monorepo/argus/.agents/skills/argus-versioning/references/semver-2.0.0-spec-clauses.md)

---

## 2. Batas Kontrak API Publik Argus (*Public API Boundary*)

Berdasarkan **Clause 4**, penentuan breaking change di Argus dievaluasi terhadap **5 Permukaan Kontrak Publik**:

1. **CLI Commands & Flags (`cmd/argus/`):** Perintah `argus check`, `argus check-migrations`, flags (`--config`, `--format`, `--strict`), exit codes, dan skema format keluaran JSON/SARIF.
2. **Skema Konfigurasi (`.argus.yaml` & `shared/config/`):** Struktur kunci YAML, aturan validasi, dan severity default.
3. **Aturan Analyzer (`rules/`):** ID kode aturan (`ARGUS-A01` s/d `ARGUS-A30+`), format pesan diagnostik, dan semantik pelaporan baris error.
4. **Model Context Protocol / MCP Server:** Nama tools (`argus_scan`, `argus_check_migration`, `argus_explain_rule`, `argus_report_issue`), skema input JSON, dan format response metadata `_meta`.
5. **Pustaka Bersama Publik (`shared/`):** Signature fungsi, tipe data, dan interface publik yang diekspor untuk konsumsi eksternal.

---

## 3. Pohon Keputusan Evaluasi Diff (*Decision Tree*)

```
              [ EVALUASI GIT DIFF: LATEST_TAG...HEAD ]
                                 │
     Apakah terdapat perubahan API Publik yang TIDAK KOMPATIBEL?
     (Hapus flag/subcommand CLI, ubah skema .argus.yaml, hapus tool MCP,
      ubah ID rule, ubah signature public di shared/, 'feat!:' / 'BREAKING CHANGE:')
                                 │
                 ┌───────────────┴───────────────┐
                YA                              TIDAK
                 │                               │
                 ▼                               ▼
         ┌───────────────┐        Apakah terdapat FITUR BARU kompatibel
         │  BUMP MAJOR   │        atau DEPREKASI baru?
         │ 1.4.2 → 2.0.0 │        (Tambah rule baru A31+, tambah flag CLI,
         └───────────────┘        tambah tool MCP, '// Deprecated:', commit 'feat:')
                                                 │
                                 ┌───────────────┴───────────────┐
                                YA                              TIDAK
                                 │                               │
                                 ▼                               ▼
                         ┌───────────────┐        Apakah terdapat BUG FIX,
                         │  BUMP MINOR   │        FP/FN FIX, atau OPTIMASI?
                         │ 1.4.2 → 1.5.0 │        (Fix AST traversal, whitelist FP,
                         └───────────────┘        optimasi cache pg_query, doc/test)
                                                                 │
                                                 ┌───────────────┴───────────────┐
                                                YA                              TIDAK
                                                 │                               │
                                                 ▼                               ▼
                                         ┌───────────────┐               ┌───────────────┐
                                         │  BUMP PATCH   │               │    NO BUMP    │
                                         │ 1.4.2 → 1.4.3 │               │   (Unchanged) │
                                         └───────────────┘               └───────────────┘
```

---

## 4. Kriteria Penentuan Berdasarkan Diff Tag Terakhir

### A. Kriteria MAJOR (`1.x.x` → `2.0.0`)
*(Klausul 7: Incompatible API Changes)*
Terjadi jika diff memperkenalkan perubahan pemecah kontrak pada konsumen Argus:
1. **CLI Breaking Changes:**
   - Menghapus perintah (`check`, `check-migrations`) atau flag yang ada (`--format`, `--config`).
   - Mengubah format mesin stdout (`--format=json` / `--format=sarif`).
   - Mengubah kode exit CLI untuk status kegagalan/keberhasilan.
2. **Konfigurasi (`.argus.yaml`):**
   - Menghapus kunci konfigurasi atau mengubah tipe nilai kunci yang sudah ada.
   - Menambahkan kunci wajib baru yang menyebabkan file konfigurasi lama gagal divalidasi.
3. **Analyzer & Rules:**
   - Menghapus aturan analyzer atau mengubah ID kode aturan (misal mengubah `ARGUS-A01` menjadi ID lain).
4. **MCP Server:**
   - Menghapus tool MCP atau mengubah nama parameter wajib pada JSON schema input tool.
5. **Runtime & Dependensi:**
   - Menaikkan versi minimum Go toolchain (misal mewajibkan Go 1.27 dari sebelumnya Go 1.25).
6. **Commit Tag:**
   - Commit memuat `!` (`feat!:`, `fix!:`) atau body memuat `BREAKING CHANGE:`.

---

### B. Kriteria MINOR (`1.4.0` → `1.5.0`)
*(Klausul 6: Backward-Compatible New Features & Deprecations)*
Terjadi jika diff menambahkan kemampuan baru tanpa memutus alur yang sudah berjalan:
1. **Aturan Baru:**
   - Menambahkan rule analyzer baru (misal mengimplementasikan `ARGUS-A31` s/d `ARGUS-A35`).
2. **CLI & Konfigurasi Ekstensi:**
   - Menambahkan flag CLI baru yang bersifat opsional dengan nilai default yang aman.
   - Menambahkan opsi konfigurasi baru di `.argus.yaml`.
3. **Fitur MCP:**
   - Menambahkan MCP tool baru atau property opsional baru pada tool schema.
4. **Deprekasi Terencana (*Clause 6 Mandate*):**
   - Menandai rule, fungsi shared, atau flag sebagai deprecated via komentar `// Deprecated:`.
   - *Prinsip:* Deprekasi **WAJIB** menaikkan MINOR (bukan Major) untuk memberikan periode transisi aman bagi pengguna.
5. **Commit Tag:**
   - Commit diawali dengan `feat:` atau `feat(...)`: tanpa tanda seru `!`.

---

### C. Kriteria PATCH (`1.4.1` → `1.4.2`)
*(Klausul 5: Backward-Compatible Bug Fixes)*
Terjadi jika diff hanya memperbaiki bug internal, merapikan performa, atau menambah pengujian:
1. **Perbaikan FP / FN Analyzer:**
   - Memperbaiki false-positive (FP) pada AST traversal (misal mendukung subquery `EXISTS` di A14).
   - Menambahkan whitelist idiom standar (misal `COUNT(*)` pada A16).
2. **Optimasi & Hygiene:**
   - Mempercepat parsing SQL via caching AST di `shared/sqlparser/`.
   - Menutup memory leak atau goroutine leak.
3. **Pengujian & Dokumentasi:**
   - Menambahkan test case di 1-SSOT Golden Corpus (`tests/correctness/`, `tests/migration/`).
   - Memperbarui dokumentasi di `wiki/` atau `README.md`.
4. **Commit Tag:**
   - Commit diawali dengan `fix:`, `perf:`, `refactor:`, `docs:`, `test:`, `chore:`.

> Matriks taksonomi lengkap: [argus-diff-criteria-matrix.md](file:///home/will/Monorepo/argus/.agents/skills/argus-versioning/references/argus-diff-criteria-matrix.md)

---

## 5. Presedensi & Invariant Reset

$$\mathbf{MAJOR > MINOR > PATCH}$$

* **Jika ada perubahan BREAKING:**
  * Bump: **MAJOR** (`1.4.2` → **`2.0.0`**).
  * **Reset Rule (Clause 7):** Minor dan Patch **wajib di-reset ke 0**.
* **Jika TIDAK ADA Breaking, tetapi ada FITUR BARU / DEPREKASI:**
  * Bump: **MINOR** (`1.4.2` → **`1.5.0`**).
  * **Reset Rule (Clause 6):** Patch **wajib di-reset ke 0**.
* **Jika HANYA ADA perbaikan bug / hygiene:**
  * Bump: **PATCH** (`1.4.2` → **`1.4.3`**).

---

## 6. Prosedur Eksekusi Otomatis

Jalankan skrip inspeksi deterministik dari root repositori:

```bash
# 1. Analisis diff dari tag terakhir ke HEAD (Rekomendasi Bump)
./.agents/skills/argus-versioning/scripts/argus-diff-inspector.sh

# 2. Mode JSON untuk integrasi pipeline CI/CD
./.agents/skills/argus-versioning/scripts/argus-diff-inspector.sh --json

# 3. Generate Release Notes terkurasi (Dry-run pratinjau)
./.agents/skills/argus-versioning/scripts/generate-release-notes.sh

# 4. Tulis ke release_notes.md dan prepend ke CHANGELOG.md (Preservasi Rilis Lama)
./.agents/skills/argus-versioning/scripts/generate-release-notes.sh --write
```

> Panduan kurasi changelog & redaksi bug kecil: [changelog-curation-and-redaction.md](file:///home/will/Monorepo/argus/.agents/skills/argus-versioning/references/changelog-curation-and-redaction.md)

---

## 7. Checklist Verifikasi Rilis Argus

- [ ] **1. Identifikasi Tag Basis:** `git describe --tags --abbrev=0` berhasil membaca tag acuan.
- [ ] **2. Three-Dot Diff:** Evaluasi diff menggunakan `git diff ${LATEST_TAG}...HEAD`.
- [ ] **3. Audit 5 Permukaan Publik:** Verifikasi tidak ada breaking change pada CLI, `.argus.yaml`, Rules, MCP, dan `shared/`.
- [ ] **4. Audit Deprekasi:** Fitur yang diberi status `Deprecated` mendapatkan bump MINOR (bukan Major).
- [ ] **5. Invariant Reset:** Nilai minor/patch di-reset ke 0 sesuai hierarki SemVer.
- [ ] **6. Imutabilitas Tag:** Tag lama tidak di-overwrite (mematuhi Clause 2).
