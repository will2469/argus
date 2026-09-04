// Package a07_error_leak evaluates error provenance and origin classification,
// distinguishing database errors from non-database (validation, client, custom) errors.
package a07_error_leak

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var errInterface = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)


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
			if isPgErrorASTType(file, fn, e.Type) {
				return errorValue{kind: errorKindDB, source: "typeassert PgError"}
			}
		}
	case *ast.SelectorExpr:
		if IsPgErrorSelector(pass, file, fn, e) {
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

	// 3. Is it stdlib errors.New?
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if isPackageCall(pass, file, fn, sel, "errors", "New") {
			return errorValue{kind: errorKindClean, source: "errors.New"}
		}
		// 4. fmt.Errorf: check if any wrapped arg is DB tainted
		if isPackageCall(pass, file, fn, sel, "fmt", "Errorf") {
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
	if isFuncErrorParam(pass, file, fn, id) {
		if isPgErrorParam(file, id.Name, fn) {
			return errorValue{kind: errorKindDB, source: "PgError param"}
		}
		return errorValue{kind: errorKindGenericParam, source: "error param"}
	}

	return errorValue{kind: errorKindClean}
}

// IsPgErrorSelector determines whether a selector refers to a pgconn.PgError struct.
func IsPgErrorSelector(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl, sel *ast.SelectorExpr) bool {
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
		if field, ok := id.Obj.Decl.(*ast.Field); ok && isPgErrorASTType(file, fn, field.Type) {
			return true
		}
		if vs, ok := id.Obj.Decl.(*ast.ValueSpec); ok && isPgErrorASTType(file, fn, vs.Type) {
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
			(path == "github.com/jackc/pgx/v4/pgconn" && name == "PgError") ||
			(path == "github.com/lib/pq" && name == "Error") {
			return true
		}
	}

	return false
}

func isFuncErrorParam(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl, id *ast.Ident) bool {
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
