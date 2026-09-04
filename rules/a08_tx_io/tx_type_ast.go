// Package a08_tx_io provides AST-level database pool and transaction interface verification.
package a08_tx_io

import (
	"go/ast"

	"github.com/will2469/argus/shared/callsite"
)

func isKnownDBPoolASTType(expr ast.Expr, file *ast.File) bool {
	if expr == nil {
		return false
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if pkgID, ok := sel.X.(*ast.Ident); ok {
			if isImportedDBPackageIdent(file, pkgID.Name) {
				switch sel.Sel.Name {
				case "DB", "Pool", "Conn":
					return true
				}
			}
		}
	}
	return false
}

func isKnownDBTxASTType(expr ast.Expr, file *ast.File) bool {
	if expr == nil {
		return false
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if pkgID, ok := sel.X.(*ast.Ident); ok {
			if isImportedDBPackageIdent(file, pkgID.Name) {
				if sel.Sel.Name == "Tx" {
					return true
				}
			}
		}
	}
	return false
}

func isDBPoolConstructorCall(call *ast.CallExpr, file *ast.File) bool {
	if call == nil {
		return false
	}
	sel := callsite.GetCallSelector(call.Fun)
	if sel == nil {
		return false
	}
	if pkgID, ok := sel.X.(*ast.Ident); ok {
		if !isImportedDBPackageIdent(file, pkgID.Name) {
			return false
		}
		switch pkgID.Name {
		case "sql":
			return sel.Sel.Name == "Open" || sel.Sel.Name == "OpenDB"
		case "pgx":
			return sel.Sel.Name == "Connect" || sel.Sel.Name == "ConnectConfig"
		case "pgxpool":
			return sel.Sel.Name == "New" || sel.Sel.Name == "NewWithConfig"
		case "sqlx":
			return sel.Sel.Name == "Open" || sel.Sel.Name == "Connect"
		}
	}
	return false
}

func isProvenDBPoolASTType(expr ast.Expr, file *ast.File) bool {
	if isKnownDBPoolASTType(expr, file) {
		return true
	}

	typeName := getASTTypeName(expr)
	if typeName != "" && file != nil {
		ts := findTypeSpec(typeName, file)
		if ts != nil {
			if iface, ok := ts.Type.(*ast.InterfaceType); ok {
				return hasASTProvenDBPoolMethods(iface, file)
			}
		}
	}

	return false
}

func hasASTProvenDBPoolMethods(iface *ast.InterfaceType, file *ast.File) bool {
	if iface == nil || iface.Methods == nil {
		return false
	}

	for _, method := range iface.Methods.List {
		ft, ok := method.Type.(*ast.FuncType)
		if !ok {
			continue
		}
		for _, name := range method.Names {
			switch name.Name {
			case "Begin", "BeginTx":
				if ft.Results != nil && len(ft.Results.List) > 0 {
					for _, res := range ft.Results.List {
						if isProvenDBTxASTType(res.Type, file) {
							return true
						}
					}
				}
			case "BeginFunc", "WithTx", "ExecuteTx":
				if ft.Params != nil {
					for _, param := range ft.Params.List {
						if cb, ok := param.Type.(*ast.FuncType); ok && cb.Params != nil && len(cb.Params.List) > 0 {
							for _, cbParam := range cb.Params.List {
								if isProvenDBTxASTType(cbParam.Type, file) {
									return true
								}
							}
						}
					}
				}
			}
		}
	}
	return false
}

func isProvenDBTxASTType(expr ast.Expr, file *ast.File) bool {
	if isKnownDBTxASTType(expr, file) {
		return true
	}

	typeName := getASTTypeName(expr)
	if typeName != "" && file != nil {
		ts := findTypeSpec(typeName, file)
		if ts != nil {
			if iface, ok := ts.Type.(*ast.InterfaceType); ok {
				return hasASTTxMethods(iface)
			}
		}
	}

	return false
}

func hasASTTxMethods(iface *ast.InterfaceType) bool {
	if iface == nil || iface.Methods == nil {
		return false
	}

	hasCommit := false
	hasRollback := false
	hasExecOrQuery := false
	for _, method := range iface.Methods.List {
		for _, name := range method.Names {
			switch name.Name {
			case "Commit":
				hasCommit = true
			case "Rollback":
				hasRollback = true
			case "Exec", "ExecContext", "Query", "QueryRow", "QueryContext", "QueryRowContext", "SendBatch":
				hasExecOrQuery = true
			}
		}
	}

	return hasCommit && hasRollback && hasExecOrQuery
}
