package runner

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/rules/a01_sql_concat"
	"github.com/will2469/argus/rules/a02_unclosed_rows"
	"github.com/will2469/argus/rules/a03_context"
	"github.com/will2469/argus/rules/a04_orderby"
	"github.com/will2469/argus/rules/a05_audit_immutability"
	"github.com/will2469/argus/rules/a06_runtime_ddl"
	"github.com/will2469/argus/rules/a07_error_leak"
	"github.com/will2469/argus/rules/a08_tx_io"
	"github.com/will2469/argus/rules/a09_advisory_lock"
	"github.com/will2469/argus/rules/a10_isolation_level"
	"github.com/will2469/argus/rules/a12_timeout_config"
	"github.com/will2469/argus/rules/a14_select_star"
	"github.com/will2469/argus/rules/a16_max_conns"
	"github.com/will2469/argus/rules/a17_nplusone"
	"github.com/will2469/argus/rules/a18_rows_err"
	"github.com/will2469/argus/rules/a19_unbounded_limit"
	"github.com/will2469/argus/rules/a20_param_limit"
	"github.com/will2469/argus/rules/a21_row_lock"
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

	// 3. ARGUS-A02: Unclosed rows & missing defer rows.Close()
	a02Issues := a02_unclosed_rows.InspectFile(pass, fset, node, dm)
	for _, issue := range a02Issues {
		pos := fset.Position(issue.Pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     pos.Line,
			Rule:     "MISSING_DEFER_CLOSE",
			Message:  issue.Message,
			Category: "reliability",
		})
	}

	// 4. ARGUS-A03: Unbounded context in database operations
	a03Issues := a03_context.InspectFile(pass, fset, node, dm)
	for _, issue := range a03Issues {
		pos := fset.Position(issue.Pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     pos.Line,
			Rule:     "UNBOUNDED_CONTEXT",
			Message:  issue.Message,
			Category: "reliability",
		})
	}

	// 5. ARGUS-A04: Unsafe dynamic ORDER BY clauses
	a04Issues := a04_orderby.InspectFile(pass, fset, node, dm)
	for _, issue := range a04Issues {
		pos := fset.Position(issue.Pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     pos.Line,
			Rule:     "UNSAFE_DYNAMIC_ORDERBY",
			Message:  issue.Message,
			Category: "security",
		})
	}

	// 6. ARGUS-A05: Audit Table Immutability
	a05Issues := a05_audit_immutability.InspectFile(pass, fset, node, dm, nil)
	for _, issue := range a05Issues {
		pos := fset.Position(issue.Pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     pos.Line,
			Rule:     "FORBIDDEN_AUDIT_MUTATION",
			Message:  issue.Message,
			Category: "security",
		})
	}

	// 7. ARGUS-A06: Runtime DDL Execution
	a06Issues := a06_runtime_ddl.InspectFile(pass, fset, node, dm)
	for _, issue := range a06Issues {
		pos := fset.Position(issue.Pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     pos.Line,
			Rule:     "RUNTIME_DDL_EXECUTION",
			Message:  issue.Message,
			Category: "reliability",
		})
	}

	// 8. ARGUS-A07: Database Error Leakage in Responses
	a07Issues := a07_error_leak.InspectFile(pass, fset, node, dm)
	for _, issue := range a07Issues {
		pos := fset.Position(issue.Pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     pos.Line,
			Rule:     "DATABASE_ERROR_LEAK",
			Message:  issue.Message,
			Category: "security",
		})
	}

	// 9. ARGUS-A08: Blocking I/O in Database Transactions
	a08Issues := a08_tx_io.InspectFile(pass, fset, node, dm)
	for _, issue := range a08Issues {
		pos := fset.Position(issue.Pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     pos.Line,
			Rule:     "TRANSACTION_BLOCKING_IO",
			Message:  issue.Message,
			Category: "performance",
		})
	}

	// 10. ARGUS-A09: Advisory Lock Scope & Namespace Hygiene
	a09Issues := a09_advisory_lock.InspectFile(pass, fset, node, dm)
	for _, issue := range a09Issues {
		pos := fset.Position(issue.Pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     pos.Line,
			Rule:     "UNSAFE_ADVISORY_LOCK",
			Message:  issue.Message,
			Category: "reliability",
		})
	}

	// 11. ARGUS-A10: Critical Table Transaction Isolation Level
	a10Issues := a10_isolation_level.InspectFile(pass, fset, node, dm, nil)
	for _, issue := range a10Issues {
		pos := fset.Position(issue.Pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     pos.Line,
			Rule:     "WEAK_ISOLATION_LEVEL",
			Message:  issue.Message,
			Category: "reliability",
		})
	}

	// 12. ARGUS-A12: Connection Pool Timeout Configuration
	a12Issues := a12_timeout_config.InspectFile(pass, fset, node, dm)
	for _, issue := range a12Issues {
		pos := fset.Position(issue.Pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     pos.Line,
			Rule:     "TIMEOUT_CONFIG_MISSING",
			Message:  issue.Message,
			Category: "reliability",
		})
	}

	// 13. ARGUS-A14: Forbidden SELECT *
	a14Issues := a14_select_star.InspectFile(pass, fset, node, dm)
	for _, issue := range a14Issues {
		pos := fset.Position(issue.Pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     pos.Line,
			Rule:     "FORBIDDEN_SELECT_STAR",
			Message:  issue.Message,
			Category: "performance",
		})
	}

	// 14. ARGUS-A16: Pool Max Connections Boundary
	a16Issues := a16_max_conns.InspectFile(pass, fset, node, dm, 0)
	for _, issue := range a16Issues {
		pos := fset.Position(issue.Pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     pos.Line,
			Rule:     "UNBOUNDED_MAX_CONNS",
			Message:  issue.Message,
			Category: "reliability",
		})
	}

	// 15. ARGUS-A17: Deep Loop Walker & Helper Call Graph Analysis
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

	// 16. ARGUS-A18: Mandatory rows.Err() Check After Cursor Loop
	a18Issues := a18_rows_err.InspectFile(pass, fset, node, dm)
	for _, issue := range a18Issues {
		pos := fset.Position(issue.Pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     pos.Line,
			Rule:     "MISSING_ROWS_ERR_CHECK",
			Message:  issue.Message,
			Category: "reliability",
		})
	}

	// 17. ARGUS-A19: Unbounded Query on High-Cardinality Table
	tableMap := a19_unbounded_limit.GetHighCardinalityTables(nil)
	keyColumnMap := a19_unbounded_limit.GetKeyColumns(nil)
	a19_unbounded_limit.InspectFile(node, fset, dm, tableMap, keyColumnMap, func(pos token.Pos, format string, args ...any) {
		p := fset.Position(pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     p.Line,
			Rule:     "UNBOUNDED_HIGH_CARDINALITY_QUERY",
			Message:  fmt.Sprintf(format, args...),
			Category: "performance",
		})
	})

	// 18. ARGUS-A20: Wire Protocol Parameter Limit (65,535)
	a20_param_limit.InspectFile(node, fset, dm, func(pos token.Pos, format string, args ...any) {
		p := fset.Position(pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     p.Line,
			Rule:     "UNBOUNDED_BATCH_PARAMS",
			Message:  fmt.Sprintf(format, args...),
			Category: "reliability",
		})
	})

	// 19. ARGUS-A21: Row Lock Convoys (FOR UPDATE without SKIP LOCKED / NOWAIT)
	keyColumnMapA21 := a21_row_lock.GetKeyColumns(nil)
	a21_row_lock.InspectFile(node, fset, dm, keyColumnMapA21, func(pos token.Pos, format string, args ...any) {
		p := fset.Position(pos)
		tracker.AddIssue(Issue{
			File:     relPath,
			Line:     p.Line,
			Rule:     "BLOCKING_ROW_LOCK",
			Message:  fmt.Sprintf(format, args...),
			Category: "performance",
		})
	})

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
