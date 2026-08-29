// Package a18_rows_err implements the ARGUS-A18 static analysis rule.
package a18_rows_err

import (
	"go/ast"
)

// IsRowsErrCall checks if an AST CallExpr represents a call to <rowsVar>.Err().
func IsRowsErrCall(call *ast.CallExpr, rowsVar string) bool {
	if call == nil {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Err" {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == rowsVar
}

// HasErrCheckInStmt inspects a statement to find whether <rowsVar>.Err() is invoked.
func HasErrCheckInStmt(stmt ast.Stmt, rowsVar string) bool {
	if stmt == nil {
		return false
	}
	found := false
	ast.Inspect(stmt, func(n ast.Node) bool {
		if found {
			return false
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if IsRowsErrCall(call, rowsVar) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// HasValidPostLoopErrCheck verifies whether the statements following a for loop
// include a valid call to <rowsVar>.Err() before any unconditional return.
func HasValidPostLoopErrCheck(stmts []ast.Stmt, rowsVar string) bool {
	if len(stmts) == 0 {
		return false
	}

	for _, stmt := range stmts {
		if HasErrCheckInStmt(stmt, rowsVar) {
			return true
		}

		// If an unconditional return statement is encountered before rows.Err() was checked,
		// then execution escapes without verifying stream integrity.
		if _, ok := stmt.(*ast.ReturnStmt); ok {
			return false
		}
	}

	return false
}
