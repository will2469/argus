// Package a21_row_lock enforces non-blocking directives (SKIP LOCKED / NOWAIT) on multi-row
// and task queue row locks to prevent lock convoys and serialization bottlenecks.
package a21_row_lock

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

// RuleCode is the official identifier for ARGUS-A21.
const RuleCode = "ARGUS-A21"

// Analyzer defines the analysis.Analyzer for ARGUS-A21.
var Analyzer = &analysis.Analyzer{
	Name: "argus_a21_unbounded_row_lock_blocking",
	Doc:  "Enforce SKIP LOCKED or NOWAIT on multi-row FOR UPDATE queue polling queries to prevent lock convoys and deadlocks (CWE-662, CWE-833)",
	Run:  run,
	Requires: []*analysis.Analyzer{
		directives.Analyzer,
		config.Analyzer,
	},
}

func run(pass *analysis.Pass) (interface{}, error) {
	cfg := pass.ResultOf[config.Analyzer].(*config.Config)
	if !cfg.IsRuleEnabled(RuleCode) {
		return nil, nil
	}

	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)
	keyColumnMap := GetKeyColumns(cfg)

	for _, file := range pass.Files {
		pos := pass.Fset.Position(file.Package)
		if strings.HasSuffix(pos.Filename, "_test.go") {
			continue
		}

		InspectFile(file, pass.Fset, dm, keyColumnMap, func(pos token.Pos, format string, args ...any) {
			pass.Reportf(pos, format, args...)
		})
	}

	return nil, nil
}

// InspectFile walks an AST file and reports violations of ARGUS-A21.
func InspectFile(file *ast.File, fset *token.FileSet, dm *directives.DirectiveMap, keyColumnMap map[string]bool, report func(pos token.Pos, format string, args ...any)) {
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		methodName := callsite.GetCallMethodName(call.Fun)
		if !callsite.IsDBQueryMethod(methodName) {
			return true
		}

		query, found := callsite.ExtractQueryString(call)
		if !found || strings.TrimSpace(query) == "" {
			return true
		}

		if violating, reason := CheckLockingQuery(query, keyColumnMap); violating {
			if dm != nil && (dm.IsIgnored(fset, call.Pos(), RuleCode) || dm.IsIgnored(fset, call.Pos(), RuleCode+".ROW-LOCK")) {
				return true
			}
			report(call.Pos(), "[%s] %s", RuleCode, reason)
		}

		return true
	})
}
