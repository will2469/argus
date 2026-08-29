// Package a05_audit_immutability enforces that audit log tables remain append-only
// by forbidding UPDATE, DELETE, TRUNCATE, MERGE, and DROP operations.
package a05_audit_immutability

import (
	"go/ast"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

const RuleCode = "ARGUS-A05"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A05.
var Analyzer = &analysis.Analyzer{
	Name: "a05",
	Doc:  "Enforces strict append-only immutability on audit log tables (prohibiting UPDATE, DELETE, TRUNCATE, MERGE, DROP)",
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

	auditTablesMap := make(map[string]bool)
	auditTables := cfg.GetStringSlice(RuleCode, "audit_tables", []string{"audit_logs", "security_events"})
	for _, t := range auditTables {
		auditTablesMap[strings.ToLower(t)] = true
	}

	// 1. Inspect Go source files
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !callsite.IsDBQueryMethod(sel.Sel.Name) {
				return true
			}

			query, found := callsite.ExtractQueryString(call)
			if !found || strings.TrimSpace(query) == "" {
				return true
			}

			if dm.IsIgnored(pass.Fset, call.Pos(), RuleCode) {
				return true
			}

			op, violatedTable := CheckSQLTampering(query, auditTablesMap)
			if op != "" {
				pass.Reportf(call.Pos(), "[%s] forbidden %s on audit table %q; audit trails must be strictly append-only", RuleCode, op, violatedTable)
			}
			return true
		})
	}

	// 2. Inspect migrations if current package directory hosts migrations
	if len(pass.Files) > 0 {
		pkgDir := filepath.Dir(pass.Fset.Position(pass.Files[0].Pos()).Filename)
		migDir := cfg.FindMatchingMigrationDir(pkgDir)
		if migDir != "" {
			issues := InspectMigrationDir(migDir, dm, auditTablesMap)
			for _, is := range issues {
				pass.Reportf(pass.Files[0].Pos(), "[%s] %s:%d: %s", RuleCode, filepath.Base(is.Filename), is.Line, is.Message)
			}
		}
	}

	return nil, nil
}
