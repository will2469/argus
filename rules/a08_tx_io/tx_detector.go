// Package a08_tx_io identifies database transaction scopes (closures and explicit Begin/Commit blocks)
// using semantic receiver and type proof to avoid confusing non-database objects with database transactions.
package a08_tx_io

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
)

// ExtractTxClosure detects closure-based transactions (BeginFunc, ExecuteTx, ExecuteLockedTx, WithTx)
// ensuring the receiver is proven to be a database pool or connection.
func ExtractTxClosure(pass *analysis.Pass, call *ast.CallExpr) *ast.FuncLit {
	if call == nil {
		return nil
	}

	methodName := callsite.GetCallMethodName(call.Fun)
	switch methodName {
	case "BeginFunc", "ExecuteTx", "ExecuteLockedTx", "WithTx":
	default:
		return nil
	}

	if !isDBReceiver(pass, call.Fun) {
		return nil
	}

	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.FuncLit); ok {
			return lit
		}
	}
	return nil
}

// InspectExplicitTxRanges detects statements executed while holding an open transaction (pool.Begin ... tx.Commit).
func InspectExplicitTxRanges(pass *analysis.Pass, body *ast.BlockStmt, onStmtInTx func(stmt ast.Stmt)) {
	if body == nil {
		return
	}

	var inTx bool
	var txVarName string

	for _, stmt := range body.List {
		if !inTx {
			// Detect tx, err := pool.Begin(...) or pool.BeginTx(...)
			if assign, ok := stmt.(*ast.AssignStmt); ok {
				for i, rhs := range assign.Rhs {
					if call, ok := rhs.(*ast.CallExpr); ok {
						name := callsite.GetCallMethodName(call.Fun)
						if (name == "Begin" || name == "BeginTx") && isDBReceiver(pass, call.Fun) {
							if i < len(assign.Lhs) {
								if id, ok := assign.Lhs[i].(*ast.Ident); ok {
									if isDBTxIdent(pass, id) {
										inTx = true
										txVarName = id.Name
									}
								}
							}
						}
					}
				}
			}
			continue
		}

		// While in transaction:
		// Defer statements (like defer tx.Rollback(ctx)) do not terminate the active transaction block early
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

func isDBReceiver(pass *analysis.Pass, fun ast.Expr) bool {
	sel := callsite.GetCallSelector(fun)
	if sel == nil {
		return false
	}

	if pass != nil && pass.TypesInfo != nil {
		t := pass.TypesInfo.TypeOf(sel.X)
		if t != nil {
			if callsite.IsPgxOrSQLType(t) || hasDBPoolMethodSet(t) {
				return true
			}
			// If type is known and doesn't match DB pool/conn, it's not a DB transaction
			return false
		}
	}

	// AST fallback: check receiver identifier naming
	recvName := getReceiverName(sel.X)
	if recvName != "" {
		lower := strings.ToLower(recvName)
		if isKnownNonDBName(lower) {
			return false
		}
		if strings.Contains(lower, "pool") || strings.Contains(lower, "db") ||
			strings.Contains(lower, "conn") || strings.Contains(lower, "store") ||
			strings.Contains(lower, "repo") || strings.Contains(lower, "txmgr") {
			return true
		}
	}

	return false
}

func getReceiverName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	}
	return ""
}

func hasDBPoolMethodSet(t types.Type) bool {
	if t == nil {
		return false
	}
	var hasBegin bool

	checkFunc := func(fn *types.Func) {
		name := fn.Name()
		if name == "Begin" || name == "BeginTx" || name == "BeginFunc" {
			hasBegin = true
		}
	}

	for {
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		} else {
			break
		}
	}

	if named, ok := t.(*types.Named); ok {
		for i := 0; i < named.NumMethods(); i++ {
			checkFunc(named.Method(i))
		}
	}
	if iface, ok := t.Underlying().(*types.Interface); ok {
		for i := 0; i < iface.NumMethods(); i++ {
			checkFunc(iface.Method(i))
		}
	}
	return hasBegin
}

func isDBTxIdent(pass *analysis.Pass, id *ast.Ident) bool {
	if id == nil {
		return false
	}
	if pass != nil && pass.TypesInfo != nil {
		t := pass.TypesInfo.TypeOf(id)
		if t != nil {
			if callsite.IsPgxOrSQLType(t) || hasDBTxMethodSet(t) {
				return true
			}
			return false
		}
	}

	// AST fallback
	lower := strings.ToLower(id.Name)
	return strings.Contains(lower, "tx") || strings.Contains(lower, "trx")
}

func hasDBTxMethodSet(t types.Type) bool {
	if t == nil {
		return false
	}
	var hasCommit, hasRollback bool

	checkFunc := func(fn *types.Func) {
		name := fn.Name()
		if name == "Commit" {
			hasCommit = true
		} else if name == "Rollback" {
			hasRollback = true
		}
	}

	for {
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		} else {
			break
		}
	}

	if named, ok := t.(*types.Named); ok {
		for i := 0; i < named.NumMethods(); i++ {
			checkFunc(named.Method(i))
		}
	}
	if iface, ok := t.Underlying().(*types.Interface); ok {
		for i := 0; i < iface.NumMethods(); i++ {
			checkFunc(iface.Method(i))
		}
	}
	return hasCommit && hasRollback
}

func isKnownNonDBName(name string) bool {
	nonDBWords := []string{"parser", "scanner", "calc", "calculator", "story", "builder", "generator"}
	for _, word := range nonDBWords {
		if strings.Contains(name, word) {
			return true
		}
	}
	return false
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
