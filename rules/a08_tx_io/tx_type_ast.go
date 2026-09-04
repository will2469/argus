// Package a08_tx_io delegates AST-level database type verification
// to the shared dbident package.
package a08_tx_io

import (
	"go/ast"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/dbident"
)



func isDBPoolConstructorCall(call *ast.CallExpr, file *ast.File) bool {
	if call == nil {
		return false
	}
	sel := callsite.GetCallSelector(call.Fun)
	if sel == nil {
		return false
	}
	pkgID, ok := sel.X.(*ast.Ident)
	if !ok || !dbident.IsImportedDBPackageIdent(file, pkgID.Name) {
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
	return false
}

func isProvenDBPoolASTType(expr ast.Expr, file *ast.File) bool {
	return dbident.IsProvenDBPoolASTType(expr, file)
}

func isProvenDBTxASTType(expr ast.Expr, file *ast.File) bool {
	return dbident.IsProvenDBTxASTType(expr, file)
}

func getASTTypeName(expr ast.Expr) string {
	return dbident.GetASTTypeName(expr)
}

