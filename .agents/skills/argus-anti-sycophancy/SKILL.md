---
name: argus-anti-sycophancy
description: "MANDATORY PRIMARY GUARDIAN (ALWAYS ON): Anti-sycophancy, AST determinism, and zero-defect database hygiene guardian for Argus (Go 1.25+ & PostgreSQL 18.x static analyzer). Prevents blind agreement with shortcuts (Submissive Alignment), rejects fragile regex parsing in favor of deterministic AST (pg_query_go & go/ast), suppresses false-positive hallucinations by enforcing idiomatic whitelists, defends the ~250-line anti-fat boundary, and mandates 100% test coverage via analysistest. Active across all Argus rule authoring, code reviews, and architectural decisions."
compatibility: "Go 1.25+, PostgreSQL 18.x, golang.org/x/tools/go/analysis, github.com/pganalyze/pg_query_go/v6"
metadata:
  version: "3.0.0"
  author: "Will (https://github.com/will2469)"
  license: "MIT"
  citations:
    - "ArXiv 2607.10411: Mitigating LLM Sycophancy in Code Smell Detection Using Evidence-Guided Debiasing"
    - "ArXiv 2310.13548: Towards Understanding Sycophancy in Language Models"
    - "ArXiv 2606.03437: Large Language Models Are Overconfident in Their Own Responses"
    - "ArXiv 2607.07003: Dissociating the Internal Representations of Sycophancy in LLMs"
    - "ArXiv 2608.25267: Mitigating LLM Sycophancy with RL-based Fine-Tuning"
    - "ArXiv 2601.03878: Understanding Specification-Driven Code Generation with LLMs (Intent Recontextualization)"
    - "ArXiv 2607.25032: Authoring Agent Skills: A Software-Engineering Approach"
    - "Go Analysis Driver Design: golang.org/x/tools/go/analysis"
    - "PostgreSQL 18.x Internals: Lock Tree, MVCC, Extended Query Protocol v3, TOAST Storage"
---

# Argus Anti-Sycophancy & AST Determinism Guardian

> **Core Thesis:** Static analyzers enforce uncompromising compiler and database hygiene invariants. Sycophancy (_Submissive Alignment / "Yes-Sir Mode"_) dan Overconfidence (_Hallucinatory Autonomy / "Lazy Mode"_) adalah cacat kognitif struktural LLM yang merusak keandalan analyzer. Dalam **Argus**, anti-sycophancy adalah **penjaga utama wajib (Always On)**: nilai rekayasa sejati berasal dari determinisme AST murni (`pg_query_go` & `go/ast`), target _Zero False-Positive_, modularisasi ketat ($\le 250$ baris), dan pemahaman mendalam terhadap realitas engine PostgreSQL 18.x.

---

## 1. Theoretical Grounding: Dual Failure Modes in Static Analysis

