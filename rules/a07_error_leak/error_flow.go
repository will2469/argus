// Package a07_error_leak traces data flow of database error strings and pgconn.PgError fields.
package a07_error_leak

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/directives"
)

// CheckPgErrorSensitiveFields checks for forbidden direct access to Detail, Hint, or Where on PgError.
func CheckPgErrorSensitiveFields(pass *analysis.Pass, sel *ast.SelectorExpr, dm *directives.DirectiveMap) {
	switch sel.Sel.Name {
	case "Detail", "Hint", "Where":
		if IsPgErrorSelector(pass, sel) {
			if dm.IsIgnored(pass.Fset, sel.Pos(), RuleCode) {
				return
			}
			pass.Reportf(sel.Pos(), "[%s] forbidden direct access to pgconn.PgError.%s; contains raw database internals and PII", RuleCode, sel.Sel.Name)
		}
	}
}

// IsPgErrorSelector determines whether a selector refers to a pgconn.PgError struct.
func IsPgErrorSelector(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	if pass != nil && pass.TypesInfo != nil {
		if typ := pass.TypesInfo.TypeOf(sel.X); typ != nil {
			if strings.Contains(typ.String(), "pgconn.PgError") {
				return true
			}
		}
	}
	// Heuristic fallback
	if id, ok := sel.X.(*ast.Ident); ok {
		name := strings.ToLower(id.Name)
		return strings.Contains(name, "pgerr") || strings.Contains(name, "pgerror")
	}
	return false
}

// CheckLeakedErrorArg validates an expression passed into an API response sink.
func CheckLeakedErrorArg(pass *analysis.Pass, arg ast.Expr, callPos token.Pos, body *ast.BlockStmt, dm *directives.DirectiveMap) {
	if dm.IsIgnored(pass.Fset, arg.Pos(), RuleCode) || dm.IsIgnored(pass.Fset, callPos, RuleCode) {
		return
	}

	// 1. Direct call: err.Error()
	if IsErrorCall(arg) {
		pass.Reportf(arg.Pos(), "[%s] raw err.Error() passed directly to HTTP response; may leak internal database errors and PII", RuleCode)
		return
	}

	// 2. Local variable assigned from err.Error() or pgErr.Detail
	if id, ok := arg.(*ast.Ident); ok {
		if IsVarAssignedFromError(id.Name, body) {
			pass.Reportf(arg.Pos(), "[%s] variable %q derived from raw err.Error() passed to HTTP response; may leak internal database errors and PII", RuleCode, id.Name)
		}
	}
}

// IsErrorCall checks if an expression is a call to .Error().
func IsErrorCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "Error" && len(call.Args) == 0
}

// IsVarAssignedFromError checks if a variable in the block is derived from err.Error() or PgError fields.
func IsVarAssignedFromError(varName string, body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}

	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range assign.Lhs {
			id, ok := lhs.(*ast.Ident)
			if ok && id.Name == varName && i < len(assign.Rhs) {
				if IsErrorCall(assign.Rhs[i]) {
					found = true
					return false
				}
				if sel, ok := assign.Rhs[i].(*ast.SelectorExpr); ok {
					if sel.Sel.Name == "Detail" || sel.Sel.Name == "Hint" || sel.Sel.Name == "Where" {
						found = true
						return false
					}
				}
			}
		}
		return true
	})
	return found
}
