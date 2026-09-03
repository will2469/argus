package a01_sql_concat

import (
	"go/ast"
	"go/types"
	"strings"
)

func isTaintSourceAST(name string, typeStr string) bool {
	switch strings.ToLower(name) {
	case "id", "nik", "email", "userid", "user_id", "param", "params", "query", "q",
		"search", "filter", "sort", "order", "orderby", "order_by", "table", "column", "rawsql", "sql":
		return true
	}
	if typeStr != "" {
		if strings.HasSuffix(typeStr, "Request") || strings.HasSuffix(typeStr, "DTO") ||
			strings.HasSuffix(typeStr, "Input") || strings.HasSuffix(typeStr, "Params") ||
			strings.HasSuffix(typeStr, "Filter") || strings.Contains(typeStr, "http.Request") {
			return true
		}
	}
	return false
}

func astExprToString(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return "*" + astExprToString(x.X)
	case *ast.SelectorExpr:
		return astExprToString(x.X) + "." + x.Sel.Name
	default:
		return ""
	}
}

func isTaintSource(name string, typ types.Type) bool {
	switch strings.ToLower(name) {
	case "id", "nik", "email", "userid", "user_id", "param", "params", "query", "q",
		"search", "filter", "sort", "order", "orderby", "order_by", "table", "column", "rawsql", "sql":
		return true
	}
	if typ != nil {
		s := typ.String()
		if strings.HasSuffix(s, "Request") || strings.HasSuffix(s, "DTO") ||
			strings.HasSuffix(s, "Input") || strings.HasSuffix(s, "Params") ||
			strings.HasSuffix(s, "Filter") || strings.Contains(s, "http.Request") {
			return true
		}
	}
	return false
}
