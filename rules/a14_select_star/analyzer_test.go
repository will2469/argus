package a14_select_star

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("failed to resolve rootDir: %v", err)
	}

	analysistest.Run(t, rootDir, Analyzer,
		"./tests/correctness/a14/positive",
		"./tests/correctness/a14/negative",
	)
}

func TestHasForbiddenSelectStar(t *testing.T) {
	cases := []struct {
		name      string
		sql       string
		forbidden bool
	}{
		{"PlainStar", "SELECT * FROM users", true},
		{"AliasStar", "SELECT u.* FROM users u", true},
		{"CTEStar", "WITH cte AS (SELECT * FROM a) SELECT id FROM cte", true},
		{"ExplicitCols", "SELECT id, name FROM users", false},
		{"CountStar", "SELECT COUNT(*) FROM users", false},
		{"ExistsStar", "SELECT id FROM users WHERE EXISTS (SELECT * FROM orders WHERE orders.user_id = users.id)", false},
		{"NotExistsStar", "SELECT id FROM users WHERE NOT EXISTS (SELECT * FROM orders WHERE orders.user_id = users.id)", false},
	}

	for _, tc := range cases {
		got := HasForbiddenSelectStar(tc.sql)
		if got != tc.forbidden {
			t.Errorf("[%s] expected forbidden=%v, got=%v for SQL:\n%s", tc.name, tc.forbidden, got, tc.sql)
		}
	}
}

func TestCheckDynamicQueryRisk(t *testing.T) {
	cases := []struct {
		name   string
		code   string
		isRisk bool
	}{
		{"DynamicConcatColumns", `"SELECT " + cols + " FROM users"`, true},
		{"StaticStarDynamicTable", `"SELECT * FROM " + table`, true},
		{"StaticExplicitColsDynamicTable", `"SELECT id, name FROM " + table`, false},
		{"StaticWhereDynamicCond", `"SELECT id, name FROM users WHERE id = " + id`, false},
		{"SprintfDynamicCols", `fmt.Sprintf("SELECT %s FROM users", cols)`, true},
		{"SprintfStaticStar", `fmt.Sprintf("SELECT * FROM %s", table)`, true},
		{"SprintfExplicitCols", `fmt.Sprintf("SELECT id, name FROM %s", table)`, false},
	}

	for _, tc := range cases {
		src := "package test\nimport \"fmt\"\nvar _ = " + tc.code
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "test.go", src, 0)
		if err != nil {
			t.Fatalf("[%s] parse failed: %v", tc.name, err)
		}
		valSpec := file.Decls[1].(*ast.GenDecl).Specs[0].(*ast.ValueSpec)
		expr := valSpec.Values[0]

		risk, _ := CheckDynamicQueryRisk(nil, file, expr, token.NoPos)
		if risk != tc.isRisk {
			t.Errorf("[%s] expected isRisk=%v, got=%v", tc.name, tc.isRisk, risk)
		}
	}
}

func TestInspectFile_ScopeResolution(t *testing.T) {
	src := `package test
import (
	"context"
	"database/sql"
)
type DB interface {
	Exec(ctx context.Context, sql string) (sql.Result, error)
	Query(ctx context.Context, sql string) (*sql.Rows, error)
}

var packageStarQuery = "SELECT * FROM users"

func TestPackageQuery(ctx context.Context, db DB) {
	db.Query(ctx, packageStarQuery)
}

func TestShadowedSafe(ctx context.Context, db DB) {
	query := "SELECT * FROM audit"
	_ = query
	if true {
		query := "SELECT id, name FROM users"
		db.Query(ctx, query)
	}
}

func TestDynamicColumns(ctx context.Context, db DB, cols string) {
	query := "SELECT " + cols + " FROM users"
	db.Query(ctx, query)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	issues := InspectFile(nil, fset, file, nil)
	// Expected:
	// 1. TestPackageQuery uses packageStarQuery (SELECT *) -> flagged
	// 2. TestShadowedSafe uses local safe query -> NOT flagged
	// 3. TestDynamicColumns uses dynamic columns -> flagged
	if len(issues) != 2 {
		t.Fatalf("expected exactly 2 issues, got %d: %v", len(issues), issues)
	}
}

func TestInspectFile_NonDBReceiver_ZeroFalsePositives(t *testing.T) {
	src := `package test
import "context"

type Calculator struct{}

func (Calculator) Query(ctx context.Context, formula string) (any, error) {
	return nil, nil
}

type SearchEngine interface {
	Query(ctx context.Context, q string) (any, error)
	Exec(ctx context.Context, q string) (int, error)
}

func TestCalculator(ctx context.Context, engine SearchEngine) {
	calculator := Calculator{}
	calculator.Query(ctx, "SELECT * FROM users")

	repo := Calculator{}
	repo.Query(ctx, "SELECT * FROM users")

	store := Calculator{}
	store.Query(ctx, "SELECT * FROM users")

	fakeStore := FakeStore{}
	fakeStore.Query(ctx, "SELECT * FROM users")

	engine.Query(ctx, "SELECT * FROM users")
	engine.Exec(ctx, "SELECT * FROM users")
}

type FakeStore struct {
	DB any
}
func (FakeStore) Query(ctx context.Context, q string) (any, error) { return nil, nil }

`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for non-DB receivers, got %d: %v", len(issues), issues)
	}
}

