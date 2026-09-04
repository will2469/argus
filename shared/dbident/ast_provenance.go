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
// must have Commit + Rollback + at least one exec/query method.
func HasASTTxMethods(iface *ast.InterfaceType) bool {
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
			case "Exec", "ExecContext", "Query", "QueryRow",
				"QueryContext", "QueryRowContext", "SendBatch":
				hasExecOrQuery = true
			}
		}
	}
	return hasCommit && hasRollback && hasExecOrQuery
}
