package a16_max_conns

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("failed to resolve rootDir: %v", err)
	}

	analysistest.Run(t, rootDir, Analyzer,
		"./tests/correctness/a16/positive",
		"./tests/correctness/a16/negative",
	)
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

func TestDynamicThresholdFormatting(t *testing.T) {
	customMax := int32(50)
	evalDSN := EvaluateDSN("postgres://localhost:5432/db?pool_max_conns=75", customMax)
	if evalDSN.Valid {
		t.Errorf("expected DSN with 75 conns to exceed custom max 50")
	}
	if !strings.Contains(evalDSN.Message, "threshold (50)") {
		t.Errorf("expected DSN message to mention 'threshold (50)', got %q", evalDSN.Message)
	}

	expr, err := parser.ParseExpr("75")
	if err != nil {
		t.Fatalf("failed to parse expr: %v", err)
	}
	evalExpr := EvaluateExpr(expr, customMax)
	if evalExpr.Valid {
		t.Errorf("expected Expr with 75 conns to exceed custom max 50")
	}
	if !strings.Contains(evalExpr.Message, "limit (50)") {
		t.Errorf("expected Expr message to mention 'limit (50)', got %q", evalExpr.Message)
	}
}

func TestEvaluateExpr_Arithmetic(t *testing.T) {
	fset := token.NewFileSet()
	_ = fset

	exprMul, err := parser.ParseExpr("10 * 2")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	eval := EvaluateExpr(exprMul, DefaultMaxSafeConns)
	if !eval.Valid || eval.Value != 20 {
		t.Errorf("expected 10 * 2 to be valid with value 20, got valid=%v, val=%d", eval.Valid, eval.Value)
	}

	exprNeg, err := parser.ParseExpr("-10")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	evalNeg := EvaluateExpr(exprNeg, DefaultMaxSafeConns)
	if evalNeg.Valid {
		t.Errorf("expected -10 to be invalid")
	}
}