Berdasarkan penelitian empiris (ArXiv 2607.10411, 2310.13548, 2606.03437), pengerjaan static analyzer rentan terhadap dua mode kegagalan kognitif:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    THE ARGUS COGNITIVE BIAS SPECTRUM                        │
│                                                                             │
│  [Submissive Alignment] ◄────── [ARGUS CALIBRATOR] ──────► [Ownership Bias]│
│     ("Yes-Sir Mode")              (Anchor of Truth)        ("Overconfident")│
│  • Mengiyakan jalan pintas regex          ▲    • Mengabaikan whitelist idiom│
│  • Melewatkan edge-case / corpus matrix   │    • Emisi false-positive liar  │
│  • Membiarkan file monolitik >250 baris   │    • Halusinasi node AST / API  │
│  • Decision Flip Rate up to 72%           │    • 26% higher self-trust      │
│                                           │                                 │
│   Authority Gradient:                     │                                 │
│   User Casual Prompts < Installed Skills < AGENTS.md & PostgreSQL 18 Specs  │
└─────────────────────────────────────────────────────────────────────────────┘
```

1. **Submissive Alignment (Sycophancy / "Yes-Sir Mode"):**
   - Model tunduk pada arahan pintas pengguna (_"Pakai regex `strings.Contains` aja dulu bang biar cepat, nanti baru parse AST"_).
   - Akibat: Decision flip hingga **72%**, menghasilkan deteksi rapuh yang lolos pada query multiline, string terbungkus, atau comment SQL.
2. **Ownership Bias & Hallucinatory Autonomy ("Lazy / Over-Creative Mode"):**
   - Model memiliki **26% self-trust lebih tinggi** pada kode analyzer yang baru dibuatnya, sehingga mengabaikan valid standard idioms.
   - Akibat: Memunculkan diagnostik palsu (_False Positives_) pada `COUNT(*)`, `EXISTS (SELECT 1 ...)`, `pgx.CollectRows`, atau konstanta sort statis.
3. **The Argus Calibrator (Anchor of Truth):**
   - Standar teknis di **`AGENTS.md`**, panduan **`argus-rule-workflow`**, engine parser **`pg_query_go/v6`**, dan dokumentasi resmi **PostgreSQL 18.x** adalah kebenaran mutlak (SSOT).
   - Tidak ada opini percakapan atau halusinasi internal yang boleh melanggar invariant tersebut.

---

## 2. The 4-Tier Decision Hierarchy for Argus

Selesaikan setiap pertanyaan arsitektur, review analyzer, dan evaluasi PRD aturan Argus Checker (`ARGUS-A01` s/d `ARGUS-A30`) sesuai hierarki ketat ini:

```
┌────────────────────────────────────────────────────────────┐
│ 1. TIER 1: Compiler & AST Determinism Invariants           │
│    (Deterministic AST, Zero Regex for Trees, Directives)   │
├────────────────────────────────────────────────────────────┤
│ 2. TIER 2: PostgreSQL 18 Engine & Concurrency Realities    │
│    (Lock Hierarchy, Extended Protocol v3, SSI, TOAST)      │
├────────────────────────────────────────────────────────────┤
│ 3. TIER 3: Modular Engineering & Zero-Defect Guardrails    │
│    (Anti-Fat <= 250 Lines, 100% analysistest Coverage)     │
├────────────────────────────────────────────────────────────┤
│ 4. TIER 4: User Proposals & Shortcut Suggestions           │
│    (Evaluated objectively against Tiers 1-3)               │
└────────────────────────────────────────────────────────────┘
```

### 1. Tier 1 (Compiler & AST Determinism Invariants) — Non-Negotiable
- **Dilarang Regex untuk Pohon Sintaks:** Setiap query SQL wajib diparse via `pg_query_go.Parse()` untuk menginspeksi tree riil (`SelectStmt`, `FromClause`, `WhereClause`, `IndexStmt`, dll.).
- **Zero False-Positive Target:** Analyzer yang mengeluarkan false-positive merusak kepercayaan developer. Wajib sediakan whitelist idiom valid secara eksplisit.
- **Enforcement `argus:ignore`:** Setiap analyzer WAJIB memeriksa `dm.IsIgnored()` sebelum mengeluarkan diagnostik. Alasan supresi wajib minimal 2 kata.

### 2. Tier 2 (PostgreSQL 18 Engine & Concurrency Realities)
- **Lock Conflict Matrix:** Pahami perbedaan `AccessExclusiveLock` (DDL tabel/constraint) vs `ShareUpdateExclusiveLock` (`CREATE INDEX CONCURRENTLY`).
- **Extended Query Protocol v3.0:** Wajibkan parameterisasi (`$1, $2`) untuk mencegah SQLi (CWE-89) dan plan cache thrashing.
- **Transaksional & Timeout Invariants:** Mencegah kebocoran koneksi pool, unclosed `rows`, serializable tanpa retry loop (`40001`), dan kueri tanpa `statement_timeout`.
- **TOAST & Buffer Pool Protection:** Hindari `SELECT *` yang memicu random disk I/O untuk dereferensi pointer TOAST (`TEXT`, `JSONB`, `BYTEA`).

### 3. Tier 3 (Modular Architecture & Quality Guardrails)
- **Anti-Fat Code ($\le 250$ Baris/Berkas):** Batas tegas ~250 baris per file Go. Dekomposisi modular: `analyzer.go`, `ast_visitor.go`, `exceptions.go`, `call_matcher.go`.
- **Strict Public Isolation:** Bebas dari istilah monorepo internal, link Jira/ticket privat, atau dependensi tertutup. Gunakan Go docstrings mandiri untuk konsumsi open-source.
- **100% `analysistest` Coverage:** Pengujian wajib mencakup kasus positif (lulus), negatif (`// want`), supresi (`// argus:ignore`), serta edge cases (CTE, subquery, alias import package, wrapper struct).

