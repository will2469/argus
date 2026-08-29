package runner

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/will2469/argus/rules/a14_select_star"
	"github.com/will2469/argus/rules/a17_nplusone"
	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/directives"
)

var (
	sqlUnsafeConcatRegex = regexp.MustCompile(`(?i)(?:["` + "`" + `]\s*(?:SELECT|INSERT\s+INTO|UPDATE|DELETE\s+FROM)\b[^"` + "`" + `]*["` + "`" + `]\s*\+\s*(?:req\.|input\.|params\.|r\.URL|c\.Query|body\.))`)
)

func scanGoSourceFile(filePath, rootDir string, tracker *MetricsTracker) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return
	}

	relPath, err := filepath.Rel(rootDir, filePath)
	if err != nil {
		relPath = filePath
	}

	dm := directives.ParseGoDirectives(node, fset)

	// 1. Inspect AST for queries and string literals (A01, A14)
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			methodName := callsite.GetCallMethodName(x.Fun)
			if callsite.IsDBQueryMethod(methodName) {
				tracker.IncrementQuerySites(1)
			}
		case *ast.BasicLit:
			if x.Kind == token.STRING {
				val := unquoteStringLit(x.Value)
				params := CountParameters(val)
				if params > 0 {
					tracker.IncrementParameterizedSites(params)
				}
				// ARGUS-A14: Forbidden SELECT *
				if a14_select_star.HasForbiddenSelectStar(val) {
					if !dm.IsIgnored(fset, x.Pos(), "ARGUS-A14") {
						pos := fset.Position(x.Pos())
						tracker.AddIssue(Issue{
							File:     relPath,
							Line:     pos.Line,
							Rule:     "FORBIDDEN_SELECT_STAR",
							Message:  "Forbidden 'SELECT *' or wildcard column selection detected. Explicitly list all required columns to minimize DB memory overhead and network payload.",
							Snippet:  strings.TrimSpace(val),
							Category: "performance",
						})
					}
				}
				// ARGUS-A01: Unsafe dynamic SQL concatenation
				if sqlUnsafeConcatRegex.MatchString(val) {
					if !dm.IsIgnored(fset, x.Pos(), "ARGUS-A01") {
						pos := fset.Position(x.Pos())
						tracker.AddIssue(Issue{
							File:     relPath,
							Line:     pos.Line,
							Rule:     "UNSAFE_SQL_CONCATENATION",
							Message:  "Unsafe dynamic SQL string concatenation of request/input detected. All query inputs must use parameterized placeholders ($1, $2).",
							Snippet:  strings.TrimSpace(val),
							Category: "security",
						})
					}
				}
			}
		}
		return true
	})

	// 2. ARGUS-A17: Deep Loop Walker & Helper Call Graph Analysis
	detector := a17_nplusone.NewHelperQueryDetector(nil, node)
	loopIssues := a17_nplusone.WalkLoops(nil, fset, node, dm, detector)
	for _, issue := range loopIssues {
		pos := fset.Position(issue.Pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     pos.Line,
			Rule:     "FORBIDDEN_QUERY_IN_LOOP",
			Message:  issue.Message,
			Category: "performance",
		})
	}
}
