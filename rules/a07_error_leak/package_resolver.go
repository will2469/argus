// Package a07_error_leak provides semantic and AST-level package resolution,
// verifying genuine package imports while preventing identifier shadowing bugs.
package a07_error_leak

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/dbident"
)

// isPackageCall verifies that a selector call targets a specific package function or constructor.
// It rigorously validates compiler type info (types.PkgName) when available,
// and in AST mode verifies file imports without identifier shadowing.
func isPackageCall(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl, sel *ast.SelectorExpr, expectedPkgPath, expectedMethod string) bool {
	if sel == nil || sel.Sel == nil || sel.Sel.Name != expectedMethod {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return isPackageIdent(pass, file, fn, id, expectedPkgPath)
}

// isPackageIdent verifies that an identifier resolves to a specific package import.
func isPackageIdent(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl, id *ast.Ident, expectedPkgPath string) bool {
	if id == nil {
		return false
	}

	// 1. Semantic verification via pass.TypesInfo
	if pass != nil && pass.TypesInfo != nil {
		if obj := pass.TypesInfo.Uses[id]; obj != nil {
			if pkg, ok := obj.(*types.PkgName); ok && pkg.Imported() != nil {
				return pkg.Imported().Path() == expectedPkgPath
			}
			// If obj is *types.Var, *types.TypeName, *types.Const, etc. -> shadowed!
			return false
		}
		if obj := pass.TypesInfo.Defs[id]; obj != nil {
			return false
		}
		// In type-checked pass, if identifier cannot be proven as types.PkgName, fail closed.
		return false
	}

	// 2. Standalone / AST mode (pass == nil or TypesInfo unavailable)
	if file == nil {
		return false
	}

	// Check if id is shadowed in local function scope
	if isIdentShadowed(fn, id.Name, id.Pos()) {
		return false
	}

	return isImportedPackage(file, expectedPkgPath, id.Name)
}

// isKnownDBPackageIdent checks whether an identifier refers to any known database driver package.
func isKnownDBPackageIdent(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl, id *ast.Ident) bool {
	if id == nil {
		return false
	}

	if pass != nil && pass.TypesInfo != nil {
		if obj := pass.TypesInfo.Uses[id]; obj != nil {
			if pkg, ok := obj.(*types.PkgName); ok && pkg.Imported() != nil {
				return dbident.IsKnownDBPackagePath(pkg.Imported().Path())
			}
		}
		return false
	}

	if file == nil || isIdentShadowed(fn, id.Name, id.Pos()) {
		return false
	}

	for _, imp := range file.Imports {
		if imp.Path == nil {
			continue
		}
		path := strings.Trim(imp.Path.Value, `"`)
		if dbident.IsKnownDBPackagePath(path) {
			localName := ""
			if imp.Name != nil {
				localName = imp.Name.Name
			} else {
				localName = defaultPackageName(path)
			}
			if localName == id.Name {
				return true
			}
		}
	}
	return false
}

// isImportedPackage checks if pkgPath is imported in file with local name matching idName.
func isImportedPackage(file *ast.File, pkgPath, idName string) bool {
	if file == nil || idName == "" {
		return false
	}
	for _, imp := range file.Imports {
		if imp.Path == nil {
			continue
		}
		path := strings.Trim(imp.Path.Value, `"`)
		if path == pkgPath {
			if imp.Name != nil {
				return imp.Name.Name == idName
			}
			return defaultPackageName(path) == idName
		}
	}
	return false
}

// defaultPackageName derives the default Go package identifier from an import path.
func defaultPackageName(path string) string {
	switch path {
	case "database/sql":
		return "sql"
	case "encoding/json":
		return "json"
	case "net/http":
		return "http"
	case "github.com/jackc/pgx/v5", "github.com/jackc/pgx/v4":
		return "pgx"
	case "github.com/jackc/pgx/v5/pgxpool", "github.com/jackc/pgx/v4/pgxpool":
		return "pgxpool"
	case "github.com/jackc/pgx/v5/pgconn", "github.com/jackc/pgx/v4/pgconn":
		return "pgconn"
	case "github.com/jmoiron/sqlx":
		return "sqlx"
	case "github.com/lib/pq":
		return "pq"
	default:
		parts := strings.Split(path, "/")
		return parts[len(parts)-1]
	}
}

// isIdentShadowed checks if name is shadowed by a parameter, receiver, or local variable
// in fn before targetPos.
func isIdentShadowed(fn *ast.FuncDecl, name string, targetPos token.Pos) bool {
	if fn == nil || name == "" {
		return false
	}

	// 1. Receiver
	if fn.Recv != nil {
		for _, field := range fn.Recv.List {
			for _, id := range field.Names {
				if id.Name == name {
					return true
				}
			}
		}
	}

	// 2. Parameters
	if fn.Type != nil && fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			for _, id := range field.Names {
				if id.Name == name {
					return true
				}
			}
		}
	}

	// 3. Named return results
	if fn.Type != nil && fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			for _, id := range field.Names {
				if id.Name == name {
					return true
				}
			}
		}
	}

	// 4. Enclosing blocks and statements
	if fn.Body != nil {
		blocks := getEnclosingBlocks(fn.Body, targetPos)
		for _, b := range blocks {
			for _, stmt := range b.List {
				if stmt.Pos() >= targetPos {
					continue
				}
				switch s := stmt.(type) {
				case *ast.AssignStmt:
					if s.Tok == token.DEFINE {
						for _, lhs := range s.Lhs {
							if id, ok := lhs.(*ast.Ident); ok && id.Name == name {
								return true
							}
						}
					}
				case *ast.DeclStmt:
					if gen, ok := s.Decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
						for _, spec := range gen.Specs {
							if valSpec, ok := spec.(*ast.ValueSpec); ok {
								for _, id := range valSpec.Names {
									if id.Name == name {
										return true
									}
								}
							}
						}
					}
				case *ast.RangeStmt:
					if s.Tok == token.DEFINE {
						if k, ok := s.Key.(*ast.Ident); ok && k.Name == name {
							return true
						}
						if v, ok := s.Value.(*ast.Ident); ok && v.Name == name {
							return true
						}
					}
				}
			}
		}
	}

	return false
}
