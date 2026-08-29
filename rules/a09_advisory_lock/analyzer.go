// Package a09_advisory_lock ensures safe PostgreSQL advisory lock usage by prohibiting
// session-level locks on connection pools and forbidding hardcoded integer magic numbers.
package a09_advisory_lock

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

const RuleCode = "ARGUS-A09"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A09.
var Analyzer = &analysis.Analyzer{
	Name: "argus_a09_advisory_lock",
	Doc:  "Enforces transaction-scoped advisory locks and namespaced identifiers",
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

	for _, file := range pass.Files {
		pos := pass.Fset.Position(file.Package)
		if strings.HasSuffix(pos.Filename, "_test.go") {
			continue
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			checkDBCallAdvisoryLock(pass, call, dm)
			CheckAdvisoryHelperArgs(pass, call, dm)
			return true
		})
	}

	return nil, nil
}

func checkDBCallAdvisoryLock(pass *analysis.Pass, call *ast.CallExpr, dm *directives.DirectiveMap) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !callsite.IsDBQueryMethod(sel.Sel.Name) {
		return
	}

	query, found := callsite.ExtractQueryString(call)
	if !found || strings.TrimSpace(query) == "" {
		return
	}

	reportAdvisoryViolations(pass, query, call.Pos(), dm)
}

func reportAdvisoryViolations(pass *analysis.Pass, query string, pos token.Pos, dm *directives.DirectiveMap) {
	if dm.IsIgnored(pass.Fset, pos, RuleCode) {
		return
	}

	violations := InspectAdvisorySQL(query)
	for _, v := range violations {
		switch v.Type {
		case ViolationSessionLock:
			pass.Reportf(pos, "[%s] forbidden session-level advisory lock %q; use transaction-scoped \"pg_advisory_xact_lock\" or \"argus.WithAdvisoryLock\" to prevent connection pool leaks", RuleCode, v.FuncName)
		case ViolationHardcodedIntKey:
			pass.Reportf(pos, "[%s] hardcoded integer advisory lock key in SQL; use registered namespace constants or argus.LockKey(domain, resource) to prevent collision", RuleCode)
		}
	}
}
