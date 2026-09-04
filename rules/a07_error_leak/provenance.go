// Package a07_error_leak provides error provenance and origin classification,
// distinguishing database errors from non-database (validation, format, client) errors.
package a07_error_leak

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// ErrorOrigin represents the originating domain of an error variable or expression.
type ErrorOrigin int

const (
	// OriginUnknown represents an unresolved error origin.
	OriginUnknown ErrorOrigin = iota
	// OriginDatabase represents an error originating from a database driver, query, or transaction.
	OriginDatabase
	// OriginNonDatabase represents an error originating from validation, parsing, or non-DB code.
	OriginNonDatabase
	// OriginGeneric represents a generic unclassified handler error parameter.
	OriginGeneric
)

// GetErrorOrigin determines whether an error expression originates from a database source.
func GetErrorOrigin(pass *analysis.Pass, expr ast.Expr, fn *ast.FuncDecl) ErrorOrigin {
	if expr == nil {
		return OriginUnknown
	}

	switch e := expr.(type) {
	case *ast.CallExpr:
		return getCallErrorOrigin(pass, e)
	case *ast.Ident:
		return getIdentErrorOrigin(pass, e, fn)
	case *ast.SelectorExpr:
		if IsPgErrorSelector(pass, e) {
			return OriginDatabase
		}
		return getSelectorErrorOrigin(pass, e, fn)
	}
	return OriginUnknown
}

func getCallErrorOrigin(pass *analysis.Pass, call *ast.CallExpr) ErrorOrigin {
	if call == nil {
		return OriginUnknown
	}

	// 1. Is it a database call?
	if isDatabaseCall(pass, call) {
		return OriginDatabase
	}

	// 2. Is it a known non-database call (validation, standard parser)?
	if isNonDatabaseCall(pass, call) {
		return OriginNonDatabase
	}

	return OriginUnknown
}

func getIdentErrorOrigin(pass *analysis.Pass, id *ast.Ident, fn *ast.FuncDecl) ErrorOrigin {
	if id == nil {
		return OriginUnknown
	}

	// 1. Explicit variable naming conventions for non-DB domain errors
	lower := strings.ToLower(id.Name)
	if isNonDBErrorName(lower) {
		return OriginNonDatabase
	}

	// 2. Type-based resolution via go/types
	if pass != nil && pass.TypesInfo != nil {
		if t := pass.TypesInfo.TypeOf(id); t != nil {
			if IsPgErrorType(t) {
				return OriginDatabase
			}
		}
	}

	// 3. Trace local assignments in the enclosing function body
	if fn != nil && fn.Body != nil {
		if origin := traceAssignOrigin(pass, id.Name, fn.Body, fn); origin != OriginUnknown {
			return origin
		}
		// If it's a function parameter, check if it has a proven PgError type or is generic
		if isFuncParam(id.Name, fn) {
			if isPgErrorParam(id.Name, fn) {
				return OriginDatabase
			}
			return OriginGeneric
		}
	}

	return OriginUnknown
}

func getSelectorErrorOrigin(pass *analysis.Pass, sel *ast.SelectorExpr, fn *ast.FuncDecl) ErrorOrigin {
	if sel == nil {
		return OriginUnknown
	}
	lower := strings.ToLower(sel.Sel.Name)
	if isNonDBErrorName(lower) {
		return OriginNonDatabase
	}
	if origin := GetErrorOrigin(pass, sel.X, fn); origin != OriginUnknown {
		return origin
	}
	return OriginUnknown
}

func traceAssignOrigin(pass *analysis.Pass, varName string, body *ast.BlockStmt, fn *ast.FuncDecl) ErrorOrigin {
	var origin ErrorOrigin
	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range assign.Lhs {
			id, ok := lhs.(*ast.Ident)
			if !ok || id.Name != varName {
				continue
			}

			var rhs ast.Expr
			if i < len(assign.Rhs) {
				rhs = assign.Rhs[i]
			} else if len(assign.Rhs) == 1 {
				rhs = assign.Rhs[0]
			} else {
				continue
			}

			// Check if RHS is a call
			if call, ok := rhs.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Error" && len(call.Args) == 0 {
					origin = GetErrorOrigin(pass, sel.X, fn)
					return false
				}
				origin = getCallErrorOrigin(pass, call)
				return false
			}

			// Check if RHS is type assertion, e.g. pgErr, ok := err.(*PgError)
			if ta, ok := rhs.(*ast.TypeAssertExpr); ok && ta.Type != nil {
				if pass != nil && pass.TypesInfo != nil {
					if t := pass.TypesInfo.TypeOf(ta.Type); t != nil && IsPgErrorType(t) {
						origin = OriginDatabase
						return false
					}
				}
				if isPgErrorASTType(ta.Type) {
					origin = OriginDatabase
					return false
				}
			}

			// Check if RHS is another identifier (alias)
			if rhsID, ok := rhs.(*ast.Ident); ok {
				origin = getIdentErrorOrigin(pass, rhsID, fn)
				return false
			}

			// Check if RHS is selector (e.g. pgErr.Detail or obj.Err)
			if sel, ok := rhs.(*ast.SelectorExpr); ok {
				if IsPgErrorSelector(pass, sel) {
					origin = OriginDatabase
					return false
				}
				origin = GetErrorOrigin(pass, sel.X, fn)
				return false
			}
		}
		return true
	})
	return origin
}

func isFuncParam(name string, fn *ast.FuncDecl) bool {
	if fn == nil || fn.Type == nil || fn.Type.Params == nil {
		return false
	}
	for _, field := range fn.Type.Params.List {
		for _, id := range field.Names {
			if id.Name == name {
				return true
			}
		}
	}
	return false
}

func isPgErrorParam(name string, fn *ast.FuncDecl) bool {
	if fn == nil || fn.Type == nil || fn.Type.Params == nil {
		return false
	}
	for _, field := range fn.Type.Params.List {
		for _, id := range field.Names {
			if id.Name == name {
				return isPgErrorASTType(field.Type)
			}
		}
	}
	return false
}

func isPgErrorASTType(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name == "PgError" || id.Name == "Error"
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if sel.Sel.Name == "PgError" || sel.Sel.Name == "Error" {
			if pkgID, ok := sel.X.(*ast.Ident); ok {
				return pkgID.Name == "pgconn" || pkgID.Name == "pq" || pkgID.Name == "pgdriver"
			}
		}
	}
	return false
}
