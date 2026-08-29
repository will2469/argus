// Package a14_select_star detects and forbids wildcard column selection (SELECT * and alias.*)
// in application database queries to prevent TOAST table bloat, buffer cache pollution, and PII leaks (CWE-200).
package a14_select_star

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

// RuleCode is the official identifier for ARGUS-A14.
const RuleCode = "ARGUS-A14"

// Analyzer defines the analysis.Analyzer for ARGUS-A14.
var Analyzer = &analysis.Analyzer{
	Name: "a14",
	Doc:  "Forbid unconstrained 'SELECT *' wildcard queries; require explicit column projection to mitigate TOAST bloat and PII leak (CWE-200)",
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

			methodName := callsite.GetCallMethodName(call.Fun)
			if !callsite.IsDBQueryMethod(methodName) {
				return true
			}

			query, found := callsite.ExtractQueryString(call)
			if !found || strings.TrimSpace(query) == "" {
				return true
			}

			if HasForbiddenSelectStar(query) {
				if dm != nil && (dm.IsIgnored(pass.Fset, call.Pos(), RuleCode) || dm.IsIgnored(pass.Fset, call.Pos(), RuleCode+".SELECT-STAR")) {
					return true
				}
				pass.Reportf(call.Pos(),
					"[%s] Forbidden 'SELECT *' or wildcard column selection detected; explicitly list required columns to prevent TOAST table bloat and data exposure (CWE-200)",
					RuleCode)
			}

			return true
		})
	}

	return nil, nil
}
