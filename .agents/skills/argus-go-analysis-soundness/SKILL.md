---
name: argus-go-analysis-soundness
description: "MANDATORY GO STATIC ANALYSIS SOUNDNESS GUARDIAN: Compiler-grade static analysis checklist, control-flow graph (CFG) invariants, type resolution rules, and provenance verification protocols for Argus rules analyzing Go source code. Enforces Type Resolution over Lexical AST Matching (preventing import alias and method spoofing bugs), Universal Path Completeness (eliminating the Existential Fallacy in switches, if-else, and directions), Value Provenance (verifying map/slice closed-set immutability), Scope Dominance (preventing conditional defer connection leaks and shadowing bugs), and Fail-Closed Lattice Joins for data-flow tracking. Auto-triggers whenever authoring, reviewing, refactoring, or auditing any Argus rule inspecting Go AST (A01-A10, A12, A14, A16-A26)."
compatibility: "Go 1.25+, golang.org/x/tools/go/analysis, go/ast, go/types, go/token"
metadata:
  version: "1.0.0"
  author: "Will (https://github.com/will2469)"
  license: "MIT"
  citations:
    - "CWE-89: Improper Neutralization of Special Elements used in an SQL Command"
    - "CWE-772: Missing Release of Resource after Effective Lifetime"
    - "CWE-400: Uncontrolled Resource Consumption"
    - "CWE-284: Improper Access Control"
    - "OWASP ASVS v4.0.3/v5.0 §V5.3 (Output Encoding and Injection Defense)"
    - "Aho, Lam, Sethi, Ullman: Compilers: Principles, Techniques, and Tools (Dragon Book 2nd Ed - Data Flow & SSA)"
    - "Edward L. Ginzton: Program Analysis and Specialization (Stanford CS343 - Lattice Frameworks)"
    - "golang.org/x/tools/go/analysis & golang.org/x/tools/go/ssa"
---

# Go Static Analysis Soundness & QC Auditor Guardian (`argus-go-analysis-soundness`)

> **Core Mandate:** Argus bertindak sebagai **Independent QA/QC Compiler & Security Auditor** berstandar industri tinggi (CWE, OWASP ASVS v5.0 L3). Pemeriksaan yang dangkal, permisif, atau hanya berbasis pencocokan leksikal teks adalah **kegagalan sistemik**.
>
> **Prinsip Utama Rekayasa Analyzer:**
> > *"Jangan upgrade analyzer dari syntactic matching langsung ke 'smart heuristics'. Upgrade ke semantic evidence."*
>
> Properti mutlak: **`SAFE means provably safe`**, BUKAN **`SAFE means analyzer menemukan pola yang kelihatan aman`**.
> Penilai Argus wajib bersikap sebagai **Auditor Netral Pihak Ketiga**: menghilangkan *Ownership Bias* (kecenderungan optimis pembuat kode menilai pekerjaannya sendiri "pasti aman"), menolak kepuasan pola dangkal, dan menuntut kepastian bukti semantik (*semantic provenance*).
>
> Menulis `ast.Inspect()` bukan berarti bebas dari jebakan! **Pencocokan nama string pada node AST (`ident.Name == "context"`, `sel.Sel.Name == "Sanitize"`, `rhs.(*ast.IndexExpr)`) hanyalah regex berkedok pohon sintaks (Lexical AST Matching).**
> Analyzer yang sejati wajib berakar pada **Semantic Type Resolution (`go/types`)**, **Universal Path Completeness ($\forall$-Paths Invariant)**, **Value Provenance**, dan **Scope Dominance**.


---

## 1. The 5 Cardinal Static Analysis Traps in Go

