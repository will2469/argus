package a07_error_leak

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var errInterface = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

func isFuncErrorParam(pass *analysis.Pass, fn *ast.FuncDecl, id *ast.Ident) bool {
	if fn == nil || fn.Type == nil || fn.Type.Params == nil || id == nil {
		return false
	}

	// First verify that id is actually a function parameter
	var paramField *ast.Field
	for _, field := range fn.Type.Params.List {
		for _, paramID := range field.Names {
			if paramID.Name == id.Name {
				paramField = field
				break
			}
		}
		if paramField != nil {
			break
		}
	}
	if paramField == nil {
		return false
	}

	// 1. Semantic type check via pass.TypesInfo if available
	if pass != nil && pass.TypesInfo != nil {
		if t := pass.TypesInfo.TypeOf(id); t != nil && t != types.Typ[types.Invalid] {
			if types.Implements(t, errInterface) || types.Identical(t, types.Universe.Lookup("error").Type()) {
				return true
			}
			if ptr, ok := t.(*types.Pointer); ok && types.Implements(ptr, errInterface) {
				return true
			}
			return false
		}
	}

	// 2. AST fallback: inspect parameter declaration type
	return isASTErrorType(paramField.Type)
}

func isASTErrorType(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	switch e := expr.(type) {
	case *ast.Ident:
		if e.Name == "error" {
			return true
		}
		switch e.Name {
		case "string", "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
			"byte", "rune", "float32", "float64", "complex64", "complex128",
			"bool", "any":
			return false
		}
		return strings.HasSuffix(e.Name, "Error") || strings.HasSuffix(e.Name, "Err")
	case *ast.StarExpr:
		return isASTErrorType(e.X)
	case *ast.SelectorExpr:
		return strings.HasSuffix(e.Sel.Name, "Error") || strings.HasSuffix(e.Sel.Name, "Err")
	}
	return false
}

func isPgErrorParam(file *ast.File, name string, fn *ast.FuncDecl) bool {
	if fn == nil || fn.Type == nil || fn.Type.Params == nil {
		return false
	}
	for _, field := range fn.Type.Params.List {
		for _, id := range field.Names {
			if id.Name == name {
				return isPgErrorASTType(file, fn, field.Type)
			}
		}
	}
	return false
}

func isPgErrorASTType(file *ast.File, fn *ast.FuncDecl, expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	// Fail-closed: only accept exact AST selectors with proven package context
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if pkgID, ok := sel.X.(*ast.Ident); ok {
			if sel.Sel.Name == "PgError" && (isPackageIdent(nil, file, fn, pkgID, "github.com/jackc/pgx/v5/pgconn") || isPackageIdent(nil, file, fn, pkgID, "github.com/jackc/pgx/v4/pgconn")) {
				return true
			}
			if sel.Sel.Name == "Error" && isPackageIdent(nil, file, fn, pkgID, "github.com/lib/pq") {
				return true
			}
		}
	}
	// Fail-closed: reject bare "PgError" or "Error" identifiers without package context
	return false
}
