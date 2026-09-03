---
name: pgquery-ast-safety-checklist
description: "MANDATORY AST SAFETY & TRAP DEFENSE: Strict checklist, gotchas, and safe traversal patterns for inspecting PostgreSQL AST via pg_query_go (v6). Enforces explicit BoolExpr.Boolop checks (preventing the A24 security bypass where OR/NOT was treated as conjunctive), correct ColumnDef StorageName and CONSTR_GENERATED inspection (for A32), bidirectional NullTest evaluation (IS NULL vs IS NOT NULL), and standardized getters for WhereClause, SortClause, and LockingClause. Auto-triggers whenever writing, reviewing, or debugging rules traversing pg_query AST (A24, A32, A34, A35, etc.)."
compatibility: "Go 1.25+, PostgreSQL 18.x, github.com/pganalyze/pg_query_go/v6"
metadata:
  version: "1.0.0"
  author: "Will (https://github.com/will2469)"
  license: "MIT"
  citations:
    - "github.com/pganalyze/pg_query_go/v6 (libpg_query PostgreSQL 17/18 AST)"
    - "PostgreSQL Source: src/backend/parser/gram.y & src/include/nodes/parsenodes.h"
    - "ArXiv 2607.10411: Evidence-Guided Debiasing in Static Analysis AST Traversal"
    - "CWE-284: Improper Access Control & OWASP API1:2023 Broken Object Level Authorization"
---

# PostgreSQL AST Safety & Inspection Checklist (`pgquery-ast-safety-checklist`)

> **Core Mandate:** Membaca AST `pg_query_go` bukan sekadar penelusuran pohon rekursif acak. Kegagalan membedakan semantik operator (AND vs OR), salah membaca field struct Protobuf, atau mengabaikan enum arah pengujian akan melahirkan **bug keamanan kritis** (seperti bypass isolasi tenant pada A24) atau **analisis mati (dead code)**.

---

## 1. The 4 Critical AST Traps & Audit Checklist

Sebelum menulis atau menyetujui kode AST walker, verifikasi 4 jebakan terbukti berikut:

###  Trap 1: The BoolExpr Disjunctive Trap (Pelajaran A24)
- [ ] **Wajib Periksa `BoolExpr.Boolop`:**
  - `BoolExprType_AND_EXPR` (1): **HANYA di sini** argumen anak boleh dianggap sebagai predikat konjungtif wajib!
  - `BoolExprType_OR_EXPR` (2): **DILARANG** menganggap anak di bawah `OR` sebagai filter isolasi. Kueri `WHERE status = 'active' OR tenant_id = '123'` membocorkan data seluruh tenant!
  - `BoolExprType_NOT_EXPR` (3): Kondisi dibalik secara logika.
-  **Solusi:** Gunakan [Safe Predicate Extractor](assets/safe_predicate_extractor.go) dan pelajari [Post-Mortem A24](references/boolexpr_conjunctive_trap.md).

### ⚙️ Trap 2: ColumnDef Storage Strategy (Pelajaran A32)
- [ ] **Gunakan `StorageName`, Bukan `Storage`:**
  Pada `ColumnDef`, field `Storage` sering kali kosong string `""`. Nama strategi disimpan pada `colDef.StorageName` dalam huruf kecil murni: `"plain"`, `"external"`, `"extended"`, atau `"main"`. Jika DDL tidak menyebut `STORAGE`, kedua field bernilai `""`.
-  **Solusi:** Gunakan [Column Storage Detector](assets/column_storage_detector.go) dan baca [Storage Modes Guide](references/column_def_storage_modes.md).

### ⚙️ Trap 3: Generated Columns Detection (Pelajaran A32/A34)
- [ ] **Periksa `ColumnDef.Constraints`, Bukan `colDef.Generated`:**
  Pada raw AST, `colDef.Generated` sering kali kosong. Definisi generated column berada di dalam `colDef.Constraints` sebagai `Constraint` dengan `Contype == pg_query.ConstrType_CONSTR_GENERATED` (5).
- [ ] **STORED vs VIRTUAL:**
  Dalam PostgreSQL standar ($\le 18$), seluruh `CONSTR_GENERATED` berstatus `STORED` (`VIRTUAL` memicu syntax error di parser).

### ⚖️ Trap 4: NullTest Direction (IS NULL vs IS NOT NULL)
- [ ] **Gunakan Enum Eksplisit `Nulltesttype`:**
  - `pg_query.NullTestType_IS_NULL` (1) menandakan `col IS NULL`.
  - `pg_query.NullTestType_IS_NOT_NULL` (2) menandakan `col IS NOT NULL`.
  - Jangan pernah berasumsi nilai default `0` atau melakukan string matching pada query mentah.
-  **Solusi:** Gunakan [NullTest Evaluator](assets/null_test_evaluator.go).

---

## 2. Quick Directory of Reusable Assets & References

Struktur modular skill ini menyediakan helper siap pakai:

```text
.agents/skills/pgquery-ast-safety-checklist/
├── SKILL.md                          # Checklist ringkas & pencegahan jebakan (berkas ini)
├── assets/                           # Kode helper siap pakai & teruji
│   ├── safe_predicate_extractor.go   # Walker konjungtif aman (memperbaiki bug A24)
│   ├── column_storage_detector.go    # Detektor mode TOAST & generated column (A32)
│   ├── null_test_evaluator.go        # Pembeda aman IS NULL vs IS NOT NULL
│   ├── clause_getter_helpers.go      # Getter WhereClause, SortClause, LockingClause
│   └── fragment_ast_normalizer.go    # Parser normalisasi kueri fragment (WHERE/AND)
└── references/                       # Manual teknis mendalam
    ├── boolexpr_conjunctive_trap.md  # Analisis matematis & post-mortem bug A24
    ├── column_def_storage_modes.md   # Bedah field ColumnDef TOAST & Generated
    └── ast_getter_cheatsheet.md      # Kamus getter lengkap protobuf pg_query_go
```

---

## 3. Fast Reference: Clause Getters

| Sasaran | Path Node AST | Tipe / Helper |
| :--- | :--- | :--- |
| **WHERE Clause** | `stmt.WhereClause` pada `SelectStmt`, `UpdateStmt`, `DeleteStmt`, `IndexStmt` | `*pg_query.Node` ([Helper](assets/clause_getter_helpers.go)) |
| **ORDER BY** | `sel.SortClause` -> `item.GetSortBy()` | `SortbyDir` (ASC/DESC), `SortbyNulls` |
| **Row Locking** | `sel.LockingClause` -> `item.GetLockingClause()` | `Strength` (FOR UPDATE), `WaitPolicy` (NOWAIT/SKIP LOCKED) |
| **Target Projections** | `sel.TargetList` -> `item.GetResTarget()` | `resTarget.Val` (`ColumnRef`, `A_Star`, `FuncCall`) |

---

## 4. Verification Protocol for AST Logic

Saat menguji fungsi penelusur AST baru:
1. Uji varian `AND` dan `OR` secara terpisah di unit test.
2. Uji query dengan CTE (`WITH ...`) dan subquery (`WHERE EXISTS (...)`).
3. Verifikasi pointer `nil` pada setiap penelusuran (`node == nil`, `stmt == nil`, `colDef == nil`).