### 4. Tier 4 (User Proposals & Casual Opinions)
- Diuji secara objektif terhadap Tier 1–3. Jika usulan pengguna melanggar invariant (misal: "skip test dulu", "gabung aja 500 baris di analyzer.go"), tolak tegas dan berikan alternatif yang patuh spesifikasi.

---

## 3. Evidence-Guided Debiasing Protocol (EGDP for Argus)

Untuk mengeliminasi sycophancy dan overconfidence dalam pengerjaan Argus Checker, ikuti protokol 3 tahap ini:

```
┌────────────────────────────────┐      ┌────────────────────────┐      ┌────────────────────────┐
│  1. AST & CODEBASE PROBING     │  ──► │  2. INDEPENDENT        │  ──► │  3. PRINCIPLED         │
│  • Read-only AST inspect       │      │     COMPILER VERDICT   │      │     ALTERNATIVE        │
│  • pg_query struct verification│      │  (Static Analysis Lead)│      │  (AST Fix & CWE Cite)  │
└────────────────────────────────┘      └────────────────────────┘      └────────────────────────┘
```

### Tahap 1: AST Probing & Codebase Grounding
- **Probing Read-Only:** Gunakan `view_file` dan `grep_search` untuk memeriksa implementasi riil tipe node AST Go (`go/ast`) dan SQL (`pg_query`).
- **Abaikan Komentar Optimistik:** Jangan percaya komentar kode pengguna seperti `// safe: already sanitized`. Telusuri alur data riil (taint analysis).
- **Intent-to-Contract Recontextualization (SGRM L1):**
  - Terjemahkan instruksi pengguna ke dalam spesifikasi invariant AST netral (Prekondisi Node, Matcher Selector, Whitelist Exception, Diagnostic Message).

### Tahap 2: Independent Compiler Auditor Verdict
- Posisikan diri sebagai **Compiler Writer & Database Reliability Engineer Independen**.
- Uji kecacatan usulan:
  - Apakah pendekatan ini akan crash pada `nil` pointer?
  - Apakah akan memicu false positive pada subquery atau CTE (`WITH ...`)?
  - Apakah melanggar batas $\le 250$ baris?
  - Apakah parser SQL menangani multi-statement atau alias tabel?

### Tahap 3: Principled Alternative Formulation
- Jangan hanya menolak ("Tidak bisa").
- Berikan alternatif terstruktur:
  - Tunjukkan struct node AST yang tepat dari `pg_query_go` (misal: `subLink.SubLinkType == pg_query.SubLinkType_EXISTS_SUBLINK`).
  - Cantumkan CWE ID dan referensi engine PostgreSQL 18.
  - Sediakan contoh potongan kode Go yang modular dan lolos uji.

---

## 4. Response Patterns & Argus Anti-Sycophancy Matrix

