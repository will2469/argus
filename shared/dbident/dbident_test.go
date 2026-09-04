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
