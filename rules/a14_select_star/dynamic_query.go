// Package a14_select_star detects wildcard risks in dynamic SQL construction under the invariant 'unknown != safe'.
package a14_select_star

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var (
	dynamicSelectStarRegex = regexp.MustCompile(`(?i)\bSELECT\s+(?:DISTINCT\s+)?(?:\w+\.)?\*\s+(?:FROM|\n\s*FROM)\b`)
	selectPrefixRegex      = regexp.MustCompile(`(?i)\bSELECT(?:\s+DISTINCT)?$`)
	fromPrefixRegex        = regexp.MustCompile(`(?i)^\s*FROM\b`)
	dynamicFormatRegex     = regexp.MustCompile(`(?i)\bSELECT(?:\s+DISTINCT)?\s+%[+v#sv]\s+FROM\b`)
)

// CheckDynamicQueryRisk evaluates whether an expression dynamically constructs a SQL query
// whose column projection cannot be verified free of wildcards ('unknown != safe').
func CheckDynamicQueryRisk(pass *analysis.Pass, file *ast.File, expr ast.Expr, pos token.Pos) (bool, string) {
	if expr == nil {
		return false, ""
	}

	// 1. If expression is an identifier, inspect its definitions
	if id, ok := expr.(*ast.Ident); ok {
		defExprs := getIdentDefExprs(pass, file, id, pos)
		for _, defExpr := range defExprs {
			if risk, reason := CheckDynamicQueryRisk(pass, file, defExpr, pos); risk {
				return true, reason
			}
		}
		return false, ""
	}

	// 2. Direct inspection of expression
	return inspectExprForDynamicRisk(pass, file, expr, pos)
}

func inspectExprForDynamicRisk(pass *analysis.Pass, file *ast.File, expr ast.Expr, pos token.Pos) (bool, string) {
	if expr == nil {
		return false, ""
	}

	// Check for fmt.Sprintf with dynamic projection or static wildcard format
	if call, ok := expr.(*ast.CallExpr); ok && isFmtSprintf(call) && len(call.Args) > 0 {
		if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			formatStr := strings.Trim(lit.Value, "`\"")
			if dynamicSelectStarRegex.MatchString(formatStr) {
				return true, "Forbidden 'SELECT *' or wildcard column selection detected; explicitly list required columns to prevent TOAST table bloat and data exposure (CWE-200)"
			}
			if dynamicFormatRegex.MatchString(formatStr) {
				return true, "Forbidden 'SELECT *' or wildcard column selection detected; dynamic column projection cannot be verified free of wildcards (CWE-200)"
			}
		}
	}

	// Check for binary expression string concatenation
	if bin, ok := expr.(*ast.BinaryExpr); ok && bin.Op == token.ADD {
		parts := flattenConcatParts(pass, file, bin, pos)
		return analyzeConcatParts(parts)
	}

	return false, ""
}

type concatPart struct {
	isLit bool
	val   string
}

func flattenConcatParts(pass *analysis.Pass, file *ast.File, expr ast.Expr, pos token.Pos) []concatPart {
	var parts []concatPart
	var walk func(e ast.Expr)
	walk = func(e ast.Expr) {
		if e == nil {
			return
		}
		switch node := e.(type) {
		case *ast.BinaryExpr:
			if node.Op == token.ADD {
				walk(node.X)
				walk(node.Y)
				return
			}
		case *ast.BasicLit:
			if node.Kind == token.STRING {
				parts = append(parts, concatPart{isLit: true, val: strings.Trim(node.Value, "`\"")})
				return
			}
		case *ast.ParenExpr:
			walk(node.X)
			return
		case *ast.Ident:
			resolved := resolveExprStrings(pass, file, node, pos, 0)
			if len(resolved) == 1 {
				parts = append(parts, concatPart{isLit: true, val: resolved[0]})
				return
			}
		}
		parts = append(parts, concatPart{isLit: false})
	}
	walk(expr)
	return parts
}

func analyzeConcatParts(parts []concatPart) (bool, string) {
	for i, p := range parts {
		// 1. Any individual static literal containing SELECT * or alias.*
		if p.isLit && dynamicSelectStarRegex.MatchString(p.val) && !IsExemptRegex(p.val) {
			return true, "Forbidden 'SELECT *' or wildcard column selection detected; explicitly list required columns to prevent TOAST table bloat and data exposure (CWE-200)"
		}

		// 2. Dynamic column projection: static part ends with SELECT, followed by dynamic part, then FROM
		if p.isLit && selectPrefixRegex.MatchString(strings.TrimSpace(p.val)) {
			if i+1 < len(parts) && !parts[i+1].isLit {
				for j := i + 2; j < len(parts); j++ {
					if parts[j].isLit && fromPrefixRegex.MatchString(parts[j].val) {
						return true, "Forbidden 'SELECT *' or wildcard column selection detected; dynamic column projection cannot be verified free of wildcards (CWE-200)"
					}
				}
			}
		}
	}
	return false, ""
}

func isFmtSprintf(call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if id, ok := sel.X.(*ast.Ident); ok && id.Name == "fmt" && sel.Sel.Name == "Sprintf" {
		return true
	}
	return false
}
