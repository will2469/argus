// Package dbident provides AST-level interface contract inspection for
// transaction and pool semantics in standalone runner mode.
package dbident

import (
	"go/ast"
)

// HasASTProvenDBPoolMethods checks an AST interface for pool semantics:
// Begin/BeginTx returning proven Tx, or BeginFunc/WithTx/ExecuteTx
// accepting a callback with proven Tx parameter.
func HasASTProvenDBPoolMethods(iface *ast.InterfaceType, file *ast.File) bool {
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
						if IsProvenDBTxASTType(res.Type, file) {
							return true
						}
					}
				}
			case "BeginFunc", "WithTx", "ExecuteTx":
				if ft.Params != nil {
					for _, param := range ft.Params.List {
						if cb, ok := param.Type.(*ast.FuncType); ok && cb.Params != nil && len(cb.Params.List) > 0 {
							for _, cbParam := range cb.Params.List {
								if IsProvenClosureTxASTType(cbParam.Type, file) {
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

// HasASTTxMethods checks an AST interface for transaction semantics:
// must have Commit + Rollback + at least one exec/query method, AND
// at least one method signature referencing an imported database driver type.
func HasASTTxMethods(iface *ast.InterfaceType, file *ast.File) bool {
	if iface == nil || iface.Methods == nil {
		return false
	}
	if file != nil && !HasDatabaseImports(file) {
		return false
	}

	hasCommit := false
	hasRollback := false
	hasExecOrQuery := false
	hasProvenance := false

	for _, method := range iface.Methods.List {
		for _, name := range method.Names {
			switch name.Name {
			case "Commit":
				hasCommit = true
			case "Rollback":
				hasRollback = true
			case "Exec", "ExecContext", "Query", "QueryRow",
				"QueryContext", "QueryRowContext", "SendBatch":
				hasExecOrQuery = true
			}
		}
		if ft, ok := method.Type.(*ast.FuncType); ok && file != nil {
			if astFuncHasDriverProvenance(ft, file) {
				hasProvenance = true
			}
		}
	}
	return hasProvenance && hasCommit && hasRollback && hasExecOrQuery
}

func astFuncHasDriverProvenance(ft *ast.FuncType, file *ast.File) bool {
	if ft == nil || file == nil {
		return false
	}
	if ft.Results != nil {
		for _, res := range ft.Results.List {
			if isASTKnownDriverType(res.Type, file) {
				return true
			}
		}
	}
	if ft.Params != nil {
		for _, p := range ft.Params.List {
			if isASTKnownDriverType(p.Type, file) {
				return true
			}
		}
	}
	return false
}

func isASTKnownDriverType(expr ast.Expr, file *ast.File) bool {
	if expr == nil || file == nil {
		return false
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkgID, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	if !IsImportedDBPackageIdent(file, pkgID.Name) {
		return false
	}
	return knownDBDriverTypeNames[sel.Sel.Name]
}
