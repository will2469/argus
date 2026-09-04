// Package a10_isolation_level provides approved database package identifiers, naming filters,
// and AST type resolution utilities.
package a10_isolation_level

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

func isKnownDBPackagePath(path string) bool {
	switch path {
	case "database/sql", "github.com/jackc/pgx/v5", "github.com/jackc/pgx/v5/pgxpool",
		"github.com/jackc/pgx/v5/pgconn", "github.com/jackc/pgx/v4", "github.com/jackc/pgx/v4/pgxpool",
		"github.com/jmoiron/sqlx", "github.com/lib/pq":
		return true
	}
	return false
}

func getPackageImportPath(pkgName string, file *ast.File) string {
	if file == nil || pkgName == "" {
		return ""
	}
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, "`\"")
		localName := defaultPackageName(path)
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
	return isKnownDBPackagePath(path)
}

func defaultPackageName(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func isKnownNonDBTypeName(name string) bool {
	switch strings.ToLower(name) {
	case "calculator", "searchengine", "search", "player", "videoplayer",
		"audioplayer", "parser", "compiler", "logger", "formatter",
		"math", "cache", "client", "workerpool", "taskrunner", "fakedbproxy", "fakesearchproxy",
		"git", "vcs", "repository", "session":
		return true
	}
	return false
}

func unwrapPointer(t types.Type) types.Type {
	for {
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		} else {
			return t
		}
	}
}

func getTypeName(t types.Type) string {
	if t == nil {
		return ""
	}
	t = unwrapPointer(t)
	if named, ok := t.(*types.Named); ok && named.Obj() != nil {
		return named.Obj().Name()
	}
	return ""
}

func getASTTypeName(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.IndexExpr:
		return getASTTypeName(e.X)
	case *ast.IndexListExpr:
		return getASTTypeName(e.X)
	case *ast.SelectorExpr:
		return e.Sel.Name
	}
	return ""
}

func findTypeSpec(name string, file *ast.File) *ast.TypeSpec {
	if file == nil || name == "" {
		return nil
	}
	for _, decl := range file.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.TYPE {
			for _, spec := range gen.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == name {
					return ts
				}
			}
		}
	}
	return nil
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
