// Package a06_runtime_ddl detects dynamic DDL patterns constructed via fmt.Sprintf or string operations.
package a06_runtime_ddl

import (
	"go/ast"
	"go/token"
	"regexp"
	"strconv"
	"strings"
)

var ddlKeywordsRegex = regexp.MustCompile(`(?i)\b(CREATE\s+TABLE|DROP\s+TABLE|ALTER\s+TABLE|TRUNCATE(\s+TABLE)?|CREATE\s+INDEX|DROP\s+INDEX|CREATE\s+SCHEMA|DROP\s+SCHEMA|GRANT\s+|REVOKE\s+)\b`)

// DetectDynamicDDL checks if an AST expression is a dynamic string construction
// (e.g. fmt.Sprintf) that assembles a DDL query.
func DetectDynamicDDL(expr ast.Expr, body *ast.BlockStmt) string {
	if expr == nil {
		return ""
	}

	// 1. Direct call: fmt.Sprintf("CREATE TABLE ...", ...)
	if call, ok := expr.(*ast.CallExpr); ok {
		if op := checkSprintfCall(call); op != "" {
			return op
		}
	}

	// 2. Variable identifier tracing
	if ident, ok := expr.(*ast.Ident); ok && body != nil {
		var foundOp string
		ast.Inspect(body, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, lhs := range assign.Lhs {
				id, ok := lhs.(*ast.Ident)
				if ok && id.Name == ident.Name && i < len(assign.Rhs) {
					rhs := assign.Rhs[i]
					if call, ok := rhs.(*ast.CallExpr); ok {
						if op := checkSprintfCall(call); op != "" {
							foundOp = op
						}
					} else if lit, ok := rhs.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						val, _ := strconv.Unquote(lit.Value)
						if op := IsDDLKeyword(val); op != "" {
							foundOp = op
						}
					}
				}
			}
			return true
		})
		if foundOp != "" {
			return foundOp
		}
	}

	return ""
}

func checkSprintfCall(call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Sprintf" {
		return ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Name != "fmt" || len(call.Args) == 0 {
		return ""
	}

	formatLit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || formatLit.Kind != token.STRING {
		return ""
	}

	formatVal, err := strconv.Unquote(formatLit.Value)
	if err != nil {
		formatVal = strings.Trim(formatLit.Value, "`\"")
	}

	return IsDDLKeyword(formatVal)
}

// IsDDLKeyword checks if a string contains DDL keywords.
func IsDDLKeyword(s string) string {
	match := ddlKeywordsRegex.FindString(s)
	if match == "" {
		return ""
	}
	upper := strings.ToUpper(strings.TrimSpace(match))
	if strings.HasPrefix(upper, "CREATE TABLE") {
		return "CREATE TABLE"
	}
	if strings.HasPrefix(upper, "DROP TABLE") {
		return "DROP"
	}
	if strings.HasPrefix(upper, "ALTER TABLE") {
		return "ALTER TABLE"
	}
	if strings.HasPrefix(upper, "TRUNCATE") {
		return "TRUNCATE"
	}
	if strings.HasPrefix(upper, "CREATE INDEX") {
		return "CREATE INDEX"
	}
	if strings.HasPrefix(upper, "GRANT") || strings.HasPrefix(upper, "REVOKE") {
		return "GRANT/REVOKE"
	}
	return upper
}
