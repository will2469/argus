package runner

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/rules/a01_sql_concat"
	"github.com/will2469/argus/rules/a14_select_star"
	"github.com/will2469/argus/rules/a17_nplusone"
	"github.com/will2469/argus/rules/a24_tenant_leak"
	"github.com/will2469/argus/rules/a26_like_sanitize"
	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/directives"
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

	// 1. Inspect AST for queries and string literals (A14)
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
			}
		}
		return true
	})

	// Construct lightweight analysis.Pass with type information for standalone mode
	typesInfo := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	conf := types.Config{
		Error: func(err error) {}, // ignore unresolved external package errors
	}
	_, _ = conf.Check(node.Name.Name, fset, []*ast.File{node}, typesInfo)
	pass := &analysis.Pass{
		Fset:      fset,
		Files:     []*ast.File{node},
		TypesInfo: typesInfo,
	}

	// 2. ARGUS-A01: SQL concatenation & unsafe formatting
	a01Issues := a01_sql_concat.InspectFile(pass, fset, node, dm)
	for _, issue := range a01Issues {
		pos := fset.Position(issue.Pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     pos.Line,
			Rule:     "UNSAFE_SQL_CONCATENATION",
			Message:  issue.Message,
			Category: "security",
		})
	}

	// 3. ARGUS-A17: Deep Loop Walker & Helper Call Graph Analysis
	detector := a17_nplusone.NewHelperQueryDetector(pass, node)
	loopIssues := a17_nplusone.WalkLoops(pass, fset, node, dm, detector)
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

	// 4. ARGUS-A26: Unsanitized LIKE/ILIKE wildcard input
	a26Issues := a26_like_sanitize.InspectFile(pass, fset, node, dm)
	for _, issue := range a26Issues {
		pos := fset.Position(issue.Pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     pos.Line,
			Rule:     "LIKE_WILDCARD_INJECTION",
			Message:  issue.Message,
			Category: "security",
		})
	}

	// 5. ARGUS-A24: Multi-tenant table isolation leak (anti-BOLA)
	a24Issues := a24_tenant_leak.InspectFile(pass, fset, node, dm, nil)
	for _, issue := range a24Issues {
		pos := fset.Position(issue.Pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     pos.Line,
			Rule:     "TENANT_ISOLATION_LEAK",
			Message:  issue.Message,
			Category: "security",
		})
	}
}
