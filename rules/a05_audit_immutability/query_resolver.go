// Package a05_audit_immutability provides query resolution utilities to extract
// compile-time SQL queries and trace variable assignments across function blocks.
package a05_audit_immutability

import (
	"go/ast"
	"go/token"

	"github.com/will2469/argus/shared/callsite"
)

func extractAllQueryStrings(call *ast.CallExpr, body *ast.BlockStmt) []string {
	if call == nil || len(call.Args) == 0 {
		return nil
	}

	var results []string
	for _, arg := range call.Args {
		switch e := arg.(type) {
		case *ast.BasicLit:
			if s := unquoteString(e); s != "" {
				results = append(results, s)
			}
		case *ast.BinaryExpr:
			if s := extractBinaryConcat(e); s != "" {
				results = append(results, s)
			}
		case *ast.Ident:
			if body != nil {
				ast.Inspect(body, func(n ast.Node) bool {
					assign, ok := n.(*ast.AssignStmt)
					if !ok || assign.Pos() >= call.Pos() {
						return true
					}
					for i, lhs := range assign.Lhs {
						if id, ok := lhs.(*ast.Ident); ok && id.Name == e.Name && i < len(assign.Rhs) {
							switch rhs := assign.Rhs[i].(type) {
							case *ast.BasicLit:
								if s := unquoteString(rhs); s != "" {
									results = append(results, s)
								}
							case *ast.BinaryExpr:
								if s := extractBinaryConcat(rhs); s != "" {
									results = append(results, s)
								}
							case *ast.Ident:
								if s := resolveIdentLiteral(rhs, body, assign.Pos()); s != "" {
									results = append(results, s)
								}
							}
						}
					}
					return true
				})
			}
		}
	}

	if len(results) == 0 {
		if s, ok := callsite.ExtractQueryString(call); ok {
			results = append(results, s)
		}
	}

	return results
}

func unquoteString(lit *ast.BasicLit) string {
	if lit == nil || lit.Kind != token.STRING {
		return ""
	}
	val := lit.Value
	if len(val) >= 2 && ((val[0] == '`' && val[len(val)-1] == '`') ||
		(val[0] == '"' && val[len(val)-1] == '"')) {
		return val[1 : len(val)-1]
	}
	return val
}

func extractBinaryConcat(bin *ast.BinaryExpr) string {
	if bin == nil || bin.Op != token.ADD {
		return ""
	}
	s1 := extractExprString(bin.X)
	s2 := extractExprString(bin.Y)
	if s1 != "" || s2 != "" {
		return s1 + s2
	}
	return ""
}

func extractExprString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return unquoteString(e)
	case *ast.BinaryExpr:
		return extractBinaryConcat(e)
	}
	return ""
}

func resolveIdentLiteral(id *ast.Ident, body *ast.BlockStmt, beforePos token.Pos) string {
	var found string
	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || assign.Pos() >= beforePos {
			return true
		}
		for i, lhs := range assign.Lhs {
			if target, ok := lhs.(*ast.Ident); ok && target.Name == id.Name && i < len(assign.Rhs) {
				if s := extractExprString(assign.Rhs[i]); s != "" {
					found = s
				}
			}
		}
		return true
	})
	return found
}
