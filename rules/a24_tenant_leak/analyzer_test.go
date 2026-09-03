package a24_tenant_leak

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	testdata, err := filepath.Abs("../../testdata")
	if err != nil {
		t.Fatalf("failed to resolve testdata path: %v", err)
	}

	analysistest.Run(t, testdata, Analyzer, "a24")
}

func TestCheckTenantQuery(t *testing.T) {
	tc := &TenantConfig{
		TenantColumn: "tenant_id",
		TenantTables: map[string]bool{
			"users":     true,
			"orders":    true,
			"customers": true,
		},
	}

	cases := []struct {
		sql        string
		violating  bool
		wantReason string
	}{
		{
			sql:       "SELECT id, name FROM users WHERE tenant_id = $1",
			violating: false,
		},
		{
			sql:       "SELECT id, name FROM users WHERE u.tenant_id = $1",
			violating: false,
		},
		{
			sql:       "SELECT id, name FROM users WHERE status = 'ACTIVE'",
			violating: true,
		},
		{
			sql:       "UPDATE orders SET status = 'DONE' WHERE tenant_id = $1 AND id = $2",
			violating: false,
		},
		{
			sql:       "UPDATE orders SET status = 'DONE' WHERE id = $1",
			violating: true,
		},
		{
			sql:       "SELECT id, name FROM lookup_data WHERE active = true",
			violating: false,
		},
		{
			sql:       "SELECT id, name FROM users WHERE id = $1 OR tenant_id = $2",
			violating: true,
		},
		{
			sql:       "SELECT id, name FROM users WHERE (id = $1 OR status = 'A') AND tenant_id = $2",
			violating: false,
		},
		{
			sql:       "SELECT id, name FROM users WHERE (status = 'A' AND tenant_id = $1) OR (status = 'B' AND tenant_id = $1)",
			violating: false,
		},
		{
			sql:       "SELECT id, name FROM users WHERE NOT (tenant_id = $1)",
			violating: true,
		},
		{
			sql:       "SELECT id, name FROM users WHERE id = $1 AND (status = 'A' OR tenant_id = $2)",
			violating: true,
		},
		// P0 Adversarial operator & constraint validation cases
		{
			sql:       "SELECT id FROM users WHERE tenant_id IS NOT NULL",
			violating: true,
		},
		{
			sql:       "SELECT id FROM users WHERE tenant_id > 0",
			violating: true,
		},
		{
			sql:       "SELECT id FROM users WHERE tenant_id != $1",
			violating: true,
		},
		{
			sql:       "SELECT id FROM users WHERE tenant_id <> $1",
			violating: true,
		},
		// P0 Multi-table JOIN isolation cases
		{
			sql:       "SELECT u.id, c.name FROM users u JOIN customers c ON c.id = u.customer_id WHERE u.tenant_id = $1",
			violating: true,
		},
		{
			sql:       "SELECT u.id, c.name FROM users u JOIN customers c ON c.id = u.customer_id WHERE u.tenant_id = $1 AND c.tenant_id = $1",
			violating: false,
		},
		{
			sql:       "SELECT u.id, c.name FROM users u JOIN customers c ON c.id = u.customer_id AND c.tenant_id = $1 WHERE u.tenant_id = $2",
			violating: false,
		},
		{
			sql:       "SELECT u.id, c.name FROM users u JOIN customers c ON c.tenant_id = u.tenant_id WHERE u.tenant_id = $1",
			violating: false,
		},
	}

	for _, c := range cases {
		violating, _ := CheckTenantQuery(c.sql, tc)
		if violating != c.violating {
			t.Errorf("[%s] expected violating=%v, got %v", c.sql, c.violating, violating)
		}
	}
}

func TestCheckTenantQuery_AutoDetectTenantQuery(t *testing.T) {
	tc := &TenantConfig{
		TenantColumn: "tenant_id",
		TenantTables: map[string]bool{
			"tenant_data": true,
		},
	}

	// 1. Non-tenant query on unconfigured table SHOULD PASS
	if violating, _ := CheckTenantQuery("SELECT id, name FROM users WHERE id = $1", tc); violating {
		t.Errorf("expected non-tenant query on users to pass, got violating=true")
	}

	// 2. Query explicitly touching tenant_id with invalid predicate SHOULD FAIL
	if violating, _ := CheckTenantQuery("SELECT id FROM users WHERE tenant_id IS NOT NULL", tc); !violating {
		t.Errorf("expected invalid tenant predicate on users to fail, got violating=false")
	}

	// 3. Query explicitly touching tenant_id with valid predicate SHOULD PASS
	if violating, _ := CheckTenantQuery("SELECT id FROM users WHERE tenant_id = $1", tc); violating {
		t.Errorf("expected valid tenant predicate on users to pass, got violating=true")
	}
}

func TestIsRLSActiveAt_DominanceAnalysis(t *testing.T) {
	src := `package testpkg

func casePrecedingQuery() {
	db.Query(ctx, "SELECT * FROM users")
	db.Exec(ctx, "SET LOCAL app.tenant_id = $1", "123")
}

func caseSubsequentQuery() {
	db.Exec(ctx, "SET LOCAL app.tenant_id = $1", "123")
	db.Query(ctx, "SELECT * FROM users")
}

func caseConditionalRLS(isSpecial bool) {
	if isSpecial {
		db.Exec(ctx, "SET LOCAL app.tenant_id = $1", "123")
	}
	db.Query(ctx, "SELECT * FROM users")
}

func caseInsideBranch(isSpecial bool) {
	if isSpecial {
		db.Exec(ctx, "SET LOCAL app.tenant_id = $1", "123")
		db.Query(ctx, "SELECT * FROM users")
	}
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	for _, decl := range f.Decls {
		fn := decl.(*ast.FuncDecl)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Query" {
				return true
			}

			active := IsRLSActiveAt(fn.Body, call.Pos(), "tenant_id")
			switch fn.Name.Name {
			case "casePrecedingQuery":
				if active {
					t.Errorf("casePrecedingQuery: expected RLS inactive before setup")
				}
			case "caseSubsequentQuery":
				if !active {
					t.Errorf("caseSubsequentQuery: expected RLS active after setup")
				}
			case "caseConditionalRLS":
				if active {
					t.Errorf("caseConditionalRLS: expected RLS inactive after branch without else")
				}
			case "caseInsideBranch":
				if !active {
					t.Errorf("caseInsideBranch: expected RLS active inside branch after setup")
				}
			}
			return true
		})
	}
}
