// Package a24_tenant_leak enforces explicit tenant isolation predicates (WHERE tenant_id = $1)
// or verified RLS session context on multi-tenant tables to prevent cross-tenant data leaks (CWE-284, BOLA).
package a24_tenant_leak

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

// RuleCode is the official identifier for ARGUS-A24.
const RuleCode = "ARGUS-A24"

// Issue describes a detected violation of ARGUS-A24.
type Issue struct {
	Pos     token.Pos
	Message string
}

// Analyzer defines the analysis.Analyzer for ARGUS-A24.
var Analyzer = &analysis.Analyzer{
	Name: "a24",
	Doc:  "Enforce explicit tenant boundary predicates (tenant_id/org_id) or verified RLS session context on multi-tenant tables (CWE-284, OWASP API1:2023 BOLA)",
	Run:  run,
	Requires: []*analysis.Analyzer{
		callsite.Analyzer,
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
	tc := LoadTenantConfig(cfg)

	for _, file := range pass.Files {
		pos := pass.Fset.Position(file.Package)
		if strings.HasSuffix(pos.Filename, "_test.go") {
			continue
		}

		issues := InspectFile(pass, pass.Fset, file, dm, tc)
		for _, issue := range issues {
			pass.Reportf(issue.Pos, "[%s] %s", RuleCode, issue.Message)
		}
	}

	return nil, nil
}

// InspectFile inspects an AST file for tenant isolation violations.
// Can be called with pass == nil in standalone CLI runner mode.
func InspectFile(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap, tc *TenantConfig) []Issue {
	if file == nil {
		return nil
	}
	if tc == nil {
		tc = LoadTenantConfig(nil)
	}

	var issues []Issue
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			methodName := callsite.GetCallMethodName(call.Fun)
			if !callsite.IsDBQueryMethod(methodName) {
				return true
			}

			if IsRLSActiveAt(fn.Body, call.Pos(), tc.TenantColumn) {
				return true
			}

			if dm != nil && fset != nil && (dm.IsIgnored(fset, call.Pos(), RuleCode) || dm.IsIgnored(fset, call.Pos(), RuleCode+".TENANT-LEAK")) {
				return true
			}

			queryStr, ok := callsite.ExtractQueryString(call)
			if !ok || strings.TrimSpace(queryStr) == "" {
				return true
			}

			if isViolating, reason := CheckTenantQuery(queryStr, tc); isViolating {
				issues = append(issues, Issue{
					Pos:     call.Pos(),
					Message: reason,
				})
			}

			return true
		})
	}

	return issues
}
