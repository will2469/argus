// Package a08_tx_io identifies database transaction scopes (closures and explicit Begin/Commit blocks).
package a08_tx_io

import (
	"go/ast"
)

// ExtractTxClosure detects closure-based transactions (BeginFunc, ExecuteTx, ExecuteLockedTx, WithTx).
func ExtractTxClosure(call *ast.CallExpr) *ast.FuncLit {
	methodName := getCallMethodName(call.Fun)
	switch methodName {
	case "BeginFunc", "ExecuteTx", "ExecuteLockedTx", "WithTx":
		for _, arg := range call.Args {
			if lit, ok := arg.(*ast.FuncLit); ok {
				return lit
			}
		}
	}
	return nil
}

// InspectExplicitTxRanges detects statements executed while holding an open transaction (pool.Begin ... tx.Commit).
func InspectExplicitTxRanges(body *ast.BlockStmt, onStmtInTx func(stmt ast.Stmt)) {
	if body == nil {
		return
	}

	var inTx bool
	var txVarName string

	for _, stmt := range body.List {
		// Detect tx, err := pool.Begin(...) or pool.BeginTx(...)
		if assign, ok := stmt.(*ast.AssignStmt); ok {
			for i, rhs := range assign.Rhs {
				if call, ok := rhs.(*ast.CallExpr); ok {
					name := getCallMethodName(call.Fun)
					if name == "Begin" || name == "BeginTx" {
						if i < len(assign.Lhs) {
							if id, ok := assign.Lhs[i].(*ast.Ident); ok {
								inTx = true
								txVarName = id.Name
							}
						}
					}
				}
			}
			continue
		}

		if inTx {
			// Defer statements (like defer tx.Rollback(ctx)) do not terminate the active transaction block now
			if _, isDefer := stmt.(*ast.DeferStmt); isDefer {
				continue
			}

			// Check if statement ends transaction: tx.Commit() or non-deferred tx.Rollback()
			if isTxEndStmt(stmt, txVarName) {
				inTx = false
				txVarName = ""
				continue
			}

			onStmtInTx(stmt)
		}
	}
}

func isTxEndStmt(stmt ast.Stmt, txVarName string) bool {
	var isEnd bool
	ast.Inspect(stmt, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if (sel.Sel.Name == "Commit" || sel.Sel.Name == "Rollback") && sel.X != nil {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == txVarName {
				isEnd = true
				return false
			}
		}
		return true
	})
	return isEnd
}

func getCallMethodName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	}
	return ""
}
