package dbident

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

func TestIsKnownDBPackagePath(t *testing.T) {
	paths := []struct {
		path string
		want bool
	}{
		{"database/sql", true},
		{"github.com/jackc/pgx/v5", true},
		{"github.com/jackc/pgx/v5/pgxpool", true},
		{"github.com/jackc/pgx/v5/pgconn", true},
		{"github.com/jackc/pgx/v4", true},
		{"github.com/jackc/pgx/v4/pgxpool", true},
		{"github.com/jmoiron/sqlx", true},
		{"github.com/lib/pq", true},
		{"net/http", false},
		{"os", false},
		{"github.com/example/mydb", false},
		{"", false},
	}
	for _, tt := range paths {
		if got := IsKnownDBPackagePath(tt.path); got != tt.want {
			t.Errorf("IsKnownDBPackagePath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestDefaultPackageName(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"database/sql", "sql"},
		{"github.com/jackc/pgx/v5", "pgx"},
		{"github.com/jackc/pgx/v5/pgxpool", "pgxpool"},
		{"github.com/jackc/pgx/v5/pgconn", "pgconn"},
		{"github.com/jmoiron/sqlx", "sqlx"},
		{"github.com/lib/pq", "pq"},
		{"encoding/json", "json"},
		{"net/http", "http"},
		{"github.com/example/mylib", "mylib"},
	}
	for _, tt := range cases {
		if got := DefaultPackageName(tt.path); got != tt.want {
			t.Errorf("DefaultPackageName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestUnwrapPointer(t *testing.T) {
	basicStr := types.Typ[types.String]
	ptr1 := types.NewPointer(basicStr)
	ptr2 := types.NewPointer(ptr1)

	if got := UnwrapPointer(basicStr); got != basicStr {
		t.Errorf("UnwrapPointer(string) should return string")
	}
	if got := UnwrapPointer(ptr1); got != basicStr {
		t.Errorf("UnwrapPointer(*string) should return string")
	}
	if got := UnwrapPointer(ptr2); got != basicStr {
		t.Errorf("UnwrapPointer(**string) should return string")
	}
}

func TestHasInvalidType(t *testing.T) {
	if !HasInvalidType(nil) {
		t.Error("HasInvalidType(nil) should be true")
	}
	if HasInvalidType(types.Typ[types.String]) {
		t.Error("HasInvalidType(string) should be false")
	}
	if !HasInvalidType(types.Typ[types.Invalid]) {
		t.Error("HasInvalidType(Invalid) should be true")
	}
	ptrInvalid := types.NewPointer(types.Typ[types.Invalid])
	if !HasInvalidType(ptrInvalid) {
		t.Error("HasInvalidType(*Invalid) should be true")
	}
}

func TestGetASTTypeName(t *testing.T) {
	cases := []struct {
		name string
		expr ast.Expr
		want string
	}{
		{"ident", &ast.Ident{Name: "Tx"}, "Tx"},
		{"star_ident", &ast.StarExpr{X: &ast.Ident{Name: "DB"}}, "DB"},
		{"selector", &ast.SelectorExpr{X: &ast.Ident{Name: "sql"}, Sel: &ast.Ident{Name: "DB"}}, "DB"},
		{"nil", nil, ""},
	}
	for _, tt := range cases {
		if got := GetASTTypeName(tt.expr); got != tt.want {
			t.Errorf("GetASTTypeName(%s) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestIsDBConstructorMethod(t *testing.T) {
	constructors := []string{"Open", "OpenDB", "Connect", "ConnectConfig", "New", "NewWithConfig"}
	for _, m := range constructors {
		if !IsDBConstructorMethod(m) {
			t.Errorf("IsDBConstructorMethod(%q) should be true", m)
		}
	}
	nonConstructors := []string{"Query", "Exec", "Begin", "Close", "Ping", "Create"}
	for _, m := range nonConstructors {
		if IsDBConstructorMethod(m) {
			t.Errorf("IsDBConstructorMethod(%q) should be false", m)
		}
	}
}

// TestFakeTxInterface_MustBeRejected is the proof that purely structural
// interface matching is eliminated. A FakeTx with Commit, Rollback, Exec
// methods (all returning error, no driver type provenance) must NOT be
// classified as a database transaction.
func TestFakeTxInterface_MustBeRejected(t *testing.T) {
	fakeTx := buildInterface(t,
		method("Commit", nil, errorResult()),
		method("Rollback", nil, errorResult()),
		method("Exec", stringParam(), errorResult()),
	)

	if IsProvenDBTxType(fakeTx) {
		t.Fatal("PROVENANCE FAILURE: FakeTx{Commit() error; Rollback() error; Exec(string) error} " +
			"was ACCEPTED as a DB transaction. This is the exact false positive we must eliminate.")
	}

	if IsProvenClosureTxType(fakeTx) {
		t.Fatal("PROVENANCE FAILURE: FakeTx was ACCEPTED by IsProvenClosureTxType")
	}
}

// TestFakeTxWithDriverProvenance_MustBeRejected verifies that even if an interface has
// sql.Result in its Exec signature, it is STILL rejected as a DB transaction because
// real DB transactions holding physical connection locks are strictly concrete driver types.
func TestFakeTxWithDriverProvenance_MustBeRejected(t *testing.T) {
	sqlPkg := types.NewPackage("database/sql", "sql")
	resultNamed := types.NewNamed(types.NewTypeName(token.NoPos, sqlPkg, "Result", nil), types.NewInterfaceType(nil, nil), nil)
	errType := types.Universe.Lookup("error").Type()

	fakeTxWithProv := buildInterface(t,
		method("Commit", nil, errorResult()),
		method("Rollback", nil, errorResult()),
		method("Exec", stringParam(), types.NewTuple(
			types.NewVar(token.NoPos, nil, "", resultNamed),
			types.NewVar(token.NoPos, nil, "", errType),
		)),
	)

	if IsProvenDBTxType(fakeTxWithProv) {
		t.Fatal("PROVENANCE FAILURE: FakeTx with sql.Result was ACCEPTED as a DB transaction. " +
			"Only concrete driver transactions (sql.Tx, pgx.Tx) can hold DB connection locks.")
	}
}

// TestFakePoolInterface_MustBeRejected ensures that an interface with Begin
// returning a non-DB type is not classified as a database pool.
func TestFakePoolInterface_MustBeRejected(t *testing.T) {
	fakePool := buildInterface(t,
		method("Begin", nil, interfaceResult()),
	)

	if IsProvenDBPoolInterface(fakePool) {
		t.Fatal("PROVENANCE FAILURE: FakePool{Begin() (any, error)} was ACCEPTED as a DB pool")
	}
}

// TestFakePoolReturningFakeTx_MustBeRejected ensures that FakePool -> Begin() FakeTx
// is rejected because FakeTx is not an authentic driver transaction.
func TestFakePoolReturningFakeTx_MustBeRejected(t *testing.T) {
	fakeTx := buildInterface(t,
		method("Commit", nil, errorResult()),
		method("Rollback", nil, errorResult()),
	)
	errType := types.Universe.Lookup("error").Type()
	fakePool := buildInterface(t,
		method("Begin", nil, types.NewTuple(
			types.NewVar(token.NoPos, nil, "", fakeTx),
			types.NewVar(token.NoPos, nil, "", errType),
		)),
	)

	if IsProvenDBPoolInterface(fakePool) {
		t.Fatal("PROVENANCE FAILURE: FakePool returning FakeTx was ACCEPTED as a DB pool")
	}
}

// TestRealPoolReturningSqlTx_MustBeAccepted ensures that Pool -> Begin() (*sql.Tx, error)
// is correctly recognized as a genuine DB pool interface.
func TestRealPoolReturningSqlTx_MustBeAccepted(t *testing.T) {
	sqlPkg := types.NewPackage("database/sql", "sql")
	txNamed := types.NewNamed(types.NewTypeName(token.NoPos, sqlPkg, "Tx", nil), types.NewStruct(nil, nil), nil)
	txPtr := types.NewPointer(txNamed)
	errType := types.Universe.Lookup("error").Type()

	realPool := buildInterface(t,
		method("Begin", nil, types.NewTuple(
			types.NewVar(token.NoPos, nil, "", txPtr),
			types.NewVar(token.NoPos, nil, "", errType),
		)),
	)

	if !IsProvenDBPoolInterface(realPool) {
		t.Fatal("PROVENANCE FAILURE: RealPool returning *sql.Tx was REJECTED by IsProvenDBPoolInterface")
	}
}

// TestCalculatorQuerier_MustBeRejected tests that a single-method Exec interface
// (e.g. Calculator with Exec(string) (sql.Result, error)) is rejected because it lacks Query.
func TestCalculatorQuerier_MustBeRejected(t *testing.T) {
	sqlPkg := types.NewPackage("database/sql", "sql")
	resultNamed := types.NewNamed(types.NewTypeName(token.NoPos, sqlPkg, "Result", nil), types.NewInterfaceType(nil, nil), nil)
	errType := types.Universe.Lookup("error").Type()

	calcQuerier := buildInterface(t,
		method("Exec", stringParam(), types.NewTuple(
			types.NewVar(token.NoPos, nil, "", resultNamed),
			types.NewVar(token.NoPos, nil, "", errType),
		)),
	)

	if IsProvenDBQuerierType(calcQuerier) {
		t.Fatal("PROVENANCE FAILURE: Calculator{Exec(string) (sql.Result, error)} was ACCEPTED as a DB querier. " +
			"Queriers must implement BOTH Exec AND Query returning driver types.")
	}
}

// TestSearchEngineQuerier_MustBeRejected tests that a single-method Query interface
// (e.g. SearchEngine with Query(string) (*sql.Rows, error)) is rejected because it lacks Exec.
func TestSearchEngineQuerier_MustBeRejected(t *testing.T) {
	sqlPkg := types.NewPackage("database/sql", "sql")
	rowsNamed := types.NewNamed(types.NewTypeName(token.NoPos, sqlPkg, "Rows", nil), types.NewStruct(nil, nil), nil)
	rowsPtr := types.NewPointer(rowsNamed)
	errType := types.Universe.Lookup("error").Type()

	searchQuerier := buildInterface(t,
		method("Query", stringParam(), types.NewTuple(
			types.NewVar(token.NoPos, nil, "", rowsPtr),
			types.NewVar(token.NoPos, nil, "", errType),
		)),
	)

	if IsProvenDBQuerierType(searchQuerier) {
		t.Fatal("PROVENANCE FAILURE: SearchEngine{Query(string) (*sql.Rows, error)} was ACCEPTED as a DB querier. " +
			"Queriers must implement BOTH Exec AND Query returning driver types.")
	}
}

// TestDBExecutor_CustomInterfaceInIsolation_MustBeRejected ensures that custom interfaces
// without proven implementation are rejected in isolation (UNKNOWN / NOT SAFE).
func TestDBExecutor_CustomInterfaceInIsolation_MustBeRejected(t *testing.T) {
	sqlPkg := types.NewPackage("database/sql", "sql")
	resultNamed := types.NewNamed(types.NewTypeName(token.NoPos, sqlPkg, "Result", nil), types.NewInterfaceType(nil, nil), nil)
	rowsNamed := types.NewNamed(types.NewTypeName(token.NoPos, sqlPkg, "Rows", nil), types.NewStruct(nil, nil), nil)
	rowsPtr := types.NewPointer(rowsNamed)
	errType := types.Universe.Lookup("error").Type()

	dbExecutor := buildInterface(t,
		method("Exec", stringParam(), types.NewTuple(
			types.NewVar(token.NoPos, nil, "", resultNamed),
			types.NewVar(token.NoPos, nil, "", errType),
		)),
		method("Query", stringParam(), types.NewTuple(
			types.NewVar(token.NoPos, nil, "", rowsPtr),
			types.NewVar(token.NoPos, nil, "", errType),
		)),
	)

	if IsProvenDBQuerierType(dbExecutor) {
		t.Fatal("PROVENANCE FAILURE: Custom interface without proven implementation was ACCEPTED by IsProvenDBQuerierType")
	}
}

// TestEvilImplementation_MustBeRejected verifies the user's adversarial scenario:
// FakeDB interface implemented by Evil struct returning (nil, nil) must NOT be proven.
func TestEvilImplementation_MustBeRejected(t *testing.T) {
	sqlPkg := types.NewPackage("database/sql", "sql")
	resultNamed := types.NewNamed(types.NewTypeName(token.NoPos, sqlPkg, "Result", nil), types.NewInterfaceType(nil, nil), nil)
	rowsNamed := types.NewNamed(types.NewTypeName(token.NoPos, sqlPkg, "Rows", nil), types.NewStruct(nil, nil), nil)
	rowsPtr := types.NewPointer(rowsNamed)
	errType := types.Universe.Lookup("error").Type()

	fakeDB := buildInterface(t,
		method("Exec", stringParam(), types.NewTuple(
			types.NewVar(token.NoPos, nil, "", resultNamed),
			types.NewVar(token.NoPos, nil, "", errType),
		)),
		method("Query", stringParam(), types.NewTuple(
			types.NewVar(token.NoPos, nil, "", rowsPtr),
			types.NewVar(token.NoPos, nil, "", errType),
		)),
	)

	// Create test package with Evil struct implementing FakeDB
	testPkg := types.NewPackage("testpkg", "testpkg")
	evilNamed := types.NewNamed(types.NewTypeName(token.NoPos, testPkg, "Evil", nil), types.NewStruct(nil, nil), nil)
	recvVar := types.NewVar(token.NoPos, testPkg, "", evilNamed)
	execSig := types.NewSignatureType(recvVar, nil, nil, stringParam(), types.NewTuple(
		types.NewVar(token.NoPos, nil, "", resultNamed),
		types.NewVar(token.NoPos, nil, "", errType),
	), false)
	querySig := types.NewSignatureType(recvVar, nil, nil, stringParam(), types.NewTuple(
		types.NewVar(token.NoPos, nil, "", rowsPtr),
		types.NewVar(token.NoPos, nil, "", errType),
	), false)
	evilNamed.AddMethod(types.NewFunc(token.NoPos, testPkg, "Exec", execSig))
	evilNamed.AddMethod(types.NewFunc(token.NoPos, testPkg, "Query", querySig))
	testPkg.Scope().Insert(evilNamed.Obj())

	// Evil direct check
	if IsProvenDBQuerierType(evilNamed) {
		t.Fatal("Evil struct should NOT be recognized as DB querier")
	}

	// FakeDB with Evil implementation in pkg
	if IsProvenDBQuerierWithPkg(fakeDB, testPkg) {
		t.Fatal("FakeDB implemented by Evil struct without DB fields MUST be rejected as DB querier")
	}
}

// TestIsExactContextType ensures that context.Context is verified semantically,
// rejecting custom types named Context from non-context packages.
func TestIsExactContextType(t *testing.T) {
	ctxPkg := types.NewPackage("context", "context")
	realCtx := types.NewNamed(types.NewTypeName(token.NoPos, ctxPkg, "Context", nil), types.NewInterfaceType(nil, nil), nil)
	if !IsExactContextType(realCtx) {
		t.Error("real context.Context should be accepted")
	}

	fakePkg := types.NewPackage("my/app/pkg", "pkg")
	fakeCtx := types.NewNamed(types.NewTypeName(token.NoPos, fakePkg, "Context", nil), types.NewInterfaceType(nil, nil), nil)
	if IsExactContextType(fakeCtx) {
		t.Error("fake Context from non-context package must be rejected")
	}
}

func TestIsImportedDBPackageIdent(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
import (
	"database/sql"
	customdb "github.com/jackc/pgx/v5"
	"net/http"
)
`
	file, err := parser.ParseFile(fset, "test.go", src, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		want bool
	}{
		{"sql", true},
		{"customdb", true},
		{"http", false},
		{"pgx", false}, // not imported with default name
		{"", false},
	}
	for _, tt := range cases {
		if got := IsImportedDBPackageIdent(file, tt.name); got != tt.want {
			t.Errorf("IsImportedDBPackageIdent(file, %q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestHasDatabaseImports(t *testing.T) {
	fset := token.NewFileSet()

	withDB, _ := parser.ParseFile(fset, "db.go", `package main; import "database/sql"`, parser.ImportsOnly)
	if !HasDatabaseImports(withDB) {
		t.Error("HasDatabaseImports should be true for file importing database/sql")
	}

	withoutDB, _ := parser.ParseFile(fset, "nodb.go", `package main; import "net/http"`, parser.ImportsOnly)
	if HasDatabaseImports(withoutDB) {
		t.Error("HasDatabaseImports should be false for file importing only net/http")
	}

	// nil file should fail-open
	if !HasDatabaseImports(nil) {
		t.Error("HasDatabaseImports(nil) should be true (fail-open)")
	}
}

func TestIsKnownDBPoolASTType(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main; import "database/sql"`
	file, _ := parser.ParseFile(fset, "test.go", src, parser.ImportsOnly)

	pool := &ast.SelectorExpr{X: &ast.Ident{Name: "sql"}, Sel: &ast.Ident{Name: "DB"}}
	if !IsKnownDBPoolASTType(pool, file) {
		t.Error("sql.DB should be recognized as DB pool AST type")
	}

	notPool := &ast.SelectorExpr{X: &ast.Ident{Name: "sql"}, Sel: &ast.Ident{Name: "Tx"}}
	if IsKnownDBPoolASTType(notPool, file) {
		t.Error("sql.Tx should NOT be recognized as DB pool AST type")
	}
}

func TestIsKnownDBTxASTType(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main; import "database/sql"`
	file, _ := parser.ParseFile(fset, "test.go", src, parser.ImportsOnly)

	tx := &ast.SelectorExpr{X: &ast.Ident{Name: "sql"}, Sel: &ast.Ident{Name: "Tx"}}
	if !IsKnownDBTxASTType(tx, file) {
		t.Error("sql.Tx should be recognized as DB Tx AST type")
	}

	notTx := &ast.SelectorExpr{X: &ast.Ident{Name: "sql"}, Sel: &ast.Ident{Name: "DB"}}
	if IsKnownDBTxASTType(notTx, file) {
		t.Error("sql.DB should NOT be recognized as DB Tx AST type")
	}
}

func TestFindTypeSpec(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
type MyTx interface {
	Commit() error
}
type MyStruct struct {
	Name string
}`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	if ts := FindTypeSpec("MyTx", file); ts == nil {
		t.Error("FindTypeSpec should find MyTx")
	}
	if ts := FindTypeSpec("MyStruct", file); ts == nil {
		t.Error("FindTypeSpec should find MyStruct")
	}
	if ts := FindTypeSpec("Missing", file); ts != nil {
		t.Error("FindTypeSpec should return nil for missing type")
	}
}

// --- Test helpers for building types.Type programmatically ---

func buildInterface(t *testing.T, methods ...*types.Func) types.Type {
	t.Helper()
	return types.NewInterfaceType(methods, nil)
}

func method(name string, params *types.Tuple, results *types.Tuple) *types.Func {
	sig := types.NewSignatureType(nil, nil, nil, params, results, false)
	return types.NewFunc(token.NoPos, nil, name, sig)
}

func errorResult() *types.Tuple {
	errorType := types.Universe.Lookup("error").Type()
	return types.NewTuple(types.NewVar(token.NoPos, nil, "", errorType))
}

func stringParam() *types.Tuple {
	return types.NewTuple(types.NewVar(token.NoPos, nil, "s", types.Typ[types.String]))
}

func interfaceResult() *types.Tuple {
	return types.NewTuple(
		types.NewVar(token.NoPos, nil, "", types.NewInterfaceType(nil, nil)),
		types.NewVar(token.NoPos, nil, "", types.Universe.Lookup("error").Type()),
	)
}
