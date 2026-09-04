// Package a07_error_leak provides semantic recognition of database callsites
// based strictly on compiler types, interface method sets, and AST declarations,
// with zero reliance on fragile receiver variable naming heuristics.
package a07_error_leak

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/dbident"
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
			return dbident.IsDBConstructorMethod(methodName)
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
				if f.Pkg() != nil && dbident.IsKnownDBPackagePath(f.Pkg().Path()) {
					return true
				}
				if sig, ok := f.Type().(*types.Signature); ok && sig.Recv() != nil {
					if dbident.IsKnownDBDriverType(sig.Recv().Type()) {
						return true
					}
				}
			}
			recvType := selType.Recv()
			if recvType != nil && recvType != types.Typ[types.Invalid] {
				recvType = dbident.UnwrapPointer(recvType)
				if dbident.IsKnownDBDriverType(recvType) {
					return true
				}
				if isQuery && dbident.IsProvenDBQuerierType(recvType) {
					return true
				}
				if !dbident.HasInvalidType(recvType) {
					return false
				}
			}
		} else if tv, ok := pass.TypesInfo.Types[sel.X]; ok && tv.Type != nil {
			recvType := dbident.UnwrapPointer(tv.Type)
			if dbident.IsKnownDBDriverType(recvType) {
				return true
			}
			if isQuery && dbident.IsProvenDBQuerierType(recvType) {
				return true
			}
			if !dbident.HasInvalidType(recvType) {
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
				recvType = dbident.UnwrapPointer(recvType)
				if dbident.IsKnownDBDriverType(recvType) {
					return true
				}
				if isQuery && dbident.IsProvenDBQuerierType(recvType) {
					return true
				}
				if !dbident.HasInvalidType(recvType) {
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

