// Package a07_error_leak traces data flow of database error strings and pgconn.PgError fields.
package a07_error_leak

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/directives"
)

// CheckPgErrorSensitiveFields checks for forbidden direct access to Detail, Hint, or Where on PgError.
func CheckPgErrorSensitiveFields(pass *analysis.Pass, fset *token.FileSet, sel *ast.SelectorExpr, dm *directives.DirectiveMap, issues *[]Issue) {
	switch sel.Sel.Name {
	case "Detail", "Hint", "Where":
		if IsPgErrorSelector(pass, sel) {
			if fset != nil && dm != nil && dm.IsIgnored(fset, sel.Pos(), RuleCode) {
				return
			}
			*issues = append(*issues, Issue{
				Pos:     sel.Pos(),
				Message: "forbidden direct access to pgconn.PgError." + sel.Sel.Name + "; contains raw database internals and PII",
			})
		}
	}
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
	// AST fallback: check if sel.X has AST type proof
	if id, ok := sel.X.(*ast.Ident); ok && id.Obj != nil {
		if field, ok := id.Obj.Decl.(*ast.Field); ok && isPgErrorASTType(field.Type) {
			return true
		}
		if vs, ok := id.Obj.Decl.(*ast.ValueSpec); ok && isPgErrorASTType(vs.Type) {
			return true
		}
		if as, ok := id.Obj.Decl.(*ast.AssignStmt); ok {
			for i, lhs := range as.Lhs {
				if lid, ok := lhs.(*ast.Ident); ok && lid.Name == id.Name {
					var rhs ast.Expr
					if i < len(as.Rhs) {
						rhs = as.Rhs[i]
					} else if len(as.Rhs) == 1 {
						rhs = as.Rhs[0]
					}
					if ta, ok := rhs.(*ast.TypeAssertExpr); ok && isPgErrorASTType(ta.Type) {
						return true
					}
				}
			}
		}
	}
	return false
}

// IsPgErrorType inspects whether a go/types Type is a pgconn.PgError or equivalent PostgreSQL error struct.
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

	tStr := t.String()
	if strings.Contains(tStr, "pgconn.PgError") || strings.Contains(tStr, "pq.Error") {
		return true
	}

	if named, ok := t.(*types.Named); ok {
		if named.Obj() != nil && named.Obj().Name() == "PgError" {
			return true
		}
		if st, ok := named.Underlying().(*types.Struct); ok {
			var hasDetail, hasHint, hasWhere bool
			for i := 0; i < st.NumFields(); i++ {
				switch st.Field(i).Name() {
				case "Detail":
					hasDetail = true
				case "Hint":
					hasHint = true
				case "Where":
					hasWhere = true
				}
			}
			if hasDetail && hasHint && hasWhere {
				return true
			}
		}
	}

	return false
}

// CheckLeakedErrorArg validates an expression passed into an API response sink.
func CheckLeakedErrorArg(pass *analysis.Pass, fset *token.FileSet, arg ast.Expr, callPos token.Pos, fn *ast.FuncDecl, dm *directives.DirectiveMap, issues *[]Issue) {
	if fset != nil && dm != nil {
		if dm.IsIgnored(fset, arg.Pos(), RuleCode) || dm.IsIgnored(fset, callPos, RuleCode) {
			return
		}
	}

	// 0. Compile-time constant strings are masked and safe
	if IsCompileTimeString(pass, arg) {
		return
	}

	// 1. Direct call: err.Error()
	if IsErrorCall(arg) {
		call := arg.(*ast.CallExpr)
		sel := call.Fun.(*ast.SelectorExpr)
		origin := GetErrorOrigin(pass, sel.X, fn)
		if origin != OriginDatabase && origin != OriginGeneric {
			return
		}
		*issues = append(*issues, Issue{
			Pos:     arg.Pos(),
			Message: "raw err.Error() passed directly to HTTP response; may leak internal database errors and PII",
		})
		return
	}

	// 2. Binary concatenation: "error: " + err.Error()
	if bin, ok := arg.(*ast.BinaryExpr); ok {
		CheckLeakedErrorArg(pass, fset, bin.X, callPos, fn, dm, issues)
		CheckLeakedErrorArg(pass, fset, bin.Y, callPos, fn, dm, issues)
		return
	}

	// 3. Formatted call: fmt.Sprintf("failed: %s", err.Error())
	if call, ok := arg.(*ast.CallExpr); ok && isFormatCall(call) {
		for _, a := range call.Args[1:] {
			CheckLeakedErrorArg(pass, fset, a, callPos, fn, dm, issues)
		}
		return
	}

	// 4. Local variable assigned from err.Error(), pgErr.Detail, or fmt.Sprintf
	if id, ok := arg.(*ast.Ident); ok {
		origin := GetErrorOrigin(pass, id, fn)
		if origin != OriginDatabase && origin != OriginGeneric {
			return
		}

		var body *ast.BlockStmt
		if fn != nil {
			body = fn.Body
		}
		if IsVarAssignedFromError(pass, id.Name, body, fn) {
			*issues = append(*issues, Issue{
				Pos:     arg.Pos(),
				Message: "variable \"" + id.Name + "\" derived from raw err.Error() passed to HTTP response; may leak internal database errors and PII",
			})
		}
	}
}

// IsErrorCall checks if an expression is a call to .Error().
func IsErrorCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "Error" && len(call.Args) == 0
}

// IsVarAssignedFromError checks if a variable in the block is derived from err.Error() or PgError fields,
// taking error origin provenance into account.
func IsVarAssignedFromError(pass *analysis.Pass, varName string, body *ast.BlockStmt, fn *ast.FuncDecl) bool {
	if body == nil {
		return false
	}

	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range assign.Lhs {
			id, ok := lhs.(*ast.Ident)
			if ok && id.Name == varName && i < len(assign.Rhs) {
				rhs := assign.Rhs[i]
				if IsErrorCall(rhs) {
					call := rhs.(*ast.CallExpr)
					sel := call.Fun.(*ast.SelectorExpr)
					origin := GetErrorOrigin(pass, sel.X, fn)
					if origin != OriginDatabase && origin != OriginGeneric {
						return false
					}
					found = true
					return false
				}
				if sel, ok := rhs.(*ast.SelectorExpr); ok {
					if sel.Sel.Name == "Detail" || sel.Sel.Name == "Hint" || sel.Sel.Name == "Where" {
						found = true
						return false
					}
				}
				if call, ok := rhs.(*ast.CallExpr); ok && isFormatCall(call) {
					for _, a := range call.Args[1:] {
						if ContainsTaintedError(pass, a, fn) {
							found = true
							return false
						}
					}
				}
				if bin, ok := rhs.(*ast.BinaryExpr); ok && bin.Op == token.ADD {
					if ContainsTaintedError(pass, bin, fn) {
						found = true
						return false
					}
				}
			}
		}
		return true
	})
	return found
}
