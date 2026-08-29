// Package a24_tenant_leak inspects function AST scope for RLS session parameter setups.
package a24_tenant_leak

import (
	"go/ast"
	"regexp"
	"strings"

	"github.com/will2469/argus/shared/callsite"
)

// HasRLSSessionSetup checks if an enclosing function/block contains an RLS configuration statement.
func HasRLSSessionSetup(fn *ast.FuncDecl, tenantCol string) bool {
	if fn == nil || fn.Body == nil {
		return false
	}

	colPattern := "tenant_id|org_id"
	if tenantCol != "" && tenantCol != "tenant_id" && tenantCol != "org_id" {
		colPattern = "tenant_id|org_id|" + regexp.QuoteMeta(strings.ToLower(tenantCol))
	}
	rlsPattern := `(?i)\bSET\s+(?:LOCAL\s+)?app\.(?:` + colPattern + `)\b|\bset_config\s*\(\s*['"]app\.(?:` + colPattern + `)['"]`
	rlsRegex := regexp.MustCompile(rlsPattern)

	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sql, ok := callsite.ExtractQueryString(call); ok {
				if rlsRegex.MatchString(sql) {
					found = true
					return false
				}
			}
		}
		return true
	})

	return found
}
