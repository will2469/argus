---
name: argus-golden-corpus
description: "MANDATORY 1-SSOT ADVERSARIAL HARNESS & RESILIENCE CORPUS: Comprehensive testing framework and Single Source of Truth (SSOT) for Argus rules (Go AST correctness & SQL migrations). Enforces the 17-pattern matrix: Positive (P1-P5 obvious, indirect, helper, nested, alias violations), Negative (N1-N5 obvious safe, legitimate idioms, unrelated APIs, sanitized inputs, static constants), and Adversarial (A1-A7 branching, reassignment, aliasing, wrappers, nested closures, generics, interfaces). Codifies the canonical 1-SSOT architecture under tests/, wires analysistest.Run to module paths, and tracks rule-by-rule adoption progress via TestGoldenCorpus_AdoptionMatrix ('golden corpus', 'test corpus', 'adversarial corpus', 'P1-P5', 'N1-N5', 'A1-A7', 'ssot test', 'adoption matrix')."
compatibility: "Go 1.25+, golang.org/x/tools/go/analysis, pg_query_go/v6"
metadata:
  version: "2.0.0"
  author: "Will (https://github.com/will2469)"
  license: "MIT"
  citations:
    - "ArXiv 2607.10411: Mitigating LLM Sycophancy in Code Smell Detection Using Evidence-Guided Debiasing"
    - "ArXiv 2606.03437: Large Language Models Are Overconfident in Their Own Responses"
    - "Go Analysis Driver Design: golang.org/x/tools/go/analysis"
    - "PostgreSQL 18.x Internals: MVCC, Lock Hierarchy, Query Tree AST"
---

# Argus Golden Adversarial Corpus Standard (`argus-golden-corpus`)

> **Core Thesis:** Static analyzers that only test *"does obvious bad code trigger?"* suffer from a dangerous false sense of security. True analyzer integrity requires answering two harder questions:
> 1. **"Can obviously safe code survive?"** (Defending against false positives that destroy developer trust).
> 2. **"Can subtly bad code evade?"** (Defending against real-world obfuscations, wrappers, branching, and data-flow breaks).
>
> **The 1-SSOT Mandate:** Seluruh aturan Argus (A01–A30) berpusat secara tunggal pada arsitektur **1-SSOT Golden Corpus** dengan `tests/correctness/<rule>/` (untuk aturan Go AST) atau `tests/migration/<rule>/` (untuk aturan SQL Migration) sebagai **Satu-Satunya Sumber Kebenaran (Single Source of Truth)**. Driver resmi Go (`analysistest.Run`) dan Standalone Runner keduanya mengevaluasi sumber kanonikal yang sama.

---

## 1. The 17-Pattern Corpus Taxonomy

