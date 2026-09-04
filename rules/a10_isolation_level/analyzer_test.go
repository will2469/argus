package a10_isolation_level

import (
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
