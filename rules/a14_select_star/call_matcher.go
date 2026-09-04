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
		if selType, ok := pass.TypesInfo.Selections[sel]; ok && selType.Recv() != nil {
			recvType = selType.Recv()
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
			if callsite.IsPgxOrSQLType(recvType) {
				return true
			}
			if isProvenDBQuerierType(recvType) {
				return true
			}
			if isStructWithDBField(recvType) {
				return true
			}
			// Compiler has complete type info: if not proven DB, fail closed
			return false
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

func isKnownDBPackagePath(path string) bool {
	switch path {
	case "database/sql", "github.com/jackc/pgx/v5", "github.com/jackc/pgx/v5/pgxpool",
		"github.com/jackc/pgx/v5/pgconn", "github.com/jackc/pgx/v4", "github.com/jackc/pgx/v4/pgxpool",
		"github.com/jmoiron/sqlx", "github.com/lib/pq":
		return true
	}
	return false
}

func isProvenDBQuerierType(t types.Type) bool {
	if t == nil {
		return false
	}
	t = unwrapPointer(t)

	var hasQuery, hasExec, hasQueryRow bool
	checkFunc := func(fn *types.Func) {
		switch fn.Name() {
		case "Query":
			hasQuery = true
		case "QueryRow":
			hasQueryRow = true
		case "Exec", "ExecContext", "Begin", "BeginTx", "SendBatch":
			hasExec = true
		}
	}

	if named, ok := t.(*types.Named); ok {
		for i := 0; i < named.NumMethods(); i++ {
			checkFunc(named.Method(i))
		}
		typeName := named.Obj().Name()
		if typeName == "DB" || typeName == "Querier" || typeName == "DBTX" || typeName == "Tx" || typeName == "Pool" {
			return hasQuery || hasQueryRow || hasExec
		}
	}

	if iface, ok := t.Underlying().(*types.Interface); ok {
		for i := 0; i < iface.NumMethods(); i++ {
			checkFunc(iface.Method(i))
		}
	}

	return (hasQuery && hasExec) || (hasQuery && hasQueryRow)
}

func isStructWithDBField(t types.Type) bool {
	if t == nil {
		return false
	}
	t = unwrapPointer(t)
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for i := 0; i < st.NumFields(); i++ {
		fType := unwrapPointer(st.Field(i).Type())
		if callsite.IsPgxOrSQLType(fType) || isProvenDBQuerierType(fType) {
			return true
		}
	}
	return false
}