Setiap analyzer Argus yang memeriksa kode Go wajib membentengi diri dari 5 jebakan yang terbukti memicu false negative dan false positive:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                 THE 5 GO STATIC ANALYSIS SOUNDNESS TRAPS                    │
├─────────────────────────────────────────────────────────────────────────────┤
│ 1. The Lexical Matching Trap  │ ident.Name == "context" vs import alias     │
│ 2. The Existential Fallacy    │ Exists ONE safe branch (∃) vs ALL paths (∀) │
│ 3. Syntax != Provenance Trap  │ *ast.IndexExpr != closed-set allowlist map  │
│ 4. Flat Scope Blindness       │ Flat ast.Inspect vs Shadowing & Dominance   │
│ 5. Flow-Insensitive Taint     │ Flat map vs Reassignment & Lattice Join    │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### Trap 1: The Lexical Matching Trap (Identitas Berbasis String)

- **Gejala Cacat:**
  - `ident.Name == "context"` $\rightarrow$ Bocor jika developer menulis `import stdctx "context"`.
  - `sel.Sel.Name == "Sanitize"` $\rightarrow$ Menerima method palsu `evil.Sanitize(userInput)` sebagai sanitizer valid.
  - `call.Fun.(*ast.Ident).Name == "Query"` $\rightarrow$ Menandai `engine.Query()` pada search engine non-DB sebagai kebocoran cursor.
- **Prinsip Soundness:**
  > **"Nama identifier bukan bukti tipe. Tipe riil hanya dibuktikan oleh Type Checker (`go/types`)."**
- **Solusi Wajib:**
  1. Gunakan `pass.TypesInfo.Uses[ident]` untuk mendapatkan `types.Object`.
  2. Untuk package, verifikasi `pkgName, ok := obj.(*types.PkgName); ok && pkgName.Imported().Path() == "context"`.
  3. Untuk receiver method, verifikasi tipe struktural receiver melalui `pass.TypesInfo.TypeOf(sel.X)` atau interface yang diimplementasikan (`types.Implements`).
  4. Sediakan fallback mapping import AST (`file.Imports`) untuk mode Standalone Runner tanpa full type-checker.

---

### Trap 2: The Existential Fallacy ($\exists$ vs $\forall$ Path Completeness)

- **Gejala Cacat:**
  - **Switch Fallacy:** Menemukan *satu* `case` yang memberikan string statis, lalu menganggap seluruh switch aman, padahal `default: col = userSort` meloloskan raw input.
  - **Direction Fallacy:** Menemukan ekspresi `dir == "DESC"` atau `dir = "DESC"`, lalu menganggap aman, padahal inisialisasi awal `dir := userDir` meloloskan SQL injection jika nilainya bukan `"DESC"`.
  - **Branching Fallacy:** `if condition { ctx, cancel = ... }` dianggap aman tanpa memeriksa bahwa pada jalur `else`, `ctx` tetap `context.Background()`.
- **Prinsip Soundness:**
  > **"Keamanan tidak dibuktikan oleh keberadaan satu jalur aman ($\exists p \in Paths$). Keamanan wajib dibuktikan berlaku pada SELURUH jalur eksekusi yang reachable ($\forall p \in Paths$)."**
- **Solusi Wajib:**
  1. **Exhaustive Switch Checking:** Setiap branch non-default WAJIB meng-assign identifier aman ATAU menghentikan eksekusi secara permanen (`return`, `panic`, `os.Exit`, `log.Fatal`).
  2. **Default Clause Invariant:** Wajib ada `default` clause yang aman/terminating, KECUALI jika variabel telah diinisialisasi dengan nilai konstan aman SEBELUM switch dimasuki.
  3. **Path-Complete Direction / Enum Checking:** Menolak mutlak variabel yang memiliki assignment dari ekspresi non-literal. Seluruh jalur wajib berakhir pada himpunan nilai yang diizinkan (`"ASC"` / `"DESC"`).

---

### Trap 3: Syntax != Provenance Trap (Tipe Node Sintaksis $\neq$ Jaminan Nilai)

- **Gejala Cacat:**
  - Menemukan `*ast.IndexExpr` (`m[k]`) dan langsung menganggapnya sebagai allowlist map.
  - Padahal map-nya bisa:
    ```go
    sortMap := getMapFromRequest() // Runtime unverified
    // atau
    sortMap[userKey] = userInput   // Mutasi dinamis di runtime
    ```
