package a01_sql_concat

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)


// IsBuilderString checks if an expression is a call to strings.Builder.String().
func IsBuilderString(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "String"
}

// IsFormattingCall checks if a call expression is an unsafe string formatting function
// (fmt.Sprintf, fmt.Sprint, strings.Join), unless provably safe compile-time assembly or sanitized.
func IsFormattingCall(call *ast.CallExpr, tracker *TaintTracker, pass *analysis.Pass) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	if pkg.Name == "fmt" && sel.Sel.Name == "Sprintf" {
		if isSafeSprintf(call, tracker, pass) {
			return false
		}
		return true
	}

	if pkg.Name == "fmt" && sel.Sel.Name == "Sprint" {
		if isSafeSprint(call, tracker, pass) {
			return false
		}
		return true
	}

	if pkg.Name == "strings" && sel.Sel.Name == "Join" {
		if isSafeStringsJoin(call, tracker, pass) {
			return false
		}
		return true
	}

	return false
}

// isSafeSprintf returns true if fmt.Sprintf only interpolates compile-time constants or sanitized identifiers.
func isSafeSprintf(call *ast.CallExpr, tracker *TaintTracker, pass *analysis.Pass) bool {
	if len(call.Args) == 0 {
		return false
	}
	// Case 1: Indexed placeholder formatting (e.g. $%d)
	if IsSafePlaceholderFormatting(call, tracker) {
		return true
	}

	// Format string must be a string literal
	formatLit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || formatLit.Kind != token.STRING {
		return false
	}

	// If no arguments, it's a pure constant string
	if len(call.Args) == 1 {
		return true
	}

	// Case 2: All arguments are compile-time constants or verified sanitized identifiers
	for _, arg := range call.Args[1:] {
		if !isSafeSQLExpr(arg, tracker, pass) {
			return false
		}
	}
	return true
}

func isSafeSprint(call *ast.CallExpr, tracker *TaintTracker, pass *analysis.Pass) bool {
	if len(call.Args) == 0 {
		return false
	}
	for _, arg := range call.Args {
		if !isSafeSQLExpr(arg, tracker, pass) {
			return false
		}
	}
	return true
}

// isSafeStringsJoin returns true if strings.Join is used solely with compile-time constants.
func isSafeStringsJoin(call *ast.CallExpr, tracker *TaintTracker, pass *analysis.Pass) bool {
	if len(call.Args) != 2 {
		return false
	}
	// Separator must be compile-time string literal (e.g. " ", "\n", ", ")
	if !IsCompileTimeStringLiteral(call.Args[1]) {
		return false
	}

	// Slice argument must be a composite literal where all elements are compile-time string constants
	sliceArg := call.Args[0]
	if compLit, ok := sliceArg.(*ast.CompositeLit); ok {
		for _, elt := range compLit.Elts {
			if !isSafeSQLExpr(elt, tracker, pass) {
				return false
			}
		}
		return true
	}
	return false
}

// isSafeSQLExpr evaluates if an expression contains zero runtime injection risk.
func isSafeSQLExpr(e ast.Expr, tracker *TaintTracker, pass *analysis.Pass) bool {
	if e == nil {
		return false
	}
	if IsCompileTimeStringLiteral(e) {
		return true
	}
	if IsSanitized(e, pass) {
		return true
	}

	switch x := e.(type) {
	case *ast.ParenExpr:
		return isSafeSQLExpr(x.X, tracker, pass)
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			return isSafeSQLExpr(x.X, tracker, pass) && isSafeSQLExpr(x.Y, tracker, pass)
		}
	case *ast.Ident:
		if pass != nil && pass.TypesInfo != nil {
			if obj := pass.TypesInfo.Uses[x]; obj != nil {
				if _, ok := obj.(*types.Const); ok {
					return true
				}
			}
		}
	}
	return false
}

// IsSafePlaceholderFormatting returns true for compliant parameterized builders
// that only interpolate placeholder indices (e.g. fmt.Sprintf(" AND col = $%d", idx)).
func IsSafePlaceholderFormatting(call *ast.CallExpr, tracker *TaintTracker) bool {
	if len(call.Args) < 2 {
		return false
	}
	formatLit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || formatLit.Kind != token.STRING {
		return false
	}

	formatVal := strings.Trim(formatLit.Value, "`\"")

	// Disallow raw string interpolations (%s, %v, %q, %x)
	if strings.Contains(formatVal, "%s") || strings.Contains(formatVal, "%v") ||
		strings.Contains(formatVal, "%q") || strings.Contains(formatVal, "%x") {
		return false
	}

	// Must contain indexed placeholder pattern, e.g. $%d
	if !strings.Contains(formatVal, "$%d") && !strings.Contains(formatVal, "$%0") {
		return false
	}

	// Verify all arguments are untainted
	for _, arg := range call.Args[1:] {
		if tracker != nil && tracker.IsTaintedExpr(arg) {
			return false
		}
	}

	return true
}

// IsCompileTimeStringLiteral recursively verifies if all operands in an expression tree are compile-time string literals.
func IsCompileTimeStringLiteral(e ast.Expr) bool {
	if e == nil {
		return false
	}
	switch x := e.(type) {
	case *ast.BasicLit:
		return x.Kind == token.STRING
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			return IsCompileTimeStringLiteral(x.X) && IsCompileTimeStringLiteral(x.Y)
		}
	case *ast.ParenExpr:
		return IsCompileTimeStringLiteral(x.X)
	}
	return false
}

// IsSafeConcat verifies if a binary addition expression consists solely of string literals or sanitized inputs.
func IsSafeConcat(x *ast.BinaryExpr, pass *analysis.Pass) bool {
	if x == nil || x.Op != token.ADD {
		return false
	}
	if IsCompileTimeStringLiteral(x) {
		return true
	}
	if isSafeSQLExpr(x.X, nil, pass) && isSafeSQLExpr(x.Y, nil, pass) {
		return true
	}
	if (IsCompileTimeStringLiteral(x.X) && IsSanitized(x.Y, pass)) ||
		(IsCompileTimeStringLiteral(x.Y) && IsSanitized(x.X, pass)) {
		return true
	}
	return false
}
