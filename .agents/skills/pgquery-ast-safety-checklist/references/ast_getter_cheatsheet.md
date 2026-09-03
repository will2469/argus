# pg_query_go AST Getters & Protobuf Field Cheatsheet

Kamus rujukan getter dan penjelajah field AST PostgreSQL (`github.com/pganalyze/pg_query_go/v6`) untuk static analysis.

---

## 1. WhereClause Getters

Klausa `WHERE` pada `pg_query_go` disimpan sebagai `*pg_query.Node`:

| Tipe Statement  | Path Field AST                                | Keterangan                                              |
| :-------------- | :-------------------------------------------- | :------------------------------------------------------ |
| `SelectStmt`    | `stmt.WhereClause *pg_query.Node`             | Filter kueri pembacaan                                  |
| `UpdateStmt`    | `stmt.WhereClause *pg_query.Node`             | Filter update baris (wajib ada pada tabel tenant)       |
| `DeleteStmt`    | `stmt.WhereClause *pg_query.Node`             | Filter hapus baris (wajib ada pada tabel tenant)        |
| `IndexStmt`     | `stmt.WhereClause *pg_query.Node`             | Predikat indeks parsial (`CREATE INDEX ... WHERE ...`)  |

---

## 2. NullTest Getters (IS NULL vs IS NOT NULL)

Ketika SQL memuat `col IS NULL` atau `col IS NOT NULL`:
- Tipe Node: `pg_query.NullTest`
- Getter: `nullTest := node.GetNullTest()`
- `nullTest.Arg`: Ekspresi yang diuji (biasanya `ColumnRef`)
- `nullTest.Nulltesttype`:
  - `pg_query.NullTestType_IS_NULL` (nilai integer: 1)
  - `pg_query.NullTestType_IS_NOT_NULL` (nilai integer: 2)

> [!WARNING]
> Dilarang berasumsi `0` adalah `IS_NULL`. Selalu lakukan komparasi eksplisit terhadap enum `pg_query.NullTestType_IS_NULL` dan `pg_query.NullTestType_IS_NOT_NULL`.

---

## 3. SortClause Getters (ORDER BY)

Pada `SelectStmt`:
- Path: `stmt.SortClause []*pg_query.Node`
- Getter per elemen: `sortBy := item.GetSortBy()`
  - `sortBy.Node`: Ekspresi kolom pengurutan (`ColumnRef`, `A_Expr`, atau fungsi).
  - `sortBy.SortbyDir`:
    - `pg_query.SortByDir_SORTBY_DEFAULT` (0)
    - `pg_query.SortByDir_SORTBY_ASC` (1)
    - `pg_query.SortByDir_SORTBY_DESC` (2)
  - `sortBy.SortbyNulls`:
    - `pg_query.SortByNulls_SORTBY_NULLS_DEFAULT` (0)
    - `pg_query.SortByNulls_SORTBY_NULLS_FIRST` (1)
    - `pg_query.SortByNulls_SORTBY_NULLS_LAST` (2)

---

## 4. LockingClause Getters (FOR UPDATE / NOWAIT / SKIP LOCKED)

Pada `SelectStmt`:
- Path: `stmt.LockingClause []*pg_query.Node`
- Getter per elemen: `clause := item.GetLockingClause()`
  - `clause.Strength`:
    - `LockClauseStrength_LCS_FORKEYSHARE` (1)
    - `LockClauseStrength_LCS_FORSHARE` (2)
    - `LockClauseStrength_LCS_FORNOKEYUPDATE` (3)
    - `LockClauseStrength_LCS_FORUPDATE` (4)
  - `clause.WaitPolicy`:
    - `LockWaitPolicy_LockWaitBlock` (0 - Default: blocking sampai timeout)
    - `LockWaitPolicy_LockWaitSkip` (1 - `SKIP LOCKED`)
    - `LockWaitPolicy_LockWaitError` (2 - `NOWAIT`)
