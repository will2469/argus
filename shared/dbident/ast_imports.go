// Package dbident provides import path resolution and package identification
// utilities for AST-level database inspection.
package dbident

import (
	"go/ast"
	"strings"
)

// HasDatabaseImports reports whether file imports any known database driver
// package, anchored to the canonical registry.
func HasDatabaseImports(file *ast.File) bool {
	if file == nil {
		return true // fail-open when no file context
	}
	for _, imp := range file.Imports {
		if imp.Path == nil {
			continue
		}
		path := strings.Trim(imp.Path.Value, `"`)
		if IsKnownDBPackagePath(path) {
			return true
		}
	}
	return false
}

// DefaultPackageName derives the default Go package identifier from an
// import path, handling well-known multi-segment paths.
func DefaultPackageName(path string) string {
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

// resolveImportPath looks up the full import path for a local package name.
func resolveImportPath(file *ast.File, pkgName string) string {
	if file == nil || pkgName == "" {
		return ""
	}
	for _, imp := range file.Imports {
		if imp.Path == nil {
			continue
		}
		path := strings.Trim(imp.Path.Value, `"`)
		localName := DefaultPackageName(path)
		if imp.Name != nil {
			localName = imp.Name.Name
		}
		if localName == pkgName {
			return path
		}
	}
	return ""
}
