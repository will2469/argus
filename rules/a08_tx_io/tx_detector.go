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

	// 2. AST-level fallback (pass == nil or unresolved type): heuristic package selector matching
	// Accept known database package selectors (sql.DB, pgxpool.Pool, etc.)
	// This is a heuristic and may produce false negatives for wrapped abstractions
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

	// Provenance-based: only accept exact package path matches
	if named, ok := t.(*types.Named); ok && named.Obj() != nil {
		pkg := named.Obj().Pkg()
		if pkg == nil {
			return false
		}
		path := pkg.Path()
		name := named.Obj().Name()

		// Exact package/type matches only
		switch path {
		case "github.com/jackc/pgx/v5/pgxpool":
			return name == "Pool"
		case "github.com/jackc/pgx/v4/pgxpool":
			return name == "Pool"
		case "database/sql":
			return name == "DB"
		}
	}

	// Heuristic fallback: for unknown packages, check if interface has transaction-like methods
	// This is less strict but necessary for detecting wrapped abstractions
	return hasTransactionMethods(t)
}

func hasTransactionMethods(t types.Type) bool {
	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return false
	}

	// Check if interface has Begin/BeginFunc/BeginTx methods (transaction pool indicators)
	hasBegin := false
	for i := 0; i < iface.NumMethods(); i++ {
		method := iface.Method(i)
		switch method.Name() {
		case "Begin", "BeginFunc", "BeginTx", "ExecuteTx", "WithTx":
			hasBegin = true
		}
	}

	return hasBegin
}

func isProvenDBTxType(t types.Type) bool {
	if t == nil {
		return false
	}
	t = unwrapPointer(t)

	// Fail-closed: only accept exact type verification via callsite.IsPgxOrSQLType
	// Reject structural method set analysis (Begin, Exec, Query) as insufficient proof
	return callsite.IsPgxOrSQLType(t)
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
	// AST-level fallback: check if type is sql.Tx, pgx.Tx, etc.
	astType := findASTType(id, fn, file)
	if astType != nil {
		return isProvenDBTxASTType(astType, file)
	}
	// Conservative: accept as transaction if unresolved (may cause false positives,
	// but prevents false negatives in critical blocking I/O detection)
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
