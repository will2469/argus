// Package a24_tenant_leak enforces explicit tenant isolation predicates (WHERE tenant_id = $1)
// or verified RLS session context on multi-tenant tables to prevent cross-tenant data leaks (CWE-284, BOLA).
package a24_tenant_leak

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

// RuleCode is the official identifier for ARGUS-A24.
const RuleCode = "ARGUS-A24"

// Analyzer defines the analysis.Analyzer for ARGUS-A24.
var Analyzer = &analysis.Analyzer{
	Name: "argus_a24_tenant_isolation_leak",
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

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}

			hasRLS := HasRLSSessionSetup(fn, tc.TenantColumn)

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				methodName := callsite.GetCallMethodName(call.Fun)
				if !callsite.IsDBQueryMethod(methodName) {
					return true
				}

				if hasRLS {
					return true
				}

				if dm != nil && (dm.IsIgnored(pass.Fset, call.Pos(), RuleCode) || dm.IsIgnored(pass.Fset, call.Pos(), RuleCode+".TENANT-LEAK")) {
					return true
				}

				queryStr, ok := callsite.ExtractQueryString(call)
				if !ok || strings.TrimSpace(queryStr) == "" {
					return true
				}

				if isViolating, reason := CheckTenantQuery(queryStr, tc); isViolating {
					pass.Reportf(call.Pos(), "[%s] %s", RuleCode, reason)
				}

				return true
			})
		}
	}

	return nil, nil
}
