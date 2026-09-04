// Package a10_isolation_level provides AST-level type and constructor resolution
// for environments where type checker information is absent or partial.
package a10_isolation_level

import (
	"go/ast"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/dbident"
)

func findASTType(expr ast.Expr, fn *ast.FuncDecl, file *ast.File) ast.Expr {
	if expr == nil {
		return nil
	}

	if id, ok := expr.(*ast.Ident); ok {
		if fn != nil && fn.Type != nil && fn.Type.Params != nil {
			for _, field := range fn.Type.Params.List {
				for _, name := range field.Names {
					if name.Name == id.Name {
						return field.Type
					}
				}
			}
		}
		if fn != nil && fn.Recv != nil {
			for _, field := range fn.Recv.List {
				for _, name := range field.Names {
					if name.Name == id.Name {
						return field.Type
					}
				}
			}
		}
	}

	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if xID, ok := sel.X.(*ast.Ident); ok {
			xType := findASTType(xID, fn, file)
			structName := getASTTypeName(xType)
			if structName != "" && file != nil {
				for _, decl := range file.Decls {
					if gen, ok := decl.(*ast.GenDecl); ok {
						for _, spec := range gen.Specs {
							if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == structName {
								if st, ok := ts.Type.(*ast.StructType); ok && st.Fields != nil {
									for _, f := range st.Fields.List {
										for _, fnm := range f.Names {
											if fnm.Name == sel.Sel.Name {
												return f.Type
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return nil
}

func isProvenDBPoolASTType(expr ast.Expr, file *ast.File) bool {
	return dbident.IsProvenDBPoolASTType(expr, file)
}

func isProvenClosureTxASTType(expr ast.Expr, file *ast.File) bool {
	return dbident.IsProvenClosureTxASTType(expr, file)
}

func isDBPoolConstructorCall(call *ast.CallExpr, file *ast.File) bool {
	if call == nil || file == nil {
		return false
	}
	sel := callsite.GetCallSelector(call.Fun)
	if sel == nil {
		return false
	}
	if pkgID, ok := sel.X.(*ast.Ident); ok {
		path := getPackageImportPath(pkgID.Name, file)
		switch path {
		case "database/sql", "github.com/jmoiron/sqlx":
			return sel.Sel.Name == "Open" || sel.Sel.Name == "OpenDB" || sel.Sel.Name == "Connect"
		case "github.com/jackc/pgx/v5", "github.com/jackc/pgx/v4":
			return sel.Sel.Name == "Connect" || sel.Sel.Name == "ConnectConfig"
		case "github.com/jackc/pgx/v5/pgxpool", "github.com/jackc/pgx/v4/pgxpool":
			return sel.Sel.Name == "New" || sel.Sel.Name == "NewWithConfig"
		}
	}
	return false
}
