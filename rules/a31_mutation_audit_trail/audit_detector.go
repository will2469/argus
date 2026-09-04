// Package a31_mutation_audit_trail provides audit invocation recognition utilities.
package a31_mutation_audit_trail

import (
	"go/ast"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
)

var auditInsertRegex = regexp.MustCompile(`(?i)^\s*INSERT\s+INTO\s+([a-zA-Z0-9_.]*audit[a-zA-Z0-9_.]*|security_events)`)

// isAuditCall verifies whether a call expression represents an audit trail recording.
func isAuditCall(call *ast.CallExpr, auditMethods []string, pass *analysis.Pass) bool {
	if call == nil {
		return false
	}

	methodName := callsite.GetCallMethodName(call.Fun)
	if methodName == "" {
		return false
	}

	// 1. Check against configured audit method names
	for _, m := range auditMethods {
		if methodName == m {
			return true
		}
	}

	// 2. Check if this is a direct database insert into an audit log table
	if callsite.IsDBQueryMethod(methodName) {
		sqlArg := callsite.ExtractSQLArg(call, pass)
		if sqlArg != nil {
			if strVal := extractSimpleQueryString(sqlArg); strVal != "" {
				if auditInsertRegex.MatchString(strVal) {
					return true
				}
			}
		}
	}

	return false
}

func extractSimpleQueryString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return strings.Trim(e.Value, "`\"")
	case *ast.BinaryExpr:
		return extractSimpleQueryString(e.X)
	case *ast.ParenExpr:
		return extractSimpleQueryString(e.X)
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Sprintf" && len(e.Args) > 0 {
			return extractSimpleQueryString(e.Args[0])
		}
	}
	return ""
}
