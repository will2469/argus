package runner

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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
	"github.com/will2469/argus/rules/a22_serializable_retry"
	"github.com/will2469/argus/rules/a23_tx_timeout"
	"github.com/will2469/argus/rules/a24_tenant_leak"
	"github.com/will2469/argus/rules/a25_expensive_cpu"
	"github.com/will2469/argus/rules/a26_like_sanitize"
	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

func scanGoSourceFile(filePath, rootDir string, tracker *MetricsTracker, appCfg *config.Config, fsys fs.FS) {
	fset := token.NewFileSet()
	var src any
	if fsys != nil {
		cleanPath := filepath.ToSlash(filepath.Clean(filePath))
		data, err := fs.ReadFile(fsys, cleanPath)
		if err != nil {
			return
		}
		src = data
	}
	node, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
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

	// Load package sibling files in the same directory for complete type checking and call graph propagation
	pkgFiles := []*ast.File{node}
	dir := filepath.Dir(filePath)
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			siblingPath := filepath.Join(dir, e.Name())
			if siblingPath == filePath {
				continue
			}
			if siblingNode, err := parser.ParseFile(fset, siblingPath, nil, 0); err == nil {
				if siblingNode.Name.Name == node.Name.Name {
					pkgFiles = append(pkgFiles, siblingNode)
				}
			}
		}
	}

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
	_, _ = conf.Check(node.Name.Name, fset, pkgFiles, typesInfo)
	pass := &analysis.Pass{
		Fset:      fset,
		Files:     pkgFiles,
		TypesInfo: typesInfo,
		ResultOf: map[*analysis.Analyzer]any{
			config.Analyzer:     appCfg,
			directives.Analyzer: dm,
		},
	}

	// 2. ARGUS-A01: SQL concatenation & unsafe formatting
	if isRuleActive(appCfg, a01_sql_concat.RuleCode) {
		var customSources []string
		if appCfg != nil {
			customSources = appCfg.GetStringSlice(a01_sql_concat.RuleCode, "custom_taint_sources", nil)
		}
		a01Issues := a01_sql_concat.InspectFileWithConfig(pass, fset, node, dm, customSources)
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
	}

	// 3. ARGUS-A02: Unclosed rows & missing defer rows.Close()
	if isRuleActive(appCfg, a02_unclosed_rows.RuleCode) {
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
	}

	// 4. ARGUS-A03: Unbounded context in database operations
	if isRuleActive(appCfg, a03_context.RuleCode) {
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
	}

	// 5. ARGUS-A04: Unsafe dynamic ORDER BY clauses
	if isRuleActive(appCfg, a04_orderby.RuleCode) {
		var allowedCols []string
		if appCfg != nil {
			allowedCols = appCfg.GetStringSlice(a04_orderby.RuleCode, "allowed_columns", nil)
		}
		a04Issues := a04_orderby.InspectFileWithConfig(pass, fset, node, dm, allowedCols)
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
	}

	// 6. ARGUS-A05: Audit Table Immutability
	if isRuleActive(appCfg, a05_audit_immutability.RuleCode) {
		var auditTablesMap map[string]bool
		if appCfg != nil {
			tables := appCfg.GetStringSlice(a05_audit_immutability.RuleCode, "audit_tables", []string{"audit_logs", "security_events"})
			auditTablesMap = make(map[string]bool)
			for _, t := range tables {
				auditTablesMap[strings.ToLower(strings.TrimSpace(t))] = true
			}
		}
		a05Issues := a05_audit_immutability.InspectFile(pass, fset, node, dm, auditTablesMap)
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
	}

	// 7. ARGUS-A06: Runtime DDL Execution
	if isRuleActive(appCfg, a06_runtime_ddl.RuleCode) {
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
	}

	// 8. ARGUS-A07: Database Error Leakage in Responses
	if isRuleActive(appCfg, a07_error_leak.RuleCode) {
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
	}

	// 9. ARGUS-A08: Blocking I/O in Database Transactions
	if isRuleActive(appCfg, a08_tx_io.RuleCode) {
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
	}

	// 10. ARGUS-A09: Advisory Lock Scope & Namespace Hygiene
	if isRuleActive(appCfg, a09_advisory_lock.RuleCode) {
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
	}

	// 11. ARGUS-A10: Critical Table Transaction Isolation Level
	if isRuleActive(appCfg, a10_isolation_level.RuleCode) {
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
	}

	// 12. ARGUS-A12: Connection Pool Timeout Configuration
	if isRuleActive(appCfg, a12_timeout_config.RuleCode) {
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
	}

	// 13. ARGUS-A14: Forbidden SELECT *
	if isRuleActive(appCfg, a14_select_star.RuleCode) {
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
	}

	// 14. ARGUS-A16: Pool Max Connections Boundary
	if isRuleActive(appCfg, a16_max_conns.RuleCode) {
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
	}

	// 15. ARGUS-A17: Deep Loop Walker & Helper Call Graph Analysis
	if isRuleActive(appCfg, a17_nplusone.RuleCode) {
		detector := a17_nplusone.NewHelperQueryDetector(pass, pkgFiles...)
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
	}

	// 16. ARGUS-A18: Mandatory rows.Err() Check After Cursor Loop
	if isRuleActive(appCfg, a18_rows_err.RuleCode) {
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
	}

	// 17. ARGUS-A19: Unbounded Query on High-Cardinality Table
	if isRuleActive(appCfg, a19_unbounded_limit.RuleCode) {
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
	}

	// 18. ARGUS-A20: Wire Protocol Parameter Limit (65,535)
	if isRuleActive(appCfg, a20_param_limit.RuleCode) {
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
	}

	// 19. ARGUS-A21: Row Lock Convoys (FOR UPDATE without SKIP LOCKED / NOWAIT)
	if isRuleActive(appCfg, a21_row_lock.RuleCode) {
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
	}

	// 20. ARGUS-A22: Serializable / RepeatableRead Retry Loop
	if isRuleActive(appCfg, a22_serializable_retry.RuleCode) {
		a22_serializable_retry.InspectFile(node, fset, dm, func(pos token.Pos, format string, args ...any) {
			p := fset.Position(pos)
			tracker.AddIssue(Issue{
				File:     relPath,
				Line:     p.Line,
				Rule:     "MISSING_SERIALIZABLE_RETRY",
				Message:  fmt.Sprintf(format, args...),
				Category: "reliability",
			})
		})
	}

	// 21. ARGUS-A23: Transaction Timeout GUC Configuration
	if isRuleActive(appCfg, a23_tx_timeout.RuleCode) {
		a23_tx_timeout.InspectFile(node, fset, dm, func(pos token.Pos, format string, args ...any) {
			p := fset.Position(pos)
			tracker.AddIssue(Issue{
				File:     relPath,
				Line:     p.Line,
				Rule:     "MISSING_TX_TIMEOUT",
				Message:  fmt.Sprintf(format, args...),
				Category: "reliability",
			})
		})
	}

	// 22. ARGUS-A26: Unsanitized LIKE/ILIKE wildcard input
	if isRuleActive(appCfg, a26_like_sanitize.RuleCode) {
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
	}

	// 23. ARGUS-A24: Multi-tenant table isolation leak (anti-BOLA)
	if isRuleActive(appCfg, a24_tenant_leak.RuleCode) {
		tcA24 := a24_tenant_leak.LoadTenantConfig(appCfg)
		a24Issues := a24_tenant_leak.InspectFile(pass, fset, node, dm, tcA24)
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

	// 24. ARGUS-A25: Expensive CPU Computations in Active Database Transactions
	if isRuleActive(appCfg, a25_expensive_cpu.RuleCode) {
		a25_expensive_cpu.InspectFile(node, fset, dm, func(pos token.Pos, format string, args ...any) {
			p := fset.Position(pos)
			tracker.AddIssue(Issue{
				File:     relPath,
				Line:     p.Line,
				Rule:     "EXPENSIVE_CPU_IN_TX",
				Message:  fmt.Sprintf(format, args...),
				Category: "performance",
			})
		})
	}
}
