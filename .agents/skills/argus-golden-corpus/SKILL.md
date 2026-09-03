---
name: argus-golden-corpus
description: "MANDATORY 1-SSOT ADVERSARIAL HARNESS & RESILIENCE CORPUS: Comprehensive testing framework and Single Source of Truth (SSOT) for Argus rules (Go AST correctness & SQL migrations). Enforces the 17-pattern matrix: Positive (P1-P5 obvious, indirect, helper, nested, alias violations), Negative (N1-N5 obvious safe, legitimate idioms, unrelated APIs, sanitized inputs, static constants), and Adversarial (A1-A7 branching, reassignment, aliasing, wrappers, nested closures, generics, interfaces). Codifies the 1-SSOT rule (eliminating duplicate legacy testdata/ in favor of tests/), wires analysistest.Run to module paths, and tracks rule-by-rule adoption progress via TestGoldenCorpus_AdoptionMatrix ('golden corpus', 'test corpus', 'adversarial corpus', 'P1-P5', 'N1-N5', 'A1-A7', 'ssot test', 'adoption matrix')."
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
> **The 1-SSOT Mandate:** Dilarang keras menduplikasi test fixture antara `testdata/` legacy dan `tests/`. Setiap rule yang mengadopsi Golden Corpus menjadikan `tests/correctness/<rule>/` atau `tests/migration/<rule>/` sebagai **Satu-Satunya Sumber Kebenaran (Single Source of Truth)**. Driver resmi Go (`analysistest.Run`) dan Standalone Runner keduanya mengevaluasi sumber yang sama.

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

Semua fixture pengujian berpusat di direktori `tests/`. Folder legacy `testdata/src/<rule>` **dihapus tuntas** begitu rule mengadopsi Golden Corpus:

```text
tests/
├── golden_corpus_status_test.go     # Automated Adoption Matrix Checker (30 Rules)
├── correctness/                     # Go AST rules
│   ├── a01/                         # ARGUS-A01: Unsafe SQL Concat (ADOPTED)
│   │   ├── positive/                # P1 - P5 (annotated with // want)
│   │   │   └── positive.go
│   │   ├── negative/                # N1 - N5 (0 diagnostics expected)
│   │   │   └── negative.go
│   │   ├── adversarial/             # A1 - A7 (stress-testing & evasion matrix)
│   │   │   └── adversarial.go
│   │   └── a01_corpus_test.go       # Automated resilience & dual-path harness
│   ├── a17/                         # ARGUS-A17: N+1 in Loops (Pending)
│   ├── a24/                         # ARGUS-A24: Tenant Isolation Leak (Pending)
│   └── a26/                         # ARGUS-A26: LIKE Wildcard Injection (Pending)
│
└── migration/                       # SQL Migration rules
    ├── a11/                         # ARGUS-A11: Destructive Migrations
    ├── a13/                         # ARGUS-A13: Missing Down Migrations
    ├── a27/                         # ARGUS-A27: Non-Concurrent Indexes
    ├── a28/                         # ARGUS-A28: Table Locking Constraints
    ├── a29/                         # ARGUS-A29: Unindexed Foreign Keys
    └── a30/                         # ARGUS-A30: Timestamps Without Timezone
```

---

## 3. Wiring `analysistest.Run` ke 1-SSOT Module Root

Untuk menghindari duplikasi fixture antara `testdata/` dan `tests/`, `rules/aXX/analyzer_test.go` diarahkan langsung ke `tests/correctness/aXX/` menggunakan module mode bawaan Go:

```go
func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	// Menguji positive (// want) dan negative (0 issues) langsung dari SSOT
	analysistest.Run(t, rootDir, Analyzer,
		"./tests/correctness/aXX/positive",
		"./tests/correctness/aXX/negative",
	)
}
```

Setiap kali baris pada `positive/positive.go` melanggar aturan, tambahkan anotasi kanonikal `// want`:
```go
db.Query(ctx, "SELECT * FROM users WHERE id = "+id) // want `\[ARGUS-A01\] unsafe SQL concatenation`
```
Dengan cara ini, `analysistest.Run` bertindak sebagai verifikator driver Go resmi, sementara `aXX_corpus_test.go` bertindak sebagai verifikator ketahanan mendalam (adversarial matrix dan standalone runner parity).

---

## 4. Rule-by-Rule Iteration Protocol (Alur Migrasi Per Rule)

Saat memigrasikan rule dari legacy unit-test ke Golden Corpus:

1. **Step 1: Scaffolding Corpus Folders**
   - Buat direktori `tests/correctness/<rule>/` dengan subfolder `positive/`, `negative/`, `adversarial/`.
2. **Step 2: Implementasi 17 Pola**
   - `positive/positive.go`: Implementasikan P1–P5 lengkap dengan anotasi `// want` dan kasus `argus:ignore`.
   - `negative/negative.go`: Implementasikan N1–N5 yang wajib bersih dari diagnostik.
   - `adversarial/adversarial.go`: Implementasikan A1–A7 untuk menguji batas ketahanan parser AST/taint.
3. **Step 3: Test Harness & Paritas Runner**
   - Buat `<rule>_corpus_test.go` yang menguji `InspectFile`, direct assertions, dan `runner.RunAuditWithConfig`.
4. **Step 4: Wiring SSOT & Hapus Legacy**
   - Perbarui `rules/<rule>/analyzer_test.go` agar mengarah ke `./tests/correctness/<rule>/...`.
   - Hapus berkas dan folder lama di `testdata/src/<rule>/`.
5. **Step 5: Verifikasi Status Adopsi**
   - Jalankan `go test -v -run TestGoldenCorpus_AdoptionMatrix ./tests`.
   - Pastikan status rule berubah dari `PENDING` menjadi `ADOPTED`.
6. **Step 6: Zero Regression Quality Gate**
   - Jalankan `make lint` dan `make test` (100% Hijau).

---

## 5. Automated Adoption Checker

Untuk memantau progress adopsi seluruh 30 aturan Argus kapan saja:

```bash
go test -v -run TestGoldenCorpus_AdoptionMatrix ./tests
```

Perintah ini akan mencetak tabel status real-time:
```text
=== RUN   TestGoldenCorpus_AdoptionMatrix
=========================================================================================================
RULE CODE    | CATEGORY     | STATUS     | SSOT PATH                        | DETAILS
---------------------------------------------------------------------------------------------------------
ARGUS-A01    | Correctness  | ADOPTED    | tests/correctness/a01            | 1 SSOT Golden Corpus (P1-P5, N1-N5, A1-A7)
ARGUS-A02    | Correctness  | PENDING    | testdata/src/a02                 | Legacy unit-level testdata fixture
...
=========================================================================================================
Golden Corpus Adoption Progress: 1 / 30 rules adopted (3.3%)
```
