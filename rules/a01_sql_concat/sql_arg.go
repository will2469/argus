package a01_sql_concat

import (
	"go/ast"
	"go/token"
	"strings"
)

func hasSQLArgument(call *ast.CallExpr) bool {
	sqlArg := extractSQLArgument(call)
	if sqlArg == nil {
		return false
	}
	if s, ok := extractSimpleString(sqlArg); ok {
		upper := strings.ToUpper(strings.TrimSpace(s))
		for _, prefix := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "WITH", "CREATE", "DROP", "ALTER"} {
			if strings.HasPrefix(upper, prefix) {
				return true
			}
		}
	}
	return false
}

func extractSimpleString(e ast.Expr) (string, bool) {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind == token.STRING && len(x.Value) >= 2 {
			return x.Value[1 : len(x.Value)-1], true
		}
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			return extractSimpleString(x.X)
		}
	}
	return "", false
}

func extractSQLArgument(call *ast.CallExpr) ast.Expr {
	if len(call.Args) == 0 {
		return nil
	}
	if len(call.Args) >= 2 {
		if isContextArg(call.Args[0]) {
			return call.Args[1]
		}
		return call.Args[0]
	}
	return call.Args[0]
}

func isContextArg(arg ast.Expr) bool {
	if id, ok := arg.(*ast.Ident); ok {
		lower := strings.ToLower(id.Name)
		return lower == "ctx" || lower == "context" || strings.HasPrefix(lower, "ctx")
	}
	if call, ok := arg.(*ast.CallExpr); ok {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			lower := strings.ToLower(sel.Sel.Name)
			if lower == "context" || lower == "background" || lower == "todo" {
				return true
			}
		}
	}
	return false
}
