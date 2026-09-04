// Package a07_error_leak provides semantic recognition of database callsites
// based strictly on compiler types, interface method sets, and AST declarations,
// with zero reliance on fragile receiver variable naming heuristics.
package a07_error_leak

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
)

// isDatabaseCall determines whether a call expression is an operation on a genuine database connection,
// pool, transaction, or driver package.
func isDatabaseCall(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl, call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	sel := callsite.GetCallSelector(call.Fun)
	if sel == nil {
		return false
	}
	methodName := sel.Sel.Name

	// Package-level calls (e.g. sql.Open, pgx.Connect)
	if id, ok := sel.X.(*ast.Ident); ok {
		if isKnownDBPackageIdent(pass, file, fn, id) {
			return isDBConstructorMethod(methodName)
		}
	}

	isQuery := callsite.IsDBQueryMethod(methodName)
	isAux := isAuxiliaryDBMethod(methodName)
	if !isQuery && !isAux {
		return false
	}

	// 1. Semantic Type Verification via pass.TypesInfo
	if pass != nil && pass.TypesInfo != nil {
		if selType, ok := pass.TypesInfo.Selections[sel]; ok {
			if f, ok := selType.Obj().(*types.Func); ok {
				if f.Pkg() != nil && isKnownDBPackagePath(f.Pkg().Path()) {
					return true
				}
				if sig, ok := f.Type().(*types.Signature); ok && sig.Recv() != nil {
					if isKnownDBDriverType(sig.Recv().Type()) {
						return true
					}
				}
			}
			recvType := selType.Recv()
			if recvType != nil && recvType != types.Typ[types.Invalid] {
				recvType = unwrapPointer(recvType)
				if isKnownDBDriverType(recvType) {
					return true
				}
				if isQuery && isProvenDBQuerierType(recvType) {
					return true
				}
				if isAux {
					if f, ok := selType.Obj().(*types.Func); ok && isDBMethodWithDriverSignature(f) {
						return true
					}
				}
				if !hasInvalidType(recvType) {
					return false
				}
			}
		} else if tv, ok := pass.TypesInfo.Types[sel.X]; ok && tv.Type != nil {
			recvType := unwrapPointer(tv.Type)
			if isKnownDBDriverType(recvType) {
				return true
			}
			if isQuery && isProvenDBQuerierType(recvType) {
				return true
			}
			if !hasInvalidType(recvType) {
				return false
			}
		} else if id, ok := sel.X.(*ast.Ident); ok {
			var recvType types.Type
			if obj := pass.TypesInfo.Uses[id]; obj != nil {
				recvType = obj.Type()
			} else if obj := pass.TypesInfo.Defs[id]; obj != nil {
				recvType = obj.Type()
			}
			if recvType != nil && recvType != types.Typ[types.Invalid] {
				recvType = unwrapPointer(recvType)
				if isKnownDBDriverType(recvType) {
					return true
				}
				if isQuery && isProvenDBQuerierType(recvType) {
					return true
				}
				if !hasInvalidType(recvType) {
					return false
				}
			}
		}
		return false
	}

	// 2. Standalone / AST Mode (pass == nil or TypesInfo unavailable)
	astType := findASTType(sel.X, fn, file)
	if astType != nil {
		if isKnownDBDriverASTType(astType, file, fn) {
			return true
		}
		if isQuery && isProvenDBQuerierASTType(astType, file, fn) {
			return true
		}
	}

	if isAssignedFromDBConstructor(pass, file, fn, sel.X) {
		return true
	}

	return false
}

func isAuxiliaryDBMethod(methodName string) bool {
	switch methodName {
	case "Commit", "Rollback", "Scan", "Err", "Ping", "PingContext",
		"Close", "Prepare", "PrepareContext", "CopyFrom":
		return true
	}
	return false
}

func isDBConstructorMethod(methodName string) bool {
	switch methodName {
	case "Open", "OpenDB", "Connect", "ConnectConfig", "New", "NewWithConfig":
		return true
	}
	return false
}

func isKnownDBPackagePath(path string) bool {
	switch path {
	case "database/sql", "github.com/jackc/pgx/v5", "github.com/jackc/pgx/v5/pgxpool",
		"github.com/jackc/pgx/v5/pgconn", "github.com/jackc/pgx/v4", "github.com/jackc/pgx/v4/pgxpool",
		"github.com/jmoiron/sqlx", "github.com/lib/pq":
		return true
	}
	return false
}

func isKnownDBDriverType(t types.Type) bool {
	if t == nil {
		return false
	}
	t = unwrapPointer(t)
	if named, ok := t.(*types.Named); ok {
		if obj := named.Obj(); obj != nil && obj.Pkg() != nil {
			if isKnownDBPackagePath(obj.Pkg().Path()) {
				switch obj.Name() {
				case "DB", "Tx", "Conn", "Pool", "Batch", "BatchResults", "Stmt", "Rows", "Row", "Result", "CommandTag":
					return true
				}
			}
		}
	}
	return false
}

func isProvenDBQuerierType(t types.Type) bool {
	if t == nil {
		return false
	}
	t = unwrapPointer(t)
	if isKnownDBDriverType(t) {
		return true
	}
	if iface, ok := t.Underlying().(*types.Interface); ok {
		for i := 0; i < iface.NumMethods(); i++ {
			if isDBMethodWithDriverSignature(iface.Method(i)) {
				return true
			}
		}
	}
	return false
}

func isDBMethodWithDriverSignature(fn *types.Func) bool {
	if fn == nil {
		return false
	}
	switch fn.Name() {
	case "Query", "QueryRow", "Exec", "ExecContext", "Begin", "BeginTx", "SendBatch":
	default:
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return false
	}
	if results := sig.Results(); results != nil {
		for i := 0; i < results.Len(); i++ {
			if isKnownDBDriverType(results.At(i).Type()) {
				return true
			}
		}
	}
	if params := sig.Params(); params != nil {
		for i := 0; i < params.Len(); i++ {
			if isKnownDBDriverType(params.At(i).Type()) {
				return true
			}
		}
	}
	return false
}

func unwrapPointer(t types.Type) types.Type {
	for {
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		} else {
			break
		}
	}
	return t
}

func hasInvalidType(t types.Type) bool {
	if t == nil {
		return true
	}
	switch x := t.(type) {
	case *types.Basic:
		return x.Kind() == types.Invalid
	case *types.Pointer:
		return hasInvalidType(x.Elem())
	case *types.Named:
		return hasInvalidType(x.Underlying())
	}
	return false
}
