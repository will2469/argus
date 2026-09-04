// Package a08_tx_io identifies database transaction scopes (closures and explicit Begin/Commit blocks)
// using semantic receiver and type proof to avoid confusing non-database objects with database transactions.
package a08_tx_io

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
)

// ExtractTxClosure detects closure-based transactions (BeginFunc, ExecuteTx, ExecuteLockedTx, WithTx)
// ensuring the receiver is proven to be a database pool or connection.
func ExtractTxClosure(pass *analysis.Pass, call *ast.CallExpr, fn *ast.FuncDecl, file *ast.File) *ast.FuncLit {
	if call == nil {
		return nil
	}

	methodName := callsite.GetCallMethodName(call.Fun)
	switch methodName {
	case "BeginFunc", "ExecuteTx", "ExecuteLockedTx", "WithTx":
	default:
		return nil
	}

	if !isDBReceiver(pass, call.Fun, fn, file) {
		return nil
	}

	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.FuncLit); ok {
			return lit
		}
	}
	return nil
}

// isDBReceiver verifies whether a call expression is invoked on a proven database connection or pool.
func isDBReceiver(pass *analysis.Pass, fun ast.Expr, fn *ast.FuncDecl, file *ast.File) bool {
	sel := callsite.GetCallSelector(fun)
	if sel == nil {
		return false
	}

	// 1. Semantic Type Checking via pass.TypesInfo
	if pass != nil && pass.TypesInfo != nil {
		var recvType types.Type
		if selType, ok := pass.TypesInfo.Selections[sel]; ok && selType.Recv() != nil {
			recvType = selType.Recv()
		} else if tv, ok := pass.TypesInfo.Types[sel.X]; ok && tv.Type != nil {
			recvType = tv.Type
		} else if id, ok := sel.X.(*ast.Ident); ok {
			if obj := pass.TypesInfo.Uses[id]; obj != nil {
				recvType = obj.Type()
			}
		}

		if recvType != nil && recvType != types.Typ[types.Invalid] {
			if callsite.IsPgxOrSQLType(recvType) || isProvenDBPoolInterface(recvType) {
				return true
			}
			return false
		}
	}

	// 2. AST-level Symbol / Type Verification (pass == nil or unresolved)
	astType := findASTType(sel.X, fn, file)
	if astType != nil && isProvenDBPoolASTType(astType, file) {
		return true
	}

	// Check if assigned from a DB constructor (sql.Open, pgxpool.New, etc.)
	if id, ok := sel.X.(*ast.Ident); ok && id.Obj != nil {
		if as, ok := id.Obj.Decl.(*ast.AssignStmt); ok {
			for i, lhs := range as.Lhs {
				if lid, ok := lhs.(*ast.Ident); ok && lid.Name == id.Name {
					var rhs ast.Expr
					if i < len(as.Rhs) {
						rhs = as.Rhs[i]
					} else if len(as.Rhs) == 1 {
						rhs = as.Rhs[0]
					}
					if rhsCall, ok := rhs.(*ast.CallExpr); ok && isDBPoolConstructorCall(rhsCall) {
						return true
					}
				}
			}
		}
	}

	return false
}

func isProvenDBPoolInterface(t types.Type) bool {
	if t == nil {
		return false
	}
	t = unwrapPointer(t)

	var hasBeginWithTx, hasBeginFuncWithTx bool
	checkMethod := func(fn *types.Func) {
		sig, ok := fn.Type().(*types.Signature)
		if !ok {
			return
		}
		switch fn.Name() {
		case "Begin", "BeginTx":
			res := sig.Results()
			if res != nil && res.Len() >= 1 {
				for i := 0; i < res.Len(); i++ {
					if isProvenDBTxType(res.At(i).Type()) {
						hasBeginWithTx = true
					}
				}
			}
		case "BeginFunc":
			params := sig.Params()
			if params != nil && params.Len() >= 1 {
				for i := 0; i < params.Len(); i++ {
					if fnSig, ok := params.At(i).Type().(*types.Signature); ok {
						if fnSig.Params() != nil && fnSig.Params().Len() >= 1 {
							if isProvenDBTxType(fnSig.Params().At(0).Type()) {
								hasBeginFuncWithTx = true
							}
						}
					}
				}
			}
		}
	}

	if named, ok := t.(*types.Named); ok {
		for i := 0; i < named.NumMethods(); i++ {
			checkMethod(named.Method(i))
		}
	}
	if iface, ok := t.Underlying().(*types.Interface); ok {
		for i := 0; i < iface.NumMethods(); i++ {
			checkMethod(iface.Method(i))
		}
	}

	return hasBeginWithTx || hasBeginFuncWithTx
}

func isProvenDBTxType(t types.Type) bool {
	if t == nil {
		return false
	}
	t = unwrapPointer(t)
	if callsite.IsPgxOrSQLType(t) {
		return true
	}

	var hasExecOrQuery bool
	checkTxFunc := func(fn *types.Func) {
		switch fn.Name() {
		case "Exec", "ExecContext", "Query", "QueryRow", "SendBatch":
			hasExecOrQuery = true
		}
	}

	if named, ok := t.(*types.Named); ok {
		for i := 0; i < named.NumMethods(); i++ {
			checkTxFunc(named.Method(i))
		}
	}
	if iface, ok := t.Underlying().(*types.Interface); ok {
		for i := 0; i < iface.NumMethods(); i++ {
			checkTxFunc(iface.Method(i))
		}
	}

	return hasExecOrQuery
}

func isDBTxIdent(pass *analysis.Pass, id *ast.Ident, fn *ast.FuncDecl, file *ast.File) bool {
	if id == nil || id.Name == "_" {
		return false
	}
	if pass != nil && pass.TypesInfo != nil {
		t := pass.TypesInfo.TypeOf(id)
		if t != nil && t != types.Typ[types.Invalid] {
			return isProvenDBTxType(t)
		}
	}
	astType := findASTType(id, fn, file)
	if astType != nil {
		return isProvenDBTxASTType(astType, file)
	}
	return true
}

func isBeginTxCall(pass *analysis.Pass, call *ast.CallExpr, fn *ast.FuncDecl, file *ast.File) bool {
	if call == nil {
		return false
	}
	name := callsite.GetCallMethodName(call.Fun)
	if name == "Begin" || name == "BeginTx" {
		return isDBReceiver(pass, call.Fun, fn, file)
	}
	return false
}

func getTxEndIdent(expr ast.Expr) *ast.Ident {
	if call, ok := expr.(*ast.CallExpr); ok {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if sel.Sel.Name == "Commit" || sel.Sel.Name == "Rollback" {
				if id, ok := sel.X.(*ast.Ident); ok {
					return id
				}
			}
		}
	}
	return nil
}