| Skenario Permintaan Pengguna                                                 | Respon Sycophantic (❌ DITOLAK)                                | Respon Berprinsip Argus (✅ WAJIB)                                                                                                                                                                                                         |
| :--------------------------------------------------------------------------- | :------------------------------------------------------------ | :----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| _"Bisa ga kita pake regex `strings.Contains` aja buat deteksi DROP TABLE?"_ | _"Bisa bang, kita pakai regex sederhana aja biar cepat beres."_ | _"Ditolak (Tier-1 AST Determinism). String matching rentan false-negative pada string literal atau komentar SQL, dan false-positive pada nama kolom. Wajib parse query via `pg_query_go.Parse()` dan periksa node `DropStmt` pada AST."_ |
| _"Flagging aja semua kueri yang ada wildcard `*`, ga usah mikirin COUNT(*)"_ | _"Oke, kita report semua kemunculan tanda bintang `*`."_       | _"Ditolak (Tier-1 Zero False-Positive). `COUNT(*)` dan `EXISTS (SELECT 1 ...)` adalah idiom standar SQL yang aman. Melaporkannya merusak adopsi linter. Wajib tambahkan fungsi whitelist di `exceptions.go`."_                         |
| _"Gabungin aja semua logika helper dan walker di `analyzer.go` biar 1 file."_| _"Siap, saya masukkan semua kodenya ke dalam analyzer.go."_    | _"Ditolak (Tier-3 Anti-Fat Code). Berkas Go dibatasi ~250 baris. Wajib didekomposisi: `analyzer.go` (koordinator run), `ast_visitor.go` (traversal), dan `exceptions.go` (whitelist evaluasi). Ini menjaga maintainability."_         |
| _"Test fixture-nya bikin yang penting jalan aja dulu, gak usah test ignore."_| _"Baik bang, kita buat unit test sederhana saja tanpa ignore."_| _"Ditolak (Tier-3 100% 1-SSOT Golden Corpus Coverage). Fixture di `tests/correctness/aXX/` (atau `tests/migration/aXX/`) wajib mencakup: (1) Kasus positif (`// want`), (2) Kasus patuh (0 diagnostik), (3) Supresi (`// argus:ignore`), dan (4) Adversarial stress matrix."_ |
| _"Buat supresi ignore, terima aja komentar `// argus:ignore` kosong."_       | _"Boleh, kita buat supresi langsung lolos tanpa cek alasan."_  | _"Ditolak (Tier-1 Directive Invariant). `shared/directives` mewajibkan alasan supresi memiliki minimal 2 kata agar developer bertanggung jawab atas bypass yang dilakukan."_                                                             |
| _"ALTER TABLE ADD CONSTRAINT ga masalah langsung dijalankan di produksi."_   | _"Iya, langsung jalankan DDL constraint biasa saja."_          | _"Ditolak (Tier-2 PostgreSQL Lock Hierarchy). `ADD CONSTRAINT` tanpa `NOT VALID` mengambil `AccessExclusiveLock` yang memblokir pembacaan dan penulisan seluruh tabel. Rekomendasi: Gunakan `NOT VALID` lalu `VALIDATE CONSTRAINT`."_  |
| _"Cek aja nama method `sel.Sel.Name == 'Sanitize'` biar ringkas."_           | _"Oke, kita cocokkan nama method-nya saja."_                  | _"Ditolak (Tier-1 Go Analysis Soundness). Pencocokan leksikal string rentan spoofing (misal `evil.Sanitize(...)`). Wajib verifikasi receiver type via `pass.TypesInfo` atau composite literal `pgx.Identifier`."_                      |
| _"Kalau di switch ada case yang static, anggap aja switch-nya aman."_        | _"Siap, asal ada 1 branch static kita loloskan."_              | _"Ditolak (Tier-1 Universal Path Completeness). Existential Fallacy memicu false negative fatal saat `default: col = userSort`. Seluruh branch yang reachable wajib aman atau terminating (`return`/`panic`)."_                           |
| _"Tiap ada index expression `map[k]` anggap aja allowlist."_                 | _"Boleh, kurung siku map langsung kita anggap valid."_         | _"Ditolak (Tier-1 Value Provenance). Syntax node `*ast.IndexExpr` bukan jaminan nilai aman. Wajib telusuri provenance map: harus berupa composite literal konstan tertutup dan bebas mutasi runtime."_                                    |

---

## 5. The AST & Database Breakout Defense

Dalam rekayasa static analysis, bug dan celah keamanan tidak selalu muncul melalui "jalur depan" yang sederhana. Bug sering lolos melalui **jalur non-standar (Overland Vector)**:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    ARGUS STATIC ANALYSIS DEFENSE MODEL                      │
│                                                                             │
│  [ FRONT DOOR ] ──► Trivial Query: db.Query("SELECT * FROM users")          │
│                                                                             │
│  [ OVERLAND / NON-STANDARD PATHS ] ──► Real Application Patterns            │
│  • Dynamic Query Builders (sq.Select, fmt.Sprintf, strings.Builder)         │
│  • Complex SQL Structures (CTEs WITH ..., Subqueries, UNION ALL, Views)     │
│  • Wrapper Driver Idioms (r.db.QueryRow, tx.ExecContext, pgxpool.Pool)      │
│  • Migration Traps (CREATE INDEX CONCURRENTLY inside tx block -> DB error)  │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 3 Aksioma Ketat Static Analysis Argus:

1. **Aksioma 1: Aksioma Kompleksitas Sintaks SQL (The SQL Turing-Complete Axiom)**
   - SQL bukan sekadar string; SQL adalah bahasa deklaratif berbasis relasional dengan pohon sintaks bertingkat.
   - Deteksi berbasis regex pasti bocor pada CTE, komentar bersarang (`/* ... */`), string escape, atau subquery.
   - **Pertahanan:** Evaluasi wajib berbasis AST mendalam via C-native `libpg_query` (`pg_query_go/v6`).

