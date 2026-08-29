// Package a02_unclosed_rows enforces that every pgx.Rows produced by Query()
// is safely closed via defer rows.Close() or consumed by an auto-closing helper.
package a02_unclosed_rows

import (
	"go/ast"
)

// IsQueryCall determines whether an AST expression is a database Query call.
func IsQueryCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Query" {
		return false
	}
	if innerSel, ok := sel.X.(*ast.SelectorExpr); ok && innerSel.Sel.Name == "URL" {
		return false
	}
	return true
}

// IsReturned checks whether a variable name is returned in any return statement within the block.
func IsReturned(body *ast.BlockStmt, rowsVar string) bool {
	returned := false
	ast.Inspect(body, func(n ast.Node) bool {
		if returned {
			return false
		}
		if ret, ok := n.(*ast.ReturnStmt); ok {
			for _, res := range ret.Results {
				if id, ok := res.(*ast.Ident); ok && id.Name == rowsVar {
					returned = true
					return false
				}
			}
		}
		return true
	})
	return returned
}

// IsSafelyClosedOrConsumed verifies whether the rows variable is closed via defer
// or consumed by an auto-closing helper.
func IsSafelyClosedOrConsumed(body *ast.BlockStmt, rowsVar string) bool {
	safe := false

	ast.Inspect(body, func(n ast.Node) bool {
		if safe {
			return false
		}

		// 1. defer rows.Close() or defer func() { rows.Close() }()
		if defStmt, ok := n.(*ast.DeferStmt); ok {
			if isCloseCallOnVar(defStmt.Call, rowsVar) {
				safe = true
				return false
			}
			if fnLit, ok := defStmt.Call.Fun.(*ast.FuncLit); ok && fnLit.Body != nil {
				ast.Inspect(fnLit.Body, func(inner ast.Node) bool {
					if call, ok := inner.(*ast.CallExpr); ok && isCloseCallOnVar(call, rowsVar) {
						safe = true
						return false
					}
					return true
				})
			}
		}

		// 2. Auto-closing helper calls: pgx.CollectRows(rows, ...), pgx.CollectOneRow, pgx.ForEachRow
		if call, ok := n.(*ast.CallExpr); ok {
			if isAutoClosingHelper(call, rowsVar) {
				safe = true
				return false
			}
		}

		return true
	})

	return safe
}

func isCloseCallOnVar(call *ast.CallExpr, rowsVar string) bool {
	if call == nil {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Close" {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == rowsVar
}

func isAutoClosingHelper(call *ast.CallExpr, rowsVar string) bool {
	if call == nil || len(call.Args) == 0 {
		return false
	}
	argIdent, ok := call.Args[0].(*ast.Ident)
	if !ok || argIdent.Name != rowsVar {
		return false
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch sel.Sel.Name {
	case "CollectRows", "CollectOneRow", "ForEachRow":
		return true
	}
	return false
}
