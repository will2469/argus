// Package a08_tx_io provides AST-level type and constructor resolution
// for environments where type checker information is absent or partial.
package a08_tx_io

import (
	"go/ast"
	"go/token"
	"strings"
)

func findASTType(expr ast.Expr, fn *ast.FuncDecl, file *ast.File) ast.Expr {
	if expr == nil {
		return nil
	}

	if id, ok := expr.(*ast.Ident); ok {
		if fn == nil && file != nil {
			fn = findEnclosingFuncDecl(file, id.Pos())
		}
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
		if fn != nil && fn.Body != nil {
			for _, stmt := range fn.Body.List {
				if decl, ok := stmt.(*ast.DeclStmt); ok {
					if gen, ok := decl.Decl.(*ast.GenDecl); ok {
						for _, spec := range gen.Specs {
							if vs, ok := spec.(*ast.ValueSpec); ok {
								for _, name := range vs.Names {
									if name.Name == id.Name {
										return vs.Type
									}
								}
							}
						}
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

func getASTTypeName(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	if idx, ok := expr.(*ast.IndexExpr); ok {
		return getASTTypeName(idx.X)
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		return sel.Sel.Name
	}
	return ""
}


func isImportedPackage(file *ast.File, pkgName, expectedPath string) bool {
	if file == nil {
		return false
	}
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if imp.Name != nil {
			if imp.Name.Name == pkgName && strings.HasSuffix(path, expectedPath) {
				return true
			}
		} else {
			parts := strings.Split(path, "/")
			lastPart := parts[len(parts)-1]
			if lastPart == pkgName && (path == expectedPath || strings.HasSuffix(path, "/"+expectedPath)) {
				return true
			}
		}
	}
	return false
}


func findEnclosingFuncDecl(file *ast.File, pos token.Pos) *ast.FuncDecl {
	if file == nil {
		return nil
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Pos() <= pos && pos <= fn.End() {
				return fn
			}
		}
	}
	return nil
}
