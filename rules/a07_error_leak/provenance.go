// Package a07_error_leak evaluates error provenance and origin classification,
// distinguishing database errors from non-database (validation, client, custom) errors.
package a07_error_leak

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// evalExprOrigin determines the semantic error origin of an AST expression.
func evalExprOrigin(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl, expr ast.Expr, state *errorState) errorValue {
	if expr == nil {
		return errorValue{kind: errorKindClean}
	}

	switch e := expr.(type) {
	case *ast.CallExpr:
		return evalCallOrigin(pass, file, fn, e, state)
	case *ast.Ident:
		return evalIdentOrigin(pass, file, fn, e, state)
	case *ast.TypeAssertExpr:
		if e.Type != nil {
			if pass != nil && pass.TypesInfo != nil {
				if t := pass.TypesInfo.TypeOf(e.Type); t != nil && IsPgErrorType(t) {
					return errorValue{kind: errorKindDB, source: "typeassert PgError"}
				}
			}
			if isPgErrorASTType(e.Type) {
				return errorValue{kind: errorKindDB, source: "typeassert PgError"}
			}
		}
	case *ast.SelectorExpr:
		if IsPgErrorSelector(pass, e) {
			return errorValue{kind: errorKindDB, source: "pgError field"}
		}
		if state != nil {
			k := makeVarKey(pass, file, fn, getRootIdent(e.X))
			if v := state.get(k); v.kind != errorKindClean {
				return v
			}
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			v1 := evalExprOrigin(pass, file, fn, e.X, state)
			v2 := evalExprOrigin(pass, file, fn, e.Y, state)
			return joinValues(v1, v2)
		}
	case *ast.ParenExpr:
		return evalExprOrigin(pass, file, fn, e.X, state)
	}

	return errorValue{kind: errorKindClean}
}

func evalCallOrigin(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl, call *ast.CallExpr, state *errorState) errorValue {
	// 1. Is it a database call?
	if isDatabaseCall(pass, file, fn, call) {
		return errorValue{kind: errorKindDB, source: "database operation"}
	}

	// 2. Is it .Error() method invocation on an error object?
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Error" && len(call.Args) == 0 {
		return evalExprOrigin(pass, file, fn, sel.X, state)
	}

	// 3. Is it errors.New?
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "errors" && sel.Sel.Name == "New" {
			return errorValue{kind: errorKindClean, source: "errors.New"}
		}
		// fmt.Errorf: check if any wrapped arg is DB tainted
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "fmt" && sel.Sel.Name == "Errorf" {
			for _, arg := range call.Args[1:] {
				if v := evalExprOrigin(pass, file, fn, arg, state); v.kind == errorKindDB {
					return errorValue{kind: errorKindDB, source: "fmt.Errorf wrapping DB error"}
				}
			}
			return errorValue{kind: errorKindClean, source: "fmt.Errorf clean"}
		}
	}

	return errorValue{kind: errorKindClean}
}

func evalIdentOrigin(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl, id *ast.Ident, state *errorState) errorValue {
	// 1. Check types.Info if it is a proven PgError type
	if pass != nil && pass.TypesInfo != nil {
		if t := pass.TypesInfo.TypeOf(id); t != nil && IsPgErrorType(t) {
			return errorValue{kind: errorKindDB, source: "PgError object"}
		}
	}

	// 2. Check flow state for tracked variable
	if state != nil {
		k := makeVarKey(pass, file, fn, id)
		if val := state.get(k); val.kind != errorKindClean {
			return val
		}
	}

	// 3. Function parameter check
	if isFuncParam(id.Name, fn) {
		if isPgErrorParam(id.Name, fn) {
			return errorValue{kind: errorKindDB, source: "PgError param"}
		}
		return errorValue{kind: errorKindGenericParam, source: "error param"}
	}

	return errorValue{kind: errorKindClean}
}

// IsPgErrorSelector determines whether a selector refers to a pgconn.PgError struct.
func IsPgErrorSelector(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	if sel == nil {
		return false
	}
	if pass != nil && pass.TypesInfo != nil {
		if typ := pass.TypesInfo.TypeOf(sel.X); typ != nil && typ != types.Typ[types.Invalid] {
			return IsPgErrorType(typ)
		}
	}
	// AST fallback
	if id, ok := sel.X.(*ast.Ident); ok && id.Obj != nil {
		if field, ok := id.Obj.Decl.(*ast.Field); ok && isPgErrorASTType(field.Type) {
			return true
		}
		if vs, ok := id.Obj.Decl.(*ast.ValueSpec); ok && isPgErrorASTType(vs.Type) {
			return true
		}
	}
	return false
}

// IsPgErrorType inspects whether a go/types Type is a pgconn.PgError or pq.Error.
// Fail-closed: only accepts exact package path verification, not structural heuristics.
func IsPgErrorType(t types.Type) bool {
	if t == nil {
		return false
	}
	for {
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		} else {
			break
		}
	}

	// Fail-closed: only accept exact package path matches
	if named, ok := t.(*types.Named); ok && named.Obj() != nil {
		pkg := named.Obj().Pkg()
		if pkg == nil {
			return false
		}
		path := pkg.Path()
		name := named.Obj().Name()

		// Exact package/type matches only
		if (path == "github.com/jackc/pgx/v5/pgconn" && name == "PgError") ||
			(path == "github.com/lib/pq" && name == "Error") {
			return true
		}
	}

	return false
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
	// Fail-closed: only accept exact AST selectors with proven package context
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if sel.Sel.Name == "PgError" || sel.Sel.Name == "Error" {
			if pkgID, ok := sel.X.(*ast.Ident); ok {
				// Only accept known PostgreSQL driver packages
				return pkgID.Name == "pgconn" || pkgID.Name == "pq"
			}
		}
	}
	// Fail-closed: reject bare "PgError" or "Error" identifiers without package context
	return false
}

func getRootIdent(expr ast.Expr) *ast.Ident {
	switch e := expr.(type) {
	case *ast.Ident:
		return e
	case *ast.UnaryExpr:
		return getRootIdent(e.X)
	case *ast.ParenExpr:
		return getRootIdent(e.X)
	}
	return nil
}
