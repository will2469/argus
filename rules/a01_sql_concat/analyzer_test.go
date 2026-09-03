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
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, rootDir, Analyzer, "./tests/correctness/a01/positive", "./tests/correctness/a01/negative")
}

func TestInspectFile_StandaloneParity(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "tests", "correctness", "a01", "positive", "positive.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, fixturePath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed parsing fixture: %v", err)
	}

	dm := directives.ParseGoDirectives(file, fset)
	issues := InspectFile(nil, fset, file, dm)

	expectedLines := map[int]bool{
		17: true,
		24: true,
		34: true,
		41: true,
		50: true,
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

func TestInspectFile_NegativeCorpus(t *testing.T) {
	src := `package testpkg

type Logger struct{}
func (Logger) Exec(msg string) {}

type HTTPClient struct{}
func (HTTPClient) Exec(cmd string) {}

type Queue struct{}
func (Queue) Query(topic string) {}

type SearchEngine struct{}
func (SearchEngine) Query(q string) string { return q }

func TestCompliant(logger Logger, client HTTPClient, q Queue, search SearchEngine, param string) {
	logger.Exec("Starting process: " + param)
	client.Exec("curl " + param)
	q.Query("job:" + param)
	search.Query("search: " + param)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	dm := directives.ParseGoDirectives(file, fset)
	issues := InspectFile(nil, fset, file, dm)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for non-DB negative corpus, got %d: %+v", len(issues), issues)
	}
}
