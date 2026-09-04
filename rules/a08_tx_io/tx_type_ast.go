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
	if isKnownDBPoolASTType(expr) {
		return true
	}
	typeName := getASTTypeName(expr)
	if typeName == "" || file == nil {
		return false
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != typeName {
				continue
			}
			if iface, ok := ts.Type.(*ast.InterfaceType); ok && iface.Methods != nil {
				var hasBegin bool
				for _, m := range iface.Methods.List {
					for _, name := range m.Names {
						if name.Name == "Begin" || name.Name == "BeginFunc" {
							hasBegin = true
						}
					}
				}
				return hasBegin
			}
		}
	}
	return false
}

func isProvenDBTxASTType(expr ast.Expr, file *ast.File) bool {
	if isKnownDBTxASTType(expr) {
		return true
	}
	typeName := getASTTypeName(expr)
	if typeName == "" || file == nil {
		return false
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != typeName {
				continue
			}
			if iface, ok := ts.Type.(*ast.InterfaceType); ok && iface.Methods != nil {
				var hasExec, hasCommit bool
				for _, m := range iface.Methods.List {
					for _, name := range m.Names {
						if name.Name == "Exec" || name.Name == "Query" {
							hasExec = true
						}
						if name.Name == "Commit" || name.Name == "Rollback" {
							hasCommit = true
						}
					}
				}
				return hasExec && hasCommit
			}
		}
	}
	return false
}
