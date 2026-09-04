// Package a14_select_star detects and forbids wildcard column selection (SELECT * and alias.*)
// in application database queries to prevent TOAST table bloat, buffer cache pollution, and PII leaks (CWE-200).
package a14_select_star

import (
	"go/ast"
	"go/token"
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

// Issue describes a detected violation of ARGUS-A14.
type Issue struct {
	Pos     token.Pos
	Message string
}

// InspectFile inspects an AST file for forbidden SELECT * wildcard queries.
// Supports both pass mode and standalone runner mode (pass == nil).
func InspectFile(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap) []Issue {
	if file == nil {
		return nil
	}
	if fset == nil && pass != nil {
		fset = pass.Fset
	}

	pos := fset.Position(file.Package)
	if strings.HasSuffix(pos.Filename, "_test.go") {
		return nil
	}

	var issues []Issue
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if !isDatabaseCall(pass, call) {
			return true
		}

		flagged := false
		queries := ResolveQueryStrings(pass, file, call)
		for _, query := range queries {
			if strings.TrimSpace(query) == "" {
				continue
			}

			if HasForbiddenSelectStar(query) {
				if fset != nil && dm != nil && (dm.IsIgnored(fset, call.Pos(), RuleCode) || dm.IsIgnored(fset, call.Pos(), RuleCode+".SELECT-STAR")) {
					flagged = true
					break
				}
				issues = append(issues, Issue{
					Pos:     call.Pos(),
					Message: "Forbidden 'SELECT *' or wildcard column selection detected; explicitly list required columns to prevent TOAST table bloat and data exposure (CWE-200)",
				})
				flagged = true
				break
			}
		}
		if flagged {
			return true
		}

		// Check dynamic query construction under the 'unknown != safe' invariant
		sqlArg := callsite.ExtractSQLArg(call, pass)
		if isRisk, reason := CheckDynamicQueryRisk(pass, file, sqlArg, call.Pos()); isRisk {
			if fset != nil && dm != nil && (dm.IsIgnored(fset, call.Pos(), RuleCode) || dm.IsIgnored(fset, call.Pos(), RuleCode+".SELECT-STAR")) {
				return true
			}
			issues = append(issues, Issue{
				Pos:     call.Pos(),
				Message: reason,
			})
		}

		return true
	})

	return issues
}

func run(pass *analysis.Pass) (interface{}, error) {
	cfg := pass.ResultOf[config.Analyzer].(*config.Config)
	if !cfg.IsRuleEnabled(RuleCode) {
		return nil, nil
	}

	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)

	for _, file := range pass.Files {
		issues := InspectFile(pass, pass.Fset, file, dm)
		for _, iss := range issues {
			pass.Reportf(iss.Pos, "[%s] %s", RuleCode, iss.Message)
		}
	}

	return nil, nil
}
