// Package a07_error_leak evaluates error masking contracts, compile-time static constants,
// and factory envelope constructors to prevent leaking unmasked database errors.
package a07_error_leak

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/directives"
)

// IsCompileTimeString checks whether an expression is guaranteed to be a compile-time constant string,
// eliminating any risk of runtime database error reflection.
func IsCompileTimeString(pass *analysis.Pass, expr ast.Expr) bool {
	if expr == nil {
		return false
	}

	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Kind == token.STRING
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return IsCompileTimeString(pass, e.X) && IsCompileTimeString(pass, e.Y)
		}
	case *ast.Ident:
		if pass != nil && pass.TypesInfo != nil {
			if obj := pass.TypesInfo.Uses[e]; obj != nil {
				if _, isConst := obj.(*types.Const); isConst {
					return true
				}
			}
		}
		if e.Obj != nil && e.Obj.Kind == ast.Con {
			return true
		}
	}

	return false
}

// CheckErrorFactoryCall inspects constructor and envelope factory calls (e.g. NewBadRequest, NewNotFound, Wrap)
// to verify that raw database errors are not directly passed as the user-facing message argument.
func CheckErrorFactoryCall(pass *analysis.Pass, fset *token.FileSet, file *ast.File, fn *ast.FuncDecl, call *ast.CallExpr, tracker *ErrorTracker, dm *directives.DirectiveMap, issues *[]Issue) {
	if call == nil {
		return
	}

	fnName := getCalledFunctionName(call)
	if !isErrorFactoryName(fnName) {
		return
	}

	for _, arg := range call.Args {
		if isCauseOptionCall(arg) {
			continue
		}

		if ContainsTaintedError(pass, file, fn, arg, tracker) {
			if fset != nil && dm != nil && (dm.IsIgnored(fset, arg.Pos(), RuleCode) || dm.IsIgnored(fset, call.Pos(), RuleCode)) {
				return
			}
			*issues = append(*issues, Issue{
				Pos:     arg.Pos(),
				Message: "unmasked database error passed as message argument to error factory " + fnName + "; leaks internal database errors into error envelope",
			})
			return
		}
	}
}

// ContainsTaintedError checks whether an expression directly or indirectly embeds an unmasked database error.
func ContainsTaintedError(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl, expr ast.Expr, tracker *ErrorTracker) bool {
	if expr == nil || IsCompileTimeString(pass, expr) {
		return false
	}

	// 1. Direct err.Error()
	if IsErrorCall(expr) {
		call := expr.(*ast.CallExpr)
		sel := call.Fun.(*ast.SelectorExpr)
		val := tracker.GetErrorStateAt(fn, sel.X, call)
		return val.kind == errorKindDB || val.kind == errorKindGenericParam
	}

	// 2. Sensitive driver fields (pgErr.Detail, pgErr.Hint, pgErr.Where)
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		switch sel.Sel.Name {
		case "Detail", "Hint", "Where":
			if IsPgErrorSelector(pass, sel) {
				return true
			}
		}
	}

	// 3. String concatenation
	if bin, ok := expr.(*ast.BinaryExpr); ok && bin.Op == token.ADD {
		return ContainsTaintedError(pass, file, fn, bin.X, tracker) || ContainsTaintedError(pass, file, fn, bin.Y, tracker)
	}

	// 4. fmt.Sprintf formatting
	if call, ok := expr.(*ast.CallExpr); ok && isFormatCall(call) {
		for _, a := range call.Args[1:] {
			if ContainsTaintedError(pass, file, fn, a, tracker) {
				return true
			}
		}
	}

	// 5. Identifier assigned from error expression
	if id, ok := expr.(*ast.Ident); ok {
		val := tracker.GetErrorStateAt(fn, id, id)
		return val.kind == errorKindDB || val.kind == errorKindGenericParam
	}

	return false
}

func isFormatCall(call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if id, ok := sel.X.(*ast.Ident); ok && id.Name == "fmt" {
		return sel.Sel.Name == "Sprintf" && len(call.Args) >= 2
	}
	return false
}

func getCalledFunctionName(call *ast.CallExpr) string {
	switch f := call.Fun.(type) {
	case *ast.SelectorExpr:
		return f.Sel.Name
	case *ast.Ident:
		return f.Name
	}
	return ""
}

func isErrorFactoryName(name string) bool {
	if strings.HasPrefix(name, "New") || strings.HasPrefix(name, "Wrap") ||
		strings.HasPrefix(name, "From") || strings.HasPrefix(name, "Create") {
		lower := strings.ToLower(name)
		return strings.Contains(lower, "error") || strings.Contains(lower, "badrequest") ||
			strings.Contains(lower, "notfound") || strings.Contains(lower, "forbidden") ||
			strings.Contains(lower, "conflict") || strings.Contains(lower, "unauthorized")
	}
	return false
}

func isCauseOptionCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	name := getCalledFunctionName(call)
	lower := strings.ToLower(name)
	return strings.Contains(lower, "cause") || strings.Contains(lower, "internal") || strings.Contains(lower, "context")
}
