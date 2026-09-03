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
	tests := []struct {
		sql        string
		isWrite    bool
		hasRowLock bool
	}{
		{"SELECT * FROM balances WHERE id = 1 FOR UPDATE", false, true},
		{"SELECT * FROM balances WHERE id = 1 FOR NO KEY UPDATE", false, true},
		{"SELECT * FROM balances WHERE id = 1", false, false},
		{"UPDATE balances SET amount = 100", true, false},
		{"INSERT INTO inventory (id) VALUES (1)", true, false},
		{"DELETE FROM accounts WHERE id = 1", true, false},
		{"UPDATE user_preferences SET theme = 'dark'", false, false},
	}

	for _, tc := range tests {
		gotWrite := IsCriticalTableWrite(tc.sql, nil)
		if gotWrite != tc.isWrite {
			t.Errorf("for %q, expected isWrite=%v, got=%v", tc.sql, tc.isWrite, gotWrite)
		}
		gotLock := HasPessimisticRowLock(tc.sql)
		if gotLock != tc.hasRowLock {
			t.Errorf("for %q, expected hasRowLock=%v, got=%v", tc.sql, tc.hasRowLock, gotLock)
		}
	}
}
