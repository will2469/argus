package a16_max_conns

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	testdata, err := filepath.Abs("../../testdata")
	if err != nil {
		t.Fatalf("failed to resolve testdata path: %v", err)
	}

	analysistest.Run(t, testdata, Analyzer, "a16")
}

func TestEvaluateDSN(t *testing.T) {
	cases := []struct {
		name       string
		dsn        string
		configured bool
		valid      bool
	}{
		{"Missing", "postgres://localhost:5432/db", false, false},
		{"ValidSmall", "postgres://localhost:5432/db?pool_max_conns=20", true, true},
		{"TooLarge", "postgres://localhost:5432/db?pool_max_conns=500", true, false},
		{"Zero", "postgres://localhost:5432/db?pool_max_conns=0", true, false},
		{"KVValid", "host=localhost dbname=db pool_max_conns=15", true, true},
	}

	for _, tc := range cases {
		eval := EvaluateDSN(tc.dsn, DefaultMaxSafeConns)
		if eval.Configured != tc.configured || eval.Valid != tc.valid {
			t.Errorf("[%s] expected configured=%v, valid=%v; got configured=%v, valid=%v",
				tc.name, tc.configured, tc.valid, eval.Configured, eval.Valid)
		}
	}
}
