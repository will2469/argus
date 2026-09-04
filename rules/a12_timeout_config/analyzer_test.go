package a12_timeout_config

import (
	"go/ast"
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
		"./tests/correctness/a12/positive",
		"./tests/correctness/a12/negative",
	)
}

func TestCheckDSN_Unit(t *testing.T) {
	cases := []struct {
		dsn         string
		wantMissing int
		wantZero    int
	}{
		{"postgres://user:pass@localhost:5432/db?statement_timeout=5000&lock_timeout=2000&idle_in_transaction_session_timeout=10000", 0, 0},
		{"postgres://user:pass@localhost:5432/db", 3, 0},
		{"postgres://user:pass@localhost:5432/db?statement_timeout=0&lock_timeout=0&idle_in_transaction_session_timeout=0", 0, 3},
		{"host=localhost port=5432 user=app dbname=app statement_timeout=5s lock_timeout=2s idle_in_transaction=10s", 0, 0},
	}

	for _, tc := range cases {
		res := CheckDSN(tc.dsn)
		if len(res.Missing) != tc.wantMissing {
			t.Errorf("for DSN %q, expected %d missing, got %d (%v)", tc.dsn, tc.wantMissing, len(res.Missing), res.Missing)
		}
		if len(res.Zero) != tc.wantZero {
			t.Errorf("for DSN %q, expected %d zero, got %d (%v)", tc.dsn, tc.wantZero, len(res.Zero), res.Zero)
		}
	}
}

func TestMeetStatus_Lattice(t *testing.T) {
	good := ConfigStatus{
		HasStatementTimeout:  true,
		HasLockTimeout:       true,
		HasIdleInTransaction: true,
		HasMaxConnIdleTime:   true,
		HasMaxConnLifetime:   true,
	}
	bad := ConfigStatus{
		HasStatementTimeout:  true,
		HasLockTimeout:       false,
		HasIdleInTransaction: true,
	}
	zero := ConfigStatus{
		HasStatementTimeout:  true,
		HasLockTimeout:       true,
		HasIdleInTransaction: true,
		HasMaxConnIdleTime:   true,
		HasMaxConnLifetime:   true,
		HasZeroTimeout:       true,
		ZeroTimeoutParam:     "statement_timeout",
	}

	// 1. Meet of good and bad must fail-closed (lacks lock_timeout, maxconn)
	m1 := meetStatus(good, bad)
	if m1.HasLockTimeout || m1.HasMaxConnIdleTime || m1.HasMaxConnLifetime {
		t.Errorf("meetStatus failed: expected missing timeouts when joined with incomplete path, got %+v", m1)
	}
	if !m1.HasStatementTimeout || !m1.HasIdleInTransaction {
		t.Errorf("meetStatus failed: expected shared timeouts preserved, got %+v", m1)
	}

	// 2. Meet of good and zero must fail-closed with zero timeout
	m2 := meetStatus(good, zero)
	if !m2.HasZeroTimeout || m2.ZeroTimeoutParam != "statement_timeout" {
		t.Errorf("meetStatus failed: expected zero timeout inherited, got %+v", m2)
	}

	// 3. Meet of good and good must stay compliant
	m3 := meetStatus(good, good)
	if !m3.HasStatementTimeout || !m3.HasLockTimeout || !m3.HasIdleInTransaction || !m3.HasMaxConnIdleTime || !m3.HasMaxConnLifetime || m3.HasZeroTimeout {
		t.Errorf("meetStatus failed: expected fully compliant status, got %+v", m3)
	}
}

func TestIsTerminating_Unit(t *testing.T) {
	retStmt := &ast.ReturnStmt{}
	if !isTerminating(retStmt) {
		t.Errorf("expected return statement to be terminating")
	}

	blockWithRet := &ast.BlockStmt{
		List: []ast.Stmt{
			&ast.ExprStmt{X: &ast.Ident{Name: "noop"}},
			&ast.ReturnStmt{},
		},
	}
	if !isTerminating(blockWithRet) {
		t.Errorf("expected block ending in return to be terminating")
	}

	normalBlock := &ast.BlockStmt{
		List: []ast.Stmt{
			&ast.ExprStmt{X: &ast.Ident{Name: "noop"}},
		},
	}
	if isTerminating(normalBlock) {
		t.Errorf("expected normal block not to be terminating")
	}
}
