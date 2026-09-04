// Package a10_isolation_level provides approved database package identifiers,
// shadowing inspection, and delegation to shared/dbident.
package a10_isolation_level

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/will2469/argus/shared/dbident"
)

func getPackageImportPath(pkgName string, file *ast.File) string {
	if file == nil || pkgName == "" {
		return ""
	}
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, "`\"")
		localName := dbident.DefaultPackageName(path)
		if imp.Name != nil {
			localName = imp.Name.Name
		}
		if localName == pkgName {
			return path
		}
	}
	return ""
}

func isKnownDBPackage(pkgName string, file *ast.File) bool {
	path := getPackageImportPath(pkgName, file)
	return dbident.IsKnownDBPackagePath(path)
}


func getASTTypeName(expr ast.Expr) string {
	return dbident.GetASTTypeName(expr)
}

func findTypeSpec(name string, file *ast.File) *ast.TypeSpec {
	return dbident.FindTypeSpec(name, file)
}

func isIdentShadowed(file *ast.File, fn *ast.FuncDecl, pos token.Pos, name string) bool {
	if file != nil {
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok && (gen.Tok == token.VAR || gen.Tok == token.CONST) {
				for _, spec := range gen.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for _, p := range vs.Names {
							if p.Name == name {
								return true
							}
						}
					}
				}
			}
		}
	}
	if fn == nil {
		return false
	}
	if fn.Type != nil && fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			for _, p := range field.Names {
				if p.Name == name {
					return true
				}
			}
		}
	}
	if fn.Body != nil {
		var shadowed bool
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if n == nil || n.Pos() >= pos {
				return false
			}
			switch s := n.(type) {
			case *ast.AssignStmt:
				if s.Tok == token.DEFINE {
					for _, lhs := range s.Lhs {
						if id, ok := lhs.(*ast.Ident); ok && id.Name == name {
							shadowed = true
							return false
						}
					}
				}
			case *ast.ValueSpec:
				for _, id := range s.Names {
					if id.Name == name {
						shadowed = true
						return false
					}
				}
			}
			return !shadowed
		})
		if shadowed {
			return true
		}
	}
	return false
}

