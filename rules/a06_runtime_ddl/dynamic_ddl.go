// Package a06_runtime_ddl detects dynamic DDL queries constructed via string concatenation,
// formatting (fmt.Sprintf), and string builders in runtime Go code.
package a06_runtime_ddl

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

// EvalDynamicDDL evaluates an AST expression to determine if it constructs a DDL query.
func EvalDynamicDDL(expr ast.Expr) string {
	if expr == nil {
		return ""
	}

	switch e := expr.(type) {
	case *ast.CallExpr:
		if op := evalSprintfDDL(e); op != "" {
			return op
		}
		if op := evalSprintDDL(e); op != "" {
			return op
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return evalConcatDDL(e)
		}
	case *ast.ParenExpr:
		return EvalDynamicDDL(e.X)
	}

	return ""
}

func evalSprintfDDL(call *ast.CallExpr) string {
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

	return MatchDDLCommand(formatVal)
}

func evalSprintDDL(call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Sprint" {
		return ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Name != "fmt" || len(call.Args) == 0 {
		return ""
	}

	var parts []string
	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			val, _ := strconv.Unquote(lit.Value)
			parts = append(parts, val)
		}
	}
	if len(parts) > 0 {
		return MatchDDLCommand(strings.Join(parts, " "))
	}
	return ""
}

func evalConcatDDL(bin *ast.BinaryExpr) string {
	parts := extractConcatLiterals(bin)
	if len(parts) == 0 {
		return ""
	}
	joined := strings.Join(parts, " ")
	return MatchDDLCommand(joined)
}

func extractConcatLiterals(e ast.Expr) []string {
	var parts []string
	var walk func(expr ast.Expr)
	walk = func(expr ast.Expr) {
		switch x := expr.(type) {
		case *ast.BasicLit:
			if x.Kind == token.STRING {
				val, err := strconv.Unquote(x.Value)
				if err != nil {
					val = strings.Trim(x.Value, "`\"")
				}
				if val != "" {
					parts = append(parts, val)
				}
			}
		case *ast.BinaryExpr:
			if x.Op == token.ADD {
				walk(x.X)
				walk(x.Y)
			}
		case *ast.ParenExpr:
			walk(x.X)
		}
	}
	walk(e)
	return parts
}
