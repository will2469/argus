// Package a07_error_leak traces data flow of database error strings and pgconn.PgError fields into response sinks.
package a07_error_leak

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/directives"
)

// CheckPgErrorSensitiveFields checks for forbidden direct access to Detail, Hint, or Where on PgError.
func CheckPgErrorSensitiveFields(pass *analysis.Pass, fset *token.FileSet, file *ast.File, fn *ast.FuncDecl, sel *ast.SelectorExpr, dm *directives.DirectiveMap, issues *[]Issue) {
	switch sel.Sel.Name {
	case "Detail", "Hint", "Where":
		if IsPgErrorSelector(pass, file, fn, sel) {
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

// CheckLeakedErrorArg validates an expression passed into an API response sink.
func CheckLeakedErrorArg(pass *analysis.Pass, fset *token.FileSet, file *ast.File, arg ast.Expr, callPos token.Pos, fn *ast.FuncDecl, tracker *ErrorTracker, dm *directives.DirectiveMap, issues *[]Issue) {
	if arg == nil {
		return
	}
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
		val := tracker.GetErrorStateAt(fn, sel.X, call)
		if val.kind == errorKindDB || val.kind == errorKindGenericParam {
			*issues = append(*issues, Issue{
				Pos:     arg.Pos(),
				Message: "raw err.Error() passed directly to HTTP response; may leak internal database errors and PII",
			})
		}
		return
	}

	// 2. Direct sensitive field access: pgErr.Detail, pgErr.Hint, pgErr.Where
	// Already inspected unconditionally by CheckPgErrorSensitiveFields on SelectorExpr.
	if sel, ok := arg.(*ast.SelectorExpr); ok {
		switch sel.Sel.Name {
		case "Detail", "Hint", "Where":
			if IsPgErrorSelector(pass, file, fn, sel) {
				return
			}
		}
	}

	// 3. Binary concatenation: "error: " + err.Error()
	if bin, ok := arg.(*ast.BinaryExpr); ok {
		CheckLeakedErrorArg(pass, fset, file, bin.X, callPos, fn, tracker, dm, issues)
		CheckLeakedErrorArg(pass, fset, file, bin.Y, callPos, fn, tracker, dm, issues)
		return
	}

	// 4. Formatted call: fmt.Sprintf("failed: %s", err.Error())
	if call, ok := arg.(*ast.CallExpr); ok && isFormatCall(pass, file, fn, call) {
		for _, a := range call.Args[1:] {
			CheckLeakedErrorArg(pass, fset, file, a, callPos, fn, tracker, dm, issues)
		}
		return
	}

	// 5. Local variable assigned from err.Error(), pgErr.Detail, or format string
	if id, ok := arg.(*ast.Ident); ok {
		val := tracker.GetErrorStateAt(fn, id, id)
		if val.kind == errorKindDB || val.kind == errorKindGenericParam {
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