- **Prinsip Soundness:**
  > **"Bentuk sintaksis (syntax node) hanyalah wadah ekspresi. Yang harus dibuktikan untuk closed-set allowlist adalah Provenance (Asal-Usul & Imutabilitas Nilai)."**
- **Solusi Wajib:**
  1. Telusuri identifier map `X` ke deklarasinya (`findLocalMapLiteral` atau `findPackageMapLiteral`).
  2. Pastikan map dideklarasikan sebagai composite literal `map[K]V{...}` di mana **seluruh nilainya** berupa `token.STRING` literal konstan.
  3. Periksa seluruh fungsi untuk memastikan map tidak pernah dimutasi di runtime (`isMapDynamicallyMutated`: tidak ada `m[k] = ...`).
  4. Jika provenance gagal dibuktikan, assignment dari map lookup tersebut WAJIB ditandai sebagai **UNSAFE**.

---

### Trap 4: Flat Scope Blindness (Variable Shadowing & Scope Dominance)

- **Gejala Cacat:**
  - **Variable Shadowing:** `ast.Inspect` mencari nama `"ctx"` secara flat. Outer block mendefinisikan `ctx := context.Background()`, inner block mendefinisikan `ctx := r.Context()`. Analyzer salah menandai inner query sebagai pelanggaran, atau sebaliknya meloloskan outer query.
  - **Conditional Defer Trap:** Analyzer mencari `defer rows.Close()` di sembarang tempat. Developer menulis:
    ```go
    if condition {
        defer rows.Close() // Terperangkap! Jika condition == false, koneksi bocor!
    }
    ```
    Analyzer lama menganggap fungsi aman karena menemukan teks `defer rows.Close()`.
- **Prinsip Soundness:**
  > **"Program Go memiliki hierarki scope leksikal pohon, bukan flat bag. Cleanup resource wajib mendominasi alur kontrol (Dominance Invariant)."**
- **Solusi Wajib:**
  1. Gunakan `types.Object` unik per deklarasi variabel untuk membedakan shadowed variables.
  2. Dalam mode standalone AST, gunakan **Lexical Scope Stack** (`pushScope()`, `popScope()`).
  3. **Scope Dominance Invariant (CWE-772):** Sebuah `defer <res>.Close()` hanya valid jika berada pada statement list / block yang sama dengan alokasi resource atau scope induk yang mendominasi seluruh jalur keluar fungsi. Defer di dalam `if`, `for`, `switch`, atau `select` child block bersyarat WAJIB DITOLAK.

---

### Trap 5: Flow-Insensitive Taint Tracking & Missing Lattice Joins

- **Gejala Cacat:**
  - Menggunakan map flat global `taintedVars[varName] = true`.
  - Mengabaikan re-assignment:
    ```go
    q := userInput  // tainted
    q = "SELECT 1"  // clean override - analyzer lama tetap menandai tainted!
    // atau
    q := "SELECT 1" // clean
    q = userInput   // dirty override - analyzer lama menganggap clean!
    ```
  - Mengabaikan join percabangan:
    ```go
    if cond { q = userInput } else { q = "SELECT 1" }
    // Jalur then tainted, jalur else clean.
    ```
- **Prinsip Soundness:**
  > **"Data flow analysis wajib bersifat flow-sensitive dengan state lattice yang di-join pada setiap titik konvergensi control flow."**
- **Solusi Wajib:**
  1. Lacak state program-point per statement.
  2. Saat menemui percabangan (`ast.IfStmt`), lakukan lattice join:
     $$State_{after} = State_{then} \sqcup State_{else}$$
  3. **Fail-Closed Principle:** Jika suatu variabel berstatus *tainted / unclosed / raw* pada **salah satu** branch yang reachable, status setelah join adalah **TAINTED / RAW**.

---

## 2. The 5-Phase Compiler-Grade Static Analysis Checklist

Sebelum memfinalisasi atau menyetujui rule analyzer Go baru:

