package a23_tx_timeout

import (
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
		"./tests/correctness/a23/positive",
		"./tests/correctness/a23/negative",
	)
}

func TestCheckDSN(t *testing.T) {
	cases := []struct {
		dsn          string
		hasTxTimeout bool
		isZero       bool
	}{
		{"postgres://user:pass@localhost:5432/db?transaction_timeout=30000", true, false},
		{"postgres://user:pass@localhost:5432/db?transaction_timeout=0", true, true},
		{"postgres://user:pass@localhost:5432/db?statement_timeout=10000", false, false},
		{"host=localhost transaction_timeout=30s", true, false},
		{"host=localhost transaction_timeout=0s", true, true},
		{"host=localhost port=5432", false, false},
	}

	for _, tc := range cases {
		hasTx, isZero := CheckDSN(tc.dsn)
		if hasTx != tc.hasTxTimeout || isZero != tc.isZero {
			t.Errorf("[%s] expected (hasTx=%v, isZero=%v), got (hasTx=%v, isZero=%v)", tc.dsn, tc.hasTxTimeout, tc.isZero, hasTx, isZero)
		}
	}
}
