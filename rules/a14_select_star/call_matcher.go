// Package a14_select_star identifies database callsites for SELECT * auditing,
// based strictly on compiler types, interface method sets, and AST declarations,
// with zero reliance on fragile receiver variable naming heuristics.
package a14_select_star

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
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
				if isKnownDBPackagePath(pkgName.Imported().Path()) {
					return true
				}
			}
		}

		var recvType types.Type
		if selType, ok := pass.TypesInfo.Selections[sel]; ok {
			if fn, ok := selType.Obj().(*types.Func); ok {
				if fn.Pkg() != nil && isKnownDBPackagePath(fn.Pkg().Path()) {
					return true
				}
				if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
					if isKnownDBDriverType(sig.Recv().Type()) {
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
			recvType = unwrapPointer(recvType)
			if isKnownDBDriverType(recvType) {
				return true
			}
			if isProvenDBQuerierType(recvType) {
				return true
			}
			// Compiler has complete type info: if not proven DB, fail closed.
			// Only fall through if types could not be fully resolved (contain Invalid).
			if !hasInvalidType(recvType) {
				return false
			}
		}
	}

	// 2. Standalone Mode (pass == nil or TypesInfo unavailable)
	if id, ok := sel.X.(*ast.Ident); ok {
		switch id.Name {
		case "sql", "pgx", "pgxpool", "sqlx", "pq":
			return true
		}
	}

	var fn *ast.FuncDecl
	if file != nil {
		fn = findEnclosingFunc(file, call.Pos())
	}
	astType := findASTType(sel.X, fn, file)
	if astType != nil && isProvenDBASTType(astType, file) {
		return true
	}

	if isAssignedFromDBConstructor(sel.X, fn) {
		return true
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
	case *types.Struct:
		for i := 0; i < x.NumFields(); i++ {
			if hasInvalidType(x.Field(i).Type()) {
				return true
			}
		}
	case *types.Interface:
		for i := 0; i < x.NumMethods(); i++ {
			if hasInvalidType(x.Method(i).Type()) {
				return true
			}
		}
	case *types.Signature:
		if results := x.Results(); results != nil {
			for i := 0; i < results.Len(); i++ {
				if hasInvalidType(results.At(i).Type()) {
					return true
				}
			}
		}
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
				case "DB", "Tx", "Conn", "Pool", "Batch", "Stmt", "Rows", "Row", "Result", "CommandTag":
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

