# The BoolExpr Disjunctive Trap & The A24 Post-Mortem

Dokumentasi mendalam mengenai perbedaan semantik predikat konjungtif (AND) vs disjungtif (OR) pada AST PostgreSQL dan post-mortem bug lolosnya isolasi tenant di ARGUS-A24.

---

## 1. Anatomi Bug ARGUS-A24 (The BoolExpr Blind Walk Trap)

Pada implementasi awal `rules/a24_tenant_leak/tenant_ast_check.go`:

```go
// KODE BERBAHAYA (BUG A24):
if bexpr := n.GetBoolExpr(); bexpr != nil {
    for _, arg := range bexpr.Args {
        walk(arg) // Memeriksa semua argumen tanpa peduli apakah AND atau OR!
    }
}
```

### Mengapa Ini Merupakan Celah Keamanan Fatal (BOLA / CWE-284)?
Misalkan terdapat kueri multi-tenant berikut:
```sql
SELECT * FROM orders WHERE status = 'COMPLETED' OR tenant_id = '123'
```
Pohon AST PostgreSQL untuk kueri di atas adalah:
- `SelectStmt.WhereClause` -> `BoolExpr`
  - `Boolop`: `OR_EXPR` (Disjunction)
  - `Args[0]`: `status = 'COMPLETED'`
  - `Args[1]`: `tenant_id = '123'`

Jika analyzer hanya melakukan penelusuran rekursif polos (`walk`) pada seluruh node anak `BoolExpr`, fungsi akan menemukan `tenant_id` di `Args[1]` dan menyimpulkan bahwa kueri **AMAN** (memiliki filter tenant).

**Realitas PostgreSQL Engine:**
Database akan mengembalikan **SEMUA baris yang memiliki `status = 'COMPLETED'` dari SELURUH tenant**, karena operator `OR` mengevaluasi benar jika salah satu sisi terpenuhi! Data tenant lain bocor secara masif (Broken Object Level Authorization).

---

## 2. Aturan Invarian Semantik Predikat AST

Dalam static analysis, predikat keamanan (seperti `tenant_id`, `is_deleted = false`, atau `user_id = $1`) hanya sah jika memenuhi **Invarian Konjungtif Mutlak**:

| Jenis Boolop                    | Sifat Logika                        | Status Validasi Invarian Keamanan                                               |
| :------------------------------ | :---------------------------------- | :------------------------------------------------------------------------------ |
| `BoolExprType_AND_EXPR` (1)     | Konjungtif: $P_1 \land P_2 \land \dots$ | **VALID:** Jika predikat target ada di salah satu anak AND, kueri terisolasi.   |
| `BoolExprType_OR_EXPR` (2)      | Disjungtif: $P_1 \lor P_2 \lor \dots$   | **INVALID:** Keberadaan predikat di satu sisi OR tidak mengisolasi sisi lainnya. |
| `BoolExprType_NOT_EXPR` (3)     | Negasi: $\neg P$                    | **INVALID:** Membalikkan logika (memilih baris selain tenant tersebut).         |

---

## 3. Pola Implementasi Aman (Safe Traversal)

Gunakan pola penelusuran terarah seperti pada [`assets/safe_predicate_extractor.go`](../assets/safe_predicate_extractor.go):

```go
if bexpr := node.GetBoolExpr(); bexpr != nil {
    switch bexpr.Boolop {
    case pg_query.BoolExprType_AND_EXPR:
        // Telusuri anak-anak konjungsi
        for _, arg := range bexpr.Args {
            if inspectConjunctiveNode(arg, targets) {
                return true
            }
        }
        return false
    case pg_query.BoolExprType_OR_EXPR, pg_query.BoolExprType_NOT_EXPR:
        // STOP: Jangan anggap anak di bawah OR/NOT sebagai filter isolasi universal!
        return false
    }
}
```