Setiap aturan yang mengadopsi Golden Corpus wajib mengimplementasikan matriks kanonikal 17-pola:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    ARGUS 17-PATTERN CORPUS TAXONOMY                         │
├─────────────────────────────────────────────────────────────────────────────┤
│  POSITIVE (Violations - MUST Trigger 100% via // want)                      │
│  • P1: Obvious Violation     — Direct raw call or straightforward bad AST   │
│  • P2: Indirect Violation    — Multi-step assignment or intermediate var    │
│  • P3: Helper Violation      — Hidden inside helper function / private sub  │
│  • P4: Nested Violation      — Nested control flow, deep block, or CTE      │
│  • P5: Alias Violation       — Aliased package, receiver, or table alias    │
├─────────────────────────────────────────────────────────────────────────────┤
│  NEGATIVE (Safe - MUST SURVIVE with 0 False Positives)                      │
│  • N1: Obvious Safe          — Standard compliant parameterized usage       │
│  • N2: Legitimate Idiom      — Standard compiler idioms (e.g. COUNT(*))     │
│  • N3: Unrelated API         — Non-DB receiver with colliding method names  │
│  • N4: Sanitized Input       — Verified sanitizer or allowlist validation   │
│  • N5: Static/Constant Input — Compile-time const, typed enum, literal math │
├─────────────────────────────────────────────────────────────────────────────┤
│  ADVERSARIAL (Edge Cases & Subtle Evaders)                                  │
│  • A1: Branch                — Conditional if/else, switch/case             │
│  • A2: Reassignment          — Taint overwritten, clean reassigned to dirty │
│  • A3: Alias                 — Type aliasing, struct embedding, ptr alias   │
│  • A4: Wrapper               — Custom DB repository wrapper / middleware    │
│  • A5: Nested Function       — Closures, anonymous funcs, defers, routines  │
│  • A6: Generic               — Type parameters [T any], generic repositories│
│  • A7: Interface             — Dynamic dispatch, type assertions, any.(DB)  │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Directory Hierarchy & 1-SSOT Architecture

Semua fixture pengujian berpusat secara eksklusif di direktori `tests/`. Standar layout dibuat **100% simetris** antara aturan Go AST dan SQL Migration:

```text
tests/
├── golden_corpus_status_test.go     # Automated Adoption Matrix Checker (30 Rules - 100% ADOPTED)
├── correctness/                     # 24 Go AST rules (A01-A10, A12, A14, A16-A26 - ALL ADOPTED)
│   ├── a01/                         # ARGUS-A01: Unsafe SQL Concat
│   │   ├── positive/                # P1 - P5 (annotated with // want)
│   │   │   └── positive.go
│   │   ├── negative/                # N1 - N5 (0 diagnostics expected)
│   │   │   └── negative.go
│   │   ├── adversarial/             # A1 - A7 (stress-testing & evasion matrix)
│   │   │   └── adversarial.go
│   │   └── a01_corpus_test.go       # Automated resilience & dual-path harness
│   └── a02/ s/d a26/                # Semuanya 100% teradopsi & teruji
│
└── migration/                       # 6 SQL Migration rules (A11, A13, A15, A27-A30 - ALL ADOPTED)
    ├── a11/                         # ARGUS-A11: Destructive Migrations
    │   ├── positive/                # positive.go (// want) + migrations/*.up.sql
    │   │   ├── positive.go
    │   │   └── migrations/          # Violating migrations (DROP TABLE, DROP COLUMN, etc.)
    │   ├── negative/                # negative.go (0 want) + migrations/*.up.sql
    │   │   ├── negative.go
    │   │   └── migrations/          # Safe migrations, -- argus:ignore
    │   ├── adversarial/             # adversarial.go + migrations/*.up.sql
    │   │   ├── adversarial.go
    │   │   └── migrations/          # SQL AST evasion (multistmt, casing, quotes, schema)
    │   └── a11_corpus_test.go       # SQL parser resilience & standalone runner parity
    └── a13, a15, a27, a28, a29, a30 # Semuanya 100% teradopsi & teruji
```

---

## 3. Wiring `analysistest.Run` ke 1-SSOT Module Root

Untuk menegakkan Single Source of Truth (1-SSOT), `rules/aXX/analyzer_test.go` diarahkan langsung ke `tests/correctness/aXX/` atau `tests/migration/aXX/` menggunakan module mode bawaan Go:

### Untuk Aturan Go AST (`tests/correctness/`):
```go
func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, rootDir, Analyzer,
		"./tests/correctness/aXX/positive",
		"./tests/correctness/aXX/negative",
	)
}
```

### Untuk Aturan SQL Migration (`tests/migration/`):
```go
func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, rootDir, Analyzer,
		"./tests/migration/aXX/positive",
		"./tests/migration/aXX/negative",
	)
}
```

Setiap kali baris pada `positive/positive.go` melanggar aturan, tambahkan anotasi kanonikal `// want`:
- Pada kode Go: `db.Query(ctx, "SELECT * FROM users WHERE id = "+id) // want \`\[ARGUS-A01\] unsafe SQL concatenation\``
- Pada package migrasi: `package positive // want \`\[ARGUS-A11\] 001_destructive.up.sql: Destructive operation ...\``

Dengan cara ini, `analysistest.Run` bertindak sebagai verifikator driver Go resmi, sementara `<rule>_corpus_test.go` bertindak sebagai verifikator ketahanan mendalam (adversarial matrix dan standalone runner parity).

---

## 4. Migration Rules Adversarial Matrix (M1–M7)

Untuk aturan SQL Migration, folder `adversarial/migrations/` menguji ketahanan parser AST `pg_query_go` terhadap vektor penghindaran sintaks SQL:

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│               MIGRATION ADVERSARIAL MATRIX (M1 - M7)                        │
├─────────────────────────────────────────────────────────────────────────────┤
│  • M1: Multi-statement Chaining  — Safe DDL followed by hidden destructive   │
│  • M2: Case Insensitivity        — Mixed case syntax (e.g. dRoP cOlUmN)     │
│  • M3: Quoted Identifiers        — Escaped quotes (e.g. DROP TABLE "users") │
│  • M4: Schema Qualification      — Explicit schemas (e.g. DROP TABLE pub.u) │
│  • M5: Interleaved Comments      — Comments inside DDL (ALTER /* c */ TABLE)│
│  • M6: Procedural Block Wrapping — Nested in DO $$ BEGIN ... END $$;        │
│  • M7: Batch Multi-Target        — Comma-separated drops (DROP TABLE a, b)  │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 5. Authoring New Rules Protocol (ARGUS-A31+)

Saat menambahkan aturan baru ke Argus Checker:

1. **Step 1: Scaffolding Corpus Folders (Direct 1-SSOT)**
   - Seluruh fixture baru langsung ditempatkan di direktori 1-SSOT `tests/`.
   - Untuk Go AST: buat `tests/correctness/<rule>/` dengan subfolder `positive/`, `negative/`, `adversarial/`.
   - Untuk Migrasi SQL: buat `tests/migration/<rule>/` dengan subfolder `positive/migrations/`, `negative/migrations/`, `adversarial/migrations/`.
2. **Step 2: Implementasi Pola Matrix**
   - `positive/`: Implementasikan kasus pelanggaran dengan anotasi `// want`.
   - `negative/`: Implementasikan kasus patuh, valid, serta verifikasi directive (`argus:ignore`).
   - `adversarial/`: Implementasikan vektor stress-test (A1–A7 untuk Go AST, M1–M7 untuk Migration SQL).
3. **Step 3: Test Harness & Paritas Runner**
   - Buat `<rule>_corpus_test.go` yang menguji `InspectFile`/`ScanMigrationDir`, direct assertions, dan `runner.RunAuditWithConfig`.
4. **Step 4: Wiring SSOT Driver**
   - Tulis `rules/<rule>/analyzer_test.go` agar mengarah ke `./tests/<category>/<rule>/positive` dan `./tests/<category>/<rule>/negative`.
5. **Step 5: Verifikasi Status Adopsi**
   - Daftarkan rule pada `tests/golden_corpus_status_test.go`.
   - Jalankan `go test -v -run TestGoldenCorpus_AdoptionMatrix ./tests`.
   - Pastikan status rule terverifikasi `ADOPTED`.
6. **Step 6: Zero Regression Quality Gate**
   - Jalankan `make lint && make test-full` (100% Hijau, 0 race condition).

---

## 6. Automated Adoption Checker

Untuk memantau integritas 1-SSOT seluruh 30 aturan Argus kapan saja:

```bash
go test -v -run TestGoldenCorpus_AdoptionMatrix ./tests
```

Status saat ini telah **100% ADOPTED**:
```text
=== RUN   TestGoldenCorpus_AdoptionMatrix
=========================================================================================================
RULE CODE    | CATEGORY     | STATUS     | SSOT PATH                        | DETAILS
---------------------------------------------------------------------------------------------------------
ARGUS-A01    | Correctness  | ADOPTED    | tests/correctness/a01            | 1 SSOT Golden Corpus (P1-P5, N1-N5, A1-A7, analysistest + runner)
ARGUS-A02    | Correctness  | ADOPTED    | tests/correctness/a02            | 1 SSOT Golden Corpus (P1-P5, N1-N5, A1-A7, analysistest + runner)
...
ARGUS-A29    | Migration    | ADOPTED    | tests/migration/a29              | 1 SSOT Golden Migration Corpus (positive, negative, adversarial + runner)
ARGUS-A30    | Migration    | ADOPTED    | tests/migration/a30              | 1 SSOT Golden Migration Corpus (positive, negative, adversarial + runner)
=========================================================================================================
Golden Corpus Adoption Progress: 30 / 30 rules adopted (100.0%)
```