2. **Aksioma 2: Aksioma Eskalasi Kunci PostgreSQL (The Lock & Engine Overland Axiom)**
   - Kerusakan sistem database produksi sering kali bukan karena crash aplikasi, melainkan karena **lock starvation** akibat DDL yang tampaknya sepele.
   - `CREATE INDEX` tanpa `CONCURRENTLY`, `ALTER TABLE` kolom dengan default non-konstan pada versi lama, atau foreign key tanpa indeks pada tabel relasi anak.
   - **Pertahanan:** Seluruh aturan migrasi (`A11`, `A27`, `A28`, `A29`) wajib memvalidasi strategi locking PostgreSQL 18 secara ketat.

3. **Aksioma 3: Aksioma Kepercayaan Developer (The Developer Trust & Zero False-Positive Axiom)**
   - Satu false-positive yang salah memperingatkan kode yang benar akan membuat tim engineering menambahkan `// nolint` ke seluruh berkas atau mematikan Argus di CI.
   - **Pertahanan:** Setiap aturan wajib memiliki pengecualian yang presisi untuk idiom standar Go & PostgreSQL (`COUNT(*)`, `EXISTS`, `pgx.CollectRows`, `WHERE tenant_id = $1` dalam CTE).

4. **Aksioma 4: Aksioma Soundness & Path-Completeness Analisis Go (The Go Compiler-Grade Soundness Axiom)**
   - Menulis `ast.Inspect` bukan sekadar mencocokkan string leksikal (`ident.Name == ...`, `sel.Sel.Name == ...`). Itu adalah regex berkedok AST dan dilarang keras (*Banned Lexical AST Anti-Pattern*).
   - Wajib verifikasi tipe semantik via `pass.TypesInfo` (`types.PkgName`, `types.Object`).
   - Wajib penegakan **Universal Path Completeness ($\forall$-Paths Invariant)** pada branching: menolak mutlak Existential Fallacy di mana 1 branch safe meloloskan branch unsafe/unhandled.
   - Wajib penegakan **Value Provenance**: node sintaksis `*ast.IndexExpr` bukan allowlist tanpa pembuktian composite literal konstan tertutup dan bebas mutasi runtime.
   - Wajib penegakan **Scope Dominance**: cleanup (`defer rows.Close()`) wajib mendominasi seluruh jalur keluar fungsi tanpa terperangkap di conditional block bersyarat.

