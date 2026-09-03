package a01_sql_concat

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/will2469/argus/shared/directives"
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

func TestInspectFile_StandaloneParity(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "src", "a01", "a01.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, fixturePath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed parsing fixture: %v", err)
	}

	dm := directives.ParseGoDirectives(file, fset)
	issues := InspectFile(nil, fset, file, dm)

	expectedLines := map[int]bool{
		53: true,
		59: true,
		63: true,
		71: true,
	}

	for _, issue := range issues {
		t.Logf("issue at line %d (pos %d): %s", fset.Position(issue.Pos).Line, issue.Pos, issue.Message)
	}

	if len(issues) != len(expectedLines) {
		t.Fatalf("expected %d issues, got %d: %+v", len(expectedLines), len(issues), issues)
	}

	for _, issue := range issues {
		line := fset.Position(issue.Pos).Line
		if !expectedLines[line] {
			t.Errorf("unexpected issue reported at line %d: %s", line, issue.Message)
		}
	}
}
