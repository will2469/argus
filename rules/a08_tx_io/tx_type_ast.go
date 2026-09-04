// Package a08_tx_io provides AST-level database pool and transaction interface verification.
package a08_tx_io

import (
	"go/ast"

	"github.com/will2469/argus/shared/callsite"
)

func isKnownDBPoolASTType(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if pkgID, ok := sel.X.(*ast.Ident); ok {
			switch pkgID.Name {
			case "sql", "pgx", "pgxpool", "sqlx", "pq":
				switch sel.Sel.Name {
				case "DB", "Pool", "Conn":
					return true
				}
			}
		}
	}
	return false
}

func isKnownDBTxASTType(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if pkgID, ok := sel.X.(*ast.Ident); ok {
			switch pkgID.Name {
			case "sql", "pgx", "pgxpool", "sqlx", "pq":
				if sel.Sel.Name == "Tx" {
					return true
				}
			}
		}
	}
	return false
}

func isDBPoolConstructorCall(call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	sel := callsite.GetCallSelector(call.Fun)
	if sel == nil {
		return false
	}
	if pkgID, ok := sel.X.(*ast.Ident); ok {
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
	// Provenance-based: accept known DB pool types with exact package selectors
	if isKnownDBPoolASTType(expr) {
		return true
	}

	// Heuristic fallback: check if this is a locally-defined type with transaction-like interface
	// This is necessary for detecting custom abstractions and test fixtures
	typeName := getASTTypeName(expr)
	if typeName != "" && file != nil {
		// Check if type is locally defined interface with Begin/BeginFunc methods
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok {
				for _, spec := range gen.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == typeName {
						if iface, ok := ts.Type.(*ast.InterfaceType); ok {
							if hasASTTransactionMethods(iface) {
								return true
							}
						}
					}
				}
			}
		}
	}

	return false
}

func hasASTTransactionMethods(iface *ast.InterfaceType) bool {
	if iface == nil || iface.Methods == nil {
		return false
	}

	// Check if interface has transaction pool methods (Begin, BeginFunc, etc.)
	for _, method := range iface.Methods.List {
		for _, name := range method.Names {
			switch name.Name {
			case "Begin", "BeginFunc", "BeginTx", "ExecuteTx", "WithTx":
				return true
			}
		}
	}
	return false
}

func isProvenDBTxASTType(expr ast.Expr, file *ast.File) bool {
	// Provenance-based: accept known DB transaction types with exact package selectors
	if isKnownDBTxASTType(expr) {
		return true
	}

	// Heuristic fallback: check if this is a locally-defined transaction-like interface
	typeName := getASTTypeName(expr)
	if typeName != "" && file != nil {
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok {
				for _, spec := range gen.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == typeName {
						if iface, ok := ts.Type.(*ast.InterfaceType); ok {
							if hasASTTxMethods(iface) {
								return true
							}
						}
					}
				}
			}
		}
	}

	return false
}

func hasASTTxMethods(iface *ast.InterfaceType) bool {
	if iface == nil || iface.Methods == nil {
		return false
	}

	// Check if interface has transaction methods (Exec, Commit, Rollback)
	hasExec := false
	hasCommit := false
	for _, method := range iface.Methods.List {
		for _, name := range method.Names {
			switch name.Name {
			case "Exec", "Query", "QueryRow":
				hasExec = true
			case "Commit", "Rollback":
				hasCommit = true
			}
		}
	}

	return hasExec && hasCommit
}
