// Package a14_select_star identifies database callsites for SELECT * auditing,
// based strictly on compiler types, interface method sets, and AST declarations,
// with zero reliance on fragile receiver variable naming heuristics.
package a14_select_star

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/dbident"
)

// isDatabaseCall verifies that a call expression targets a genuine database querier.
func isDatabaseCall(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	sel := callsite.GetCallSelector(call.Fun)
	if sel == nil || !callsite.IsDBQueryMethod(sel.Sel.Name) {
		return false
	}

	// 1. Semantic Type Resolution via pass.TypesInfo
	if pass != nil && pass.TypesInfo != nil {
		// Package-level calls from database packages: sql.Open, pgx.Connect, etc.
		if id, ok := sel.X.(*ast.Ident); ok {
			if pkgName, ok := pass.TypesInfo.Uses[id].(*types.PkgName); ok {
				if dbident.IsKnownDBPackagePath(pkgName.Imported().Path()) {
					return true
				}
			}
		}

		var recvType types.Type
		if selType, ok := pass.TypesInfo.Selections[sel]; ok {
			if fn, ok := selType.Obj().(*types.Func); ok {
				if fn.Pkg() != nil && dbident.IsKnownDBPackagePath(fn.Pkg().Path()) {
					return true
				}
				if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
					if dbident.IsKnownDBDriverType(sig.Recv().Type()) {
						return true
					}
				}
			}
			if selType.Recv() != nil {
				recvType = selType.Recv()
			}
		} else if tv, ok := pass.TypesInfo.Types[sel.X]; ok && tv.Type != nil {
			recvType = tv.Type
		} else if id, ok := sel.X.(*ast.Ident); ok {
			if obj := pass.TypesInfo.Uses[id]; obj != nil {
				recvType = obj.Type()
			} else if obj := pass.TypesInfo.Defs[id]; obj != nil {
				recvType = obj.Type()
			}
		}

		if recvType != nil && recvType != types.Typ[types.Invalid] {
			recvType = dbident.UnwrapPointer(recvType)
			if dbident.IsKnownDBDriverType(recvType) {
				return true
			}
			if pass.Pkg != nil && dbident.IsProvenDBQuerierWithPkg(recvType, pass.Pkg) {
				return true
			}
			if dbident.IsProvenDBQuerierType(recvType) {
				return true
			}
			// Compiler has complete type info: if not proven DB, fail closed.
			// Only fall through if types could not be fully resolved (contain Invalid).
			if !dbident.HasInvalidType(recvType) {
				return false
			}
		}
	}

	// 2. Standalone Mode (pass == nil or TypesInfo unavailable)
	var fn *ast.FuncDecl
	if file != nil {
		fn = findEnclosingFunc(file, call.Pos())
	}
	if fn != nil && file != nil && dbident.IsAssignedFromNonDBConstructor(sel.X, fn, file) {
		return false
	}
	if id, ok := sel.X.(*ast.Ident); ok {
		if file != nil && dbident.IsImportedDBPackageIdent(file, id.Name) {
			return true
		}
	}

	astType := findASTType(sel.X, fn, file)
	if astType != nil && isProvenDBASTType(astType, file) {
		return true
	}

	if isAssignedFromDBConstructor(sel.X, fn, file) {
		return true
	}

	return false
}