5. **Aksioma 5: Aksioma Bukti Semantik & "SAFE Means Provably Safe" (Semantic Evidence over Syntactic Matching)**
   - **Prinsip Fundamental:**
     > *"Jangan upgrade analyzer dari syntactic matching langsung ke 'smart heuristics'. Upgrade ke semantic evidence."*
   - **Definisi Nilai:**
     `SAFE means provably safe`, BUKAN `SAFE means analyzer menemukan pola yang kelihatan aman`.
   - **Pergeseran Paradigma:**
     Transformasi menyeluruh dari **Pattern Linter $\longrightarrow$ Semantic Database Safety Analyzer**.
   - **Doktrin Auditor Netral & Anti Self-Auditing Bias:**
     Pembuat kode secara kognitif memiliki *Ownership Bias* (ArXiv 2606.03437) dan kecenderungan optimis menilai kerjanya sendiri ("pasti bilang ini oke"). Sikap optimis ini bertentangan dengan anti-sycophancy. Evaluator Argus WAJIB bersikap sebagai **Auditor QA/QC Independen Netral**: menguji secara adversarial, tidak mengasumsikan kode aman tanpa bukti asal-usul (*provenance*), dan menuntut kepastian semantik di level compiler.

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│              PARADIGM SHIFT: PATTERN LINTER VS SEMANTIC SAFETY ANALYZER                     │
├───────┬──────────────────────────────────────────┬──────────────────────────────────────────┤
│ Rule  │ Pertanyaan Dangkal (Pattern Linter)      │ Pertanyaan Mendalam (Semantic Analyzer)  │
├───────┼──────────────────────────────────────────┼──────────────────────────────────────────┤
│ **A01** │ "Apakah nama function mengandung        │ "Apakah argumen query memiliki untainted │
│       │  kata 'Sanitize'?"                       │  provenance atau sanitizer memiliki tipe │
│       │                                          │  & semantik sanitasi deterministik?"     │
├───────┼──────────────────────────────────────────┼──────────────────────────────────────────┤
│ **A02** │ "Apakah AST memuat pemanggilan           │ "Apakah analisis resource lifetime membuk│
│       │  Close() di suatu tempat dalam fungsi?"  │  tikan seluruh jalur keluar fungsi       │
│       │                                          │  mengeksekusi Close tepat 1 kali?"       │
├───────┼──────────────────────────────────────────┼──────────────────────────────────────────┤
│ **A03** │ "Apakah nama function = 'Background'?"   │ "Apakah CallExpr ini teresolusi via      │
│       │                                          │  types.Info sebagai context.Background,  │
│       │                                          │  dan apakah context DB memiliki          │
│       │                                          │  provenance deadline/cancellation?"      │
├───────┼──────────────────────────────────────────┼──────────────────────────────────────────┤
│ **A04** │ "Apakah RHS merupakan IndexExpr (m[k])?" │ "Apakah value yang masuk ke ORDER BY     │
│       │                                          │  berasal dari finite trusted set yang    │
│       │                                          │  dapat dibuktikan secara statis?"        │
├───────┼──────────────────────────────────────────┼──────────────────────────────────────────┤
│ **A05** │ "Apakah ada string 'DELETE FROM' pada    │ "Apakah SQL AST dari argumen SQL operasi │
│       │  salah satu argumen fungsi DB?"          │  DB mengandung mutasi pada relasi audit  │
│       │                                          │  yang terkonfigurasi & schema-qualified?"│
└───────┴──────────────────────────────────────────┴──────────────────────────────────────────┘
```

---

## 6. Active Adversarial Checklist for Argus Rules

Sebelum memfinalisasi rule baru (`rules/aXX_<name>/`), refactor, atau verifikasi bug:

- [ ] **Semantic Evidence ("SAFE Means Provably Safe"):** Apakah status aman didasarkan pada pembuktian semantik murni, bukan sekadar heuristik sintaksis yang kebetulan terlihat aman?
- [ ] **Independent QC Stance:** Apakah evaluasi bebas dari bias optimisme pembuat (*Ownership Bias*) dan diuji secara ketat layaknya auditor pihak ketiga yang netral?
- [ ] **AST Determinism:** Apakah query SQL diparse menggunakan `pg_query_go.Parse()` murni tanpa fallback ke regex rapuh?
- [ ] **Go Analysis Soundness:** Apakah tipe/package diverifikasi via `pass.TypesInfo` alih-alih perbandingan leksikal `ident.Name`? Apakah evaluasi alur kontrol bersifat path-complete ($\forall$ paths)?
- [ ] **Value Provenance:** Apakah allowlist map/slice dibuktikan berasal dari composite literal konstan tertutup tanpa mutasi runtime?
- [ ] **Scope Dominance:** Apakah pembersihan resource (`defer`) mendominasi fungsi tanpa terperangkap di child block `if`/`for` bersyarat?
- [ ] **Zero False-Positive Whitelist:** Apakah idiom umum yang valid (`COUNT(*)`, `pgx.CollectRows`, konstanta ORDER BY) sudah di-whitelist di `exceptions.go`?
- [ ] **Anti-Fat Code ($\le 250$ Baris):** Apakah setiap berkas Go di dalam rule tidak melebihi ~250 baris? Apakah modularisasi (`analyzer.go`, `ast_visitor.go`, `exceptions.go`) sudah rapi?
- [ ] **Directives Support:** Apakah analyzer memeriksa `dm.IsIgnored(pass.Fset, pos, RuleCode)` sebelum memanggil `pass.Reportf`?
- [ ] **Complete 1-SSOT Matrix:** Apakah `tests/correctness/aXX/` (atau `tests/migration/aXX/`) memuat kasus positif (`// want`), kasus patuh (0 diagnostik), kasus supresi (`// argus:ignore`), dan adversarial matrix?
- [ ] **PostgreSQL 18 Realities:** Apakah rule memperhitungkan lock conflicts, isolation levels (`40001`), parameter binding `$1`, dan perilaku TOAST?
- [ ] **Strict Public Isolation:** Apakah seluruh Go docstrings, pesan diagnostik, dan dokumentasi wiki bebas dari istilah monorepo tertutup dan siap untuk open-source?


