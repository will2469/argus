package a10_isolation_level

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
		t.Fatal(err)
	}
	analysistest.Run(t, rootDir, Analyzer,
		"./tests/correctness/a10/positive",
		"./tests/correctness/a10/negative",
	)
}

func TestIsolationEval_Unit(t *testing.T) {
	writeTests := []struct {
		name          string
		sql           string
		expectTable   string
		expectWritten bool
	}{
		{"balances update", "UPDATE balances SET amount = 100", "balances", true},
		{"inventory insert", "INSERT INTO inventory (id) VALUES (1)", "inventory", true},
		{"accounts delete", "DELETE FROM accounts WHERE id = 1", "accounts", true},
		{"non-critical table", "UPDATE user_preferences SET theme = 'dark'", "", false},
		{"schema qualified non-public", "UPDATE archive.balances SET amount = 100", "", false},
		{"audit log literal with balances", "INSERT INTO audit_log (message) VALUES ('updated balances successfully')", "", false},
	}

	for _, tc := range writeTests {
		ref, ok := ExtractCriticalWriteTable(tc.sql, nil)
		if ok != tc.expectWritten {
			t.Errorf("[%s] expected write=%v, got=%v (ref=%+v)", tc.name, tc.expectWritten, ok, ref)
		}
		if ok && ref.Name != tc.expectTable {
			t.Errorf("[%s] expected table name %q, got %q", tc.name, tc.expectTable, ref.Name)
		}
	}

	// Test Table-Correlated Row Locking
	balancesRef := TableRef{Name: "balances"}
	unrelatedLock := []TableRef{{Name: "unrelated_table"}}
	balancesLock := []TableRef{{Name: "balances"}}

	if isTableProtected(balancesRef, unrelatedLock, nil) {
		t.Errorf("CORRECTNESS BUG: unrelated table row lock must NOT protect balances")
	}
	if !isTableProtected(balancesRef, balancesLock, nil) {
		t.Errorf("CORRECTNESS BUG: balances row lock must protect balances")
	}

	// Test Advisory Lock Correlation
	unrelatedAdvisory := []string{"select pg_advisory_xact_lock(999);"}
	correlatedAdvisory := []string{"select pg_advisory_xact_lock(hashtext('balances:' || id));"}

	if isTableProtected(balancesRef, nil, unrelatedAdvisory) {
		t.Errorf("CORRECTNESS BUG: unrelated advisory lock (999) must NOT protect balances")
	}
	if !isTableProtected(balancesRef, nil, correlatedAdvisory) {
		t.Errorf("CORRECTNESS BUG: correlated advisory lock must protect balances")
	}
}

func TestTxSoundness_CalculatorAndShadowing(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main

import (
	"context"
	"database/sql"
)

type Pool interface {
	Begin(ctx context.Context) (*sql.Tx, error)
}

type Calculator struct{}
func (Calculator) Exec(sql string) {}

type TokenParser struct{}
func (TokenParser) Begin() *TokenParser { return &TokenParser{} }

func testNonDBBegin(p TokenParser) {
	tx := p.Begin()
	_ = tx
}

func testCalculatorExec(ctx context.Context, pool Pool, calc Calculator) {
	tx, _ := pool.Begin(ctx)
	calc.Exec("UPDATE balances SET amount = 1")
	_ = tx.Commit()
}

func testQueryShadowSafe(ctx context.Context, pool Pool) {
	query := "UPDATE balances SET amount = 1"
	tx, _ := pool.Begin(ctx)
	if true {
		query = "SELECT * FROM unrelated"
		_, _ = tx.Exec(query)
	}
	_ = tx.Commit()
	_ = query
}

func testQueryShadowViolated(ctx context.Context, pool Pool) {
	query := "SELECT * FROM unrelated"
	tx, _ := pool.Begin(ctx)
	if true {
		query = "UPDATE balances SET amount = 1"
		_, _ = tx.Exec(query)
	}
	_ = tx.Commit()
	_ = query
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		var issues []Issue
		inspectFunctionIsolation(nil, fset, fn, file, nil, nil, &issues)

		switch fn.Name.Name {
		case "testNonDBBegin":
			if len(issues) != 0 {
				t.Fatalf("expected 0 issues for TokenParser.Begin(), got %d: %v", len(issues), issues)
			}
		case "testCalculatorExec":
			if len(issues) != 0 {
				t.Fatalf("expected 0 issues for Calculator.Exec, got %d: %v", len(issues), issues)
			}
		case "testQueryShadowSafe":
			if len(issues) != 0 {
				t.Fatalf("expected 0 issues for shadowed safe query, got %d: %v", len(issues), issues)
			}
		case "testQueryShadowViolated":
			if len(issues) != 1 {
				t.Fatalf("expected 1 issue for shadowed violating query, got %d: %v", len(issues), issues)
			}
		}
	}
}

func TestSemanticHelperAndPoolRejection(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main

import (
	"context"
	"database/sql"
)

type Pool interface {
	Begin(ctx context.Context) (*sql.Tx, error)
}

type OrderService struct{}
func (OrderService) WithTx(a, b any) {}

type Calculator struct{}
func (Calculator) BeginFunc(a, b any) {}

type WorkerPool struct {
	workers []int
}
func (WorkerPool) Begin() {}

type DBHelper struct{}
func (DBHelper) WithTx(ctx context.Context, pool Pool, fn func(*sql.Tx) error) error {
	return nil
}

func testOrderServiceWithTx(svc OrderService) {
	svc.WithTx("balances", func() {
		_ = "UPDATE balances SET amount = 1"
	})
}

func testCalculatorBeginFunc(calc Calculator) {
	calc.BeginFunc("balances", "UPDATE balances SET amount = 1")
}

func testWorkerPoolBegin(wp WorkerPool) {
	wp.Begin()
}

func testGenuineHelperViolated(ctx context.Context, h DBHelper, p Pool) {
	h.WithTx(ctx, p, func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE balances SET amount = 1")
		return err
	})
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		var issues []Issue
		inspectFunctionIsolation(nil, fset, fn, file, nil, nil, &issues)

		switch fn.Name.Name {
		case "testOrderServiceWithTx":
			if len(issues) != 0 {
				t.Fatalf("expected 0 issues for OrderService.WithTx, got %d: %v", len(issues), issues)
			}
		case "testCalculatorBeginFunc":
			if len(issues) != 0 {
				t.Fatalf("expected 0 issues for Calculator.BeginFunc, got %d: %v", len(issues), issues)
			}
		case "testWorkerPoolBegin":
			if len(issues) != 0 {
				t.Fatalf("expected 0 issues for WorkerPool.Begin, got %d: %v", len(issues), issues)
			}
		case "testGenuineHelperViolated":
			if len(issues) != 1 {
				t.Fatalf("expected 1 issue for genuine DBHelper.WithTx, got %d: %v", len(issues), issues)
			}
		}
	}
}

