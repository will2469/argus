// Package a05_audit_immutability provides query resolution utilities to extract
// the target SQL query argument from database calls, ignoring parameter data values.
package a05_audit_immutability

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

// extractTargetQueryString extracts the exact SQL statement argument from a database call.
// It distinguishes context parameters from SQL query arguments and ignores bound data arguments.
func extractTargetQueryString(call *ast.CallExpr, body *ast.BlockStmt) (string, bool) {
	if call == nil || len(call.Args) == 0 {
		return "", false
	}

	sqlArg := extractSQLArgExpr(call)
	if sqlArg == nil {
		return "", false
	}

	return resolveExprString(sqlArg, body, call.Pos())
}

func extractSQLArgExpr(call *ast.CallExpr) ast.Expr {
	if len(call.Args) >= 2 {
		if isContextArg(call.Args[0]) {
			return call.Args[1]
		}
		return call.Args[0]
	}
	if len(call.Args) == 1 {
		if isContextArg(call.Args[0]) {
			return nil
		}
		return call.Args[0]
	}
	return nil
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

func resolveExprString(expr ast.Expr, body *ast.BlockStmt, beforePos token.Pos) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return unquoteLiteral(e.Value), true
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			s1, ok1 := resolveExprString(e.X, body, beforePos)
			s2, ok2 := resolveExprString(e.Y, body, beforePos)
			if ok1 && ok2 {
				return s1 + s2, true
			}
			if ok1 {
				return s1, true
			}
			if ok2 {
				return s2, true
			}
		}
	case *ast.Ident:
		if body != nil {
			return resolveIdentAssignment(e.Name, body, beforePos)
		}
	case *ast.CallExpr:
		// Support fmt.Sprintf("... %s ...", ...)
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Sprintf" {
			if len(e.Args) > 0 {
				if lit, ok := e.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					return unquoteLiteral(lit.Value), true
				}
			}
		}
	}
	return "", false
}

func resolveIdentAssignment(varName string, body *ast.BlockStmt, beforePos token.Pos) (string, bool) {
	var found string
	var ok bool
	ast.Inspect(body, func(n ast.Node) bool {
		assign, isAssign := n.(*ast.AssignStmt)
		if !isAssign || assign.Pos() >= beforePos {
			return true
		}
		for i, lhs := range assign.Lhs {
			if id, isId := lhs.(*ast.Ident); isId && id.Name == varName && i < len(assign.Rhs) {
				if s, resolved := resolveExprString(assign.Rhs[i], body, assign.Pos()); resolved {
					found = s
					ok = true
				}
			}
		}
		return true
	})
	return found, ok
}

func unquoteLiteral(val string) string {
	s, err := strconv.Unquote(val)
	if err == nil {
		return s
	}
	if len(val) >= 2 && ((val[0] == '`' && val[len(val)-1] == '`') ||
		(val[0] == '"' && val[len(val)-1] == '"')) {
		return val[1 : len(val)-1]
	}
	return val
}
