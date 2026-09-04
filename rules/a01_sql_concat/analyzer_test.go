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
	if isUnsafeSQL(safeLit, nil, nil, nil) {
		t.Errorf("expected string literal to be safe")
	}

	compileTimeConcat := parseExpr(t, `"SELECT id " + "FROM users"`)
	if isUnsafeSQL(compileTimeConcat, nil, nil, nil) {
		t.Errorf("expected compile-time string literal concatenation to be safe")
	}
}

func TestIsUnsafeSQL_RuntimeConcat(t *testing.T) {
	runtimeConcat := parseExpr(t, `"SELECT id FROM " + tableName`)
	if !isUnsafeSQL(runtimeConcat, nil, nil, nil) {
		t.Errorf("expected runtime concatenation to be flagged as unsafe")
	}
}

func TestIsUnsafeSQL_FmtSprintf(t *testing.T) {
	sprintfCall := parseExpr(t, `fmt.Sprintf("SELECT * FROM %s", tableName)`)
	if !isUnsafeSQL(sprintfCall, nil, nil, nil) {
		t.Errorf("expected fmt.Sprintf to be flagged as unsafe")
	}
}

func TestIsUnsafeSQL_Sanitizer(t *testing.T) {
	sanitizedCall := parseExpr(t, `SanitizeIdentifier(tableName)`)
	if isUnsafeSQL(sanitizedCall, nil, nil, nil) {
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

func TestIsUnsafeSQL_EvilSanitizerRejected(t *testing.T) {
	// Arbitrary struct method .Sanitize(...) must NOT be trusted
	evilCall := parseExpr(t, `"SELECT * FROM " + evil.Sanitize(userInput)`)
	if !isUnsafeSQL(evilCall, nil, nil, nil) {
		t.Errorf("expected evil.Sanitize to be flagged as unsafe")
	}

	// Arbitrary struct method .SanitizeIdentifier(...) must NOT be trusted unless trusted package
	evilIdentCall := parseExpr(t, `"SELECT * FROM " + evil.SanitizeIdentifier(userInput)`)
	if !isUnsafeSQL(evilIdentCall, nil, nil, nil) {
		t.Errorf("expected evil.SanitizeIdentifier to be flagged as unsafe")
	}
}

func TestIsUnsafeSQL_StringsJoin(t *testing.T) {
	safeJoin := parseExpr(t, `strings.Join([]string{"SELECT id", "FROM users", "WHERE active = true"}, " ")`)
	if isUnsafeSQL(safeJoin, nil, nil, nil) {
		t.Errorf("expected compile-time strings.Join to be safe")
	}

	unsafeJoin := parseExpr(t, `strings.Join([]string{"SELECT id", userInput}, " ")`)
	if !isUnsafeSQL(unsafeJoin, nil, nil, nil) {
		t.Errorf("expected dynamic strings.Join to be flagged as unsafe")
	}
}

func TestIsUnsafeSQL_SafeSprintf(t *testing.T) {
	safeConstSprintf := parseExpr(t, `fmt.Sprintf("SELECT %s FROM users", "id")`)
	if isUnsafeSQL(safeConstSprintf, nil, nil, nil) {
		t.Errorf("expected fmt.Sprintf with string literal arg to be safe")
	}

	safeSanitizedSprintf := parseExpr(t, `fmt.Sprintf("SELECT %s FROM %s", SanitizeIdentifier(col), "users")`)
	if isUnsafeSQL(safeSanitizedSprintf, nil, nil, nil) {
		t.Errorf("expected fmt.Sprintf with SanitizeIdentifier to be safe")
	}

	unsafeSprintf := parseExpr(t, `fmt.Sprintf("SELECT * FROM %s", userInput)`)
	if !isUnsafeSQL(unsafeSprintf, nil, nil, nil) {
		t.Errorf("expected fmt.Sprintf with runtime variable to be unsafe")
	}
}

func TestInspectFile_FlowSensitivity(t *testing.T) {
	src := `package testpkg
import "context"

type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

func TestReassignmentClean(ctx context.Context, db DB, userInput string) {
	q := userInput
	q = "SELECT * FROM users"
	_, _ = db.Exec(ctx, q) // clean override: must be safe!
}

func TestBranchTaint(ctx context.Context, db DB, userInput string, cond bool) {
	q := "SELECT * FROM users"
	if cond {
		q = userInput
	}
	_, _ = db.Exec(ctx, q) // conditional taint: must be flagged!
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "flow_test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 1 {
		t.Fatalf("expected exactly 1 issue for flow-sensitive branch taint, got %d", len(issues))
	}
	line := fset.Position(issues[0].Pos).Line
	if line != 19 {
		t.Errorf("expected issue on line 19 (TestBranchTaint), got line %d", line)
	}
}

func TestInspectFile_CustomTaintSources(t *testing.T) {
	src := `package testpkg
import "context"

type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

func TestCustomNIK(ctx context.Context, db DB, nik string) {
	q := nik
	_, _ = db.Exec(ctx, q)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "custom_test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Without custom_taint_sources: nik is not in default universal list
	if issues := InspectFile(nil, fset, file, nil); len(issues) != 0 {
		t.Errorf("expected 0 issues without custom sources, got %d", len(issues))
	}

	// With custom_taint_sources: nik is tracked as taint source
	if issues := InspectFileWithConfig(nil, fset, file, nil, []string{"nik"}); len(issues) != 1 {
		t.Fatalf("expected 1 issue with custom source nik, got %d", len(issues))
	}
}

