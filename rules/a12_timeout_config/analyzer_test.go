package a12_timeout_config

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