- [ ] **1. Type Resolution:**
  - Apakah package pencocokan diverifikasi via `types.PkgName` / `Imported().Path()`?
  - Apakah receiver method diverifikasi tipenya, bukan hanya mencocokkan string nama method?
  - Apakah import alias (`import ctx "context"`) ditangani dengan benar?
- [ ] **2. Universal Path Completeness ($\forall$-Paths):**
  - Apakah evaluasi kondisi berlaku untuk **seluruh** jalur eksekusi yang reachable?
  - Apakah switch statement memverifikasi seluruh branch dan klausul default?
  - Apakah ada cabang `if-else` yang meloloskan fallback data mentah?
- [ ] **3. Value & Allowlist Provenance:**
  - Apakah map/slice allowlist diverifikasi sebagai composite literal konstan?
  - Apakah seluruh elemen nilainya berupa string literal compile-time?
  - Apakah ada mutasi dinamis (`assign to index`) pada data allowlist?
- [ ] **4. Scope Hierarchy & Dominance:**
  - Apakah defer/cleanup berada di enclosing scope yang mendominasi penggunaan resource?
  - Apakah defer di dalam child block bersyarat (`if`, `for`) ditolak secara tegas?
  - Apakah variabel yang di-shadow di inner block dibedakan identitasnya?
- [ ] **5. Fail-Closed Data Flow & Lattice Join:**
  - Apakah state digabungkan (*join*) secara union pada akhir percabangan?
  - Apakah re-assignment (clean override dan dirty override) memperbarui status variabel secara tepat?

---

## 3. Quick Code Patterns: The Good, The Bad, and The Sound

### Pattern 1: Memverifikasi Package Type-Safe (Bukan Lexical Match)

```go
// ❌ BAD: Lexical AST Matching (Bocor pada import alias)
if id, ok := sel.X.(*ast.Ident); ok && id.Name == "context" { ... }

// ✅ GOOD: Type-Aware Package Verification
if pass != nil && pass.TypesInfo != nil {
    if id, ok := sel.X.(*ast.Ident); ok {
        if pkgName, ok := pass.TypesInfo.Uses[id].(*types.PkgName); ok {
            if pkgName.Imported().Path() == "context" {
                return true // 100% sound, import alias apa pun tertangkap!
            }
        }
    }
}
```

### Pattern 2: Memverifikasi Exhaustive Switch (Bukan Single-Branch Match)

```go
// ❌ BAD: Existential Fallacy (1 safe branch menganggap seluruh switch aman)
for _, cc := range sw.Body.List {
    if branchHasStaticString(cc) { return true } // BUG: default: col = userSort lolos!
}

// ✅ GOOD: Universal Path Completeness
hasDefault := false
for _, stmt := range sw.Body.List {
    cc := stmt.(*ast.CaseClause)
    if len(cc.List) == 0 { hasDefault = true }
    if isTerminating(cc.Body) { continue } // return, panic -> safe
    if !branchAssignsApprovedStaticLiteral(cc, varName) {
        return false // ANY branch assigning unapproved expr invalidates switch!
    }
}
if !hasDefault && !isVarSafelyInitializedPrior(varName, sw) {
    return false // Unhandled case leaves variable unsafe!
}
return true
```

### Pattern 3: Memverifikasi Scope Dominance pada Defer (CWE-772)

```go
// ❌ BAD: Flat Search (Defer di dalam if-branch dianggap aman)
ast.Inspect(body, func(n ast.Node) bool {
    if isDeferClose(n) { hasDefer = true } // BUG: if cond { defer rows.Close() } lolos!
    return true
})

// ✅ GOOD: Scope Dominance Enforcement
queryBlock := findEnclosingStmtList(body, queryStmt)
deferBlock := findEnclosingStmtList(body, deferStmt)
// Defer wajib berada di statement list yang sama dengan query atau scope leluhur langsung
if queryBlock != deferBlock {
    return false // Trapped in conditional or loop block -> LEAK!
}
```
