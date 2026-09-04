// Package dbident provides AST-level database identity verification for
// standalone runner mode (pass == nil) where type-checker information
// is unavailable. All AST checks anchor through file import declarations.
package dbident

import (
	"go/ast"
	"strings"
)

// IsImportedDBPackageIdent checks whether pkgName refers to a known
// database driver package imported in file. Handles aliased imports.
func IsImportedDBPackageIdent(file *ast.File, pkgName string) bool {
	if file == nil || pkgName == "" {
		return false
	}
	for _, imp := range file.Imports {
		if imp.Path == nil {
			continue
		}
		path := strings.Trim(imp.Path.Value, `"`)
		if !IsKnownDBPackagePath(path) {
			continue
		}
		localName := DefaultPackageName(path)
		if imp.Name != nil {
			localName = imp.Name.Name
		}
		if localName == pkgName {
			return true
		}
	}
	return false
}

// IsKnownDBPoolASTType checks whether an AST type expression refers to a
// known database pool type (sql.DB, pgxpool.Pool, pgx.Conn) by verifying
// the package qualifier against file imports.
func IsKnownDBPoolASTType(expr ast.Expr, file *ast.File) bool {
	if expr == nil {
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
	switch sel.Sel.Name {
	case "DB", "Pool", "Conn":
		return true
	}
	return false
}

// IsKnownDBTxASTType checks whether an AST type expression refers to a
// known database transaction type (sql.Tx, pgx.Tx) by verifying the
// package qualifier against file imports.
func IsKnownDBTxASTType(expr ast.Expr, file *ast.File) bool {
	if expr == nil {
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
	return IsImportedDBPackageIdent(file, pkgID.Name) && sel.Sel.Name == "Tx"
}

// IsKnownDBDriverASTType checks whether an AST type expression refers to a
// known database driver type by verifying the package qualifier against file imports.
func IsKnownDBDriverASTType(expr ast.Expr, file *ast.File) bool {
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

// IsASTErrorType checks whether an AST expression is the built-in "error" type.
func IsASTErrorType(expr ast.Expr) bool {
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name == "error"
	}
	return false
}

// IsASTExecOrQueryMethod checks whether an interface method signature represents
// a genuine database Exec or Query method: requires 2 return values with an imported
// database driver type as the first return value and terminal error as the second.
func IsASTExecOrQueryMethod(m *ast.Field, file *ast.File) bool {
	if m == nil || file == nil {
		return false
	}
	ft, ok := m.Type.(*ast.FuncType)
	if !ok || ft.Results == nil || len(ft.Results.List) != 2 {
		return false
	}
	if !IsASTErrorType(ft.Results.List[1].Type) {
		return false
	}
	return IsKnownDBDriverASTType(ft.Results.List[0].Type, file)
}

// IsProvenDBPoolASTType checks whether expr is a proven DB pool type at
// the AST level: either a direct known type (sql.DB), or a local
// interface type whose methods trace back to proven DB types.
func IsProvenDBPoolASTType(expr ast.Expr, file *ast.File) bool {
	if IsKnownDBPoolASTType(expr, file) {
		return true
	}

	typeName := GetASTTypeName(expr)
	if typeName == "" || file == nil {
		return false
	}
	ts := FindTypeSpec(typeName, file)
	if ts == nil {
		return false
	}
	iface, ok := ts.Type.(*ast.InterfaceType)
	if !ok {
		return false
	}
	return HasASTProvenDBPoolMethods(iface, file)
}

// IsProvenDBTxASTType checks whether expr is a proven DB transaction type
// at the AST level: either a direct known type (sql.Tx), or a local
// interface with Commit+Rollback+ExecOrQuery methods.
func IsProvenDBTxASTType(expr ast.Expr, file *ast.File) bool {
	if IsKnownDBTxASTType(expr, file) {
		return true
	}

	typeName := GetASTTypeName(expr)
	if typeName == "" || file == nil {
		return false
	}
	ts := FindTypeSpec(typeName, file)
	if ts == nil {
		return false
	}
	iface, ok := ts.Type.(*ast.InterfaceType)
	if !ok {
		return false
	}
	return HasASTTxMethods(iface, file)
}

// IsProvenClosureTxASTType checks whether expr is a suitable transaction
// type for closure-based transaction APIs at the AST level.
func IsProvenClosureTxASTType(expr ast.Expr, file *ast.File) bool {
	return IsProvenDBTxASTType(expr, file)
}

// IsDBPoolConstructorCall checks whether call is a known database pool
// constructor invocation (sql.Open, pgxpool.New, pgx.Connect, etc.)
// by verifying the package qualifier against file imports.
func IsDBPoolConstructorCall(call *ast.CallExpr, file *ast.File) bool {
	if call == nil || file == nil {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkgID, ok := sel.X.(*ast.Ident)
	if !ok || !IsImportedDBPackageIdent(file, pkgID.Name) {
		return false
	}
	path := resolveImportPath(file, pkgID.Name)
	switch path {
	case "database/sql", "github.com/jmoiron/sqlx":
		return sel.Sel.Name == "Open" || sel.Sel.Name == "OpenDB" || sel.Sel.Name == "Connect"
	case "github.com/jackc/pgx/v5", "github.com/jackc/pgx/v4":
		return sel.Sel.Name == "Connect" || sel.Sel.Name == "ConnectConfig"
	case "github.com/jackc/pgx/v5/pgxpool", "github.com/jackc/pgx/v4/pgxpool":
		return sel.Sel.Name == "New" || sel.Sel.Name == "NewWithConfig"
	}
	return false
}

