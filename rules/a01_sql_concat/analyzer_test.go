package a01_sql_concat

import (
	"go/ast"
	"go/parser"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func parseExpr(t *testing.T, exprStr string) ast.Expr {
	e, err := parser.ParseExpr(exprStr)
	if err != nil {
		t.Fatalf("failed parsing expr %q: %v", exprStr, err)
	}
	return e
}

func TestIsUnsafeSQL_Literals(t *testing.T) {
	safeLit := parseExpr(t, `"SELECT id FROM users WHERE id = $1"`)
	if isUnsafeSQL(safeLit, nil) {
		t.Errorf("expected string literal to be safe")
	}

	compileTimeConcat := parseExpr(t, `"SELECT id " + "FROM users"`)
	if isUnsafeSQL(compileTimeConcat, nil) {
		t.Errorf("expected compile-time string literal concatenation to be safe")
	}
}

func TestIsUnsafeSQL_RuntimeConcat(t *testing.T) {
	runtimeConcat := parseExpr(t, `"SELECT id FROM " + tableName`)
	if !isUnsafeSQL(runtimeConcat, nil) {
		t.Errorf("expected runtime concatenation to be flagged as unsafe")
	}
}

func TestIsUnsafeSQL_FmtSprintf(t *testing.T) {
	sprintfCall := parseExpr(t, `fmt.Sprintf("SELECT * FROM %s", tableName)`)
	if !isUnsafeSQL(sprintfCall, nil) {
		t.Errorf("expected fmt.Sprintf to be flagged as unsafe")
	}
}

func TestIsUnsafeSQL_Sanitizer(t *testing.T) {
	sanitizedCall := parseExpr(t, `SanitizeIdentifier(tableName)`)
	if isUnsafeSQL(sanitizedCall, nil) {
		t.Errorf("expected SanitizeIdentifier call to be recognized as safe")
	}
}

func TestAnalyzer(t *testing.T) {
	testdata, err := filepath.Abs("../../testdata")
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, testdata, Analyzer, "a01")
}
