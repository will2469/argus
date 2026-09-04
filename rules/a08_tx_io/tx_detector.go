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
			if isProvenDBPoolInterface(recvType) {
				return true
			}
			if !hasInvalidType(recvType) {
				return false
			}
		}
	}

	// 2. AST-level fallback (pass == nil or unresolved type)
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
					if rhsCall, ok := rhs.(*ast.CallExpr); ok && isDBPoolConstructorCall(rhsCall, file) {
						return true
					}
				}
			}
		}
	}

	return false
}

func isDBTxIdent(pass *analysis.Pass, id *ast.Ident, fn *ast.FuncDecl, file *ast.File) bool {
	if id == nil || id.Name == "_" {
		return false
	}
	if pass != nil && pass.TypesInfo != nil {
		t := pass.TypesInfo.TypeOf(id)
		if t != nil && t != types.Typ[types.Invalid] {
			if isProvenDBTxType(t) {
				return true
			}
			if !hasInvalidType(t) {
				return false
			}
		}
	}
	// AST-level fallback: check if type is sql.Tx, pgx.Tx, or proven AST Tx interface
	astType := findASTType(id, fn, file)
	if astType != nil {
		return isProvenDBTxASTType(astType, file)
	}
	// Fail-closed against false positives: unresolved identifiers cannot be assumed as DB transactions
	return false
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
