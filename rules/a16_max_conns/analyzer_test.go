package a16_max_conns

import (
	"go/ast"
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

func TestEvaluateExpr_LargeArithmeticOverflow(t *testing.T) {
	expr, err := parser.ParseExpr("2000000000 * 2")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	eval := EvaluateExpr(expr, DefaultMaxSafeConns)
	if eval.Valid {
		t.Errorf("expected 2000000000 * 2 to be flagged as invalid exceeding maxSafe, got valid=true")
	}
	if !strings.Contains(eval.Message, "4000000000") || !strings.Contains(eval.Message, "exceeds safe direct connection limit") {
		t.Errorf("expected message to mention '4000000000' and 'exceeds', got %q", eval.Message)
	}
}

func TestStrictSemanticMatcher(t *testing.T) {
	src := `package main
type WorkerPool struct{}
func (WorkerPool) New(ctx any, s string) {}
var pgxpool WorkerPool
func Run() {
	pgxpool.New(nil, "postgres://bad/db")
}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	issues := InspectFile(nil, fset, file, nil, 100)
	if len(issues) != 0 {
		t.Errorf("expected 0 issues for unrelated WorkerPool variable named pgxpool, got %d", len(issues))
	}
}

func TestMatchesVar_StrictTargetObj(t *testing.T) {
	objA := &ast.Object{Name: "dsn"}
	idWithObj := &ast.Ident{Name: "dsn", Obj: objA}
	idWithoutObj := &ast.Ident{Name: "dsn", Obj: nil}
	idDifferentObj := &ast.Ident{Name: "dsn", Obj: &ast.Object{Name: "dsn"}}

	if !matchesVar(idWithObj, "dsn", objA) {
		t.Errorf("expected matching object to return true")
	}
	if matchesVar(idWithoutObj, "dsn", objA) {
		t.Errorf("expected id without obj to NOT match when targetObj is set")
	}
	if matchesVar(idDifferentObj, "dsn", objA) {
		t.Errorf("expected different obj to NOT match when targetObj is set")
	}
	if !matchesVar(idWithoutObj, "dsn", nil) {
		t.Errorf("expected name-based fallback when targetObj is nil")
	}
}

func TestDSNReachingDefinitions_DeepAliasChain(t *testing.T) {
	src := `package main
import "context"
type pgxpoolPkg struct{}
var pgxpool pgxpoolPkg
func (pgxpoolPkg) New(ctx context.Context, dsn string) {}
func Run(ctx context.Context) {
	a0 := "postgres://bad/db"
	a1 := a0
	a2 := a1
	a3 := a2
	a4 := a3
	a5 := a4
	a6 := a5
	a7 := a6
	a8 := a7
	a9 := a8
	a10 := a9
	a11 := a10
	a12 := a11
	a13 := a12
	a14 := a13
	a15 := a14
	pgxpool.New(ctx, a15)
}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	issues := InspectFile(nil, fset, file, nil, 100)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue when 15-hop alias chain propagates unsafe DSN, got %d", len(issues))
	}
}


