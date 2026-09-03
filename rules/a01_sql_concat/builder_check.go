package a01_sql_concat

import (
	"go/ast"
	"go/token"
	"strings"
)

// PropagateBuilderCall marks a strings.Builder instance as tainted if WriteString/Write
// is called with a tainted expression or unsafe formatting.
func PropagateBuilderCall(t *TaintTracker, call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "WriteString" && sel.Sel.Name != "Write") {
		return
	}
	if len(call.Args) > 0 && t.IsTaintedExpr(call.Args[0]) {
		if ident, ok := sel.X.(*ast.Ident); ok {
			if t.pass != nil && t.pass.TypesInfo != nil {
				if obj := t.pass.TypesInfo.Uses[ident]; obj != nil {
					t.tainted[obj] = struct{}{}
				}
			} else {
				t.markTaintedName(ident.Name)
			}
		}
	}
}

// IsBuilderString checks if an expression is a call to strings.Builder.String().
func IsBuilderString(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "String"
}

// IsFormattingCall checks if a call expression is an unsafe string formatting function
// (fmt.Sprintf, fmt.Sprint, strings.Join), unless explicitly safe for parameterized indexing.
func IsFormattingCall(call *ast.CallExpr, tracker *TaintTracker) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	if pkg.Name == "fmt" && sel.Sel.Name == "Sprintf" {
		// Kasus B: fmt.Sprintf khusus indeks placeholder $%d diizinkan jika format aman
		if IsSafePlaceholderFormatting(call, tracker) {
			return false
		}
		return true
	}

	if pkg.Name == "fmt" && sel.Sel.Name == "Sprint" {
		return true
	}

	if pkg.Name == "strings" && sel.Sel.Name == "Join" {
		return true
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
func IsSafeConcat(x *ast.BinaryExpr) bool {
	if x == nil || x.Op != token.ADD {
		return false
	}
	if IsCompileTimeStringLiteral(x) {
		return true
	}
	if (IsCompileTimeStringLiteral(x.X) && IsSanitized(x.Y)) ||
		(IsCompileTimeStringLiteral(x.Y) && IsSanitized(x.X)) {
		return true
	}
	return false
}

// IsSanitized returns true if the expression is protected by an approved identifier sanitizer.
func IsSanitized(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fn.Sel.Name == "Sanitize" || fn.Sel.Name == "SanitizeIdentifier"
	case *ast.Ident:
		return fn.Name == "SanitizeIdentifier" || fn.Name == "Sanitize"
	}
	return false
}

func hasSQLArgument(call *ast.CallExpr) bool {
	sqlArg := extractSQLArgument(call)
	if sqlArg == nil {
		return false
	}
	if s, ok := extractSimpleString(sqlArg); ok {
		upper := strings.ToUpper(strings.TrimSpace(s))
		for _, prefix := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "WITH", "CREATE", "DROP", "ALTER"} {
			if strings.HasPrefix(upper, prefix) {
				return true
			}
		}
	}
	return false
}

func extractSimpleString(e ast.Expr) (string, bool) {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind == token.STRING && len(x.Value) >= 2 {
			return x.Value[1 : len(x.Value)-1], true
		}
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			return extractSimpleString(x.X)
		}
	}
	return "", false
}

func extractSQLArgument(call *ast.CallExpr) ast.Expr {
	if len(call.Args) == 0 {
		return nil
	}
	if len(call.Args) >= 2 {
		if isContextArg(call.Args[0]) {
			return call.Args[1]
		}
		return call.Args[0]
	}
	return call.Args[0]
}

func isContextArg(arg ast.Expr) bool {
	if id, ok := arg.(*ast.Ident); ok {
		lower := strings.ToLower(id.Name)
		return lower == "ctx" || lower == "context" || strings.HasPrefix(lower, "ctx")
	}
	if call, ok := arg.(*ast.CallExpr); ok {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			lower := strings.ToLower(sel.Sel.Name)
			if lower == "context" || lower == "background" || lower == "todo" {
				return true
			}
		}
	}
	return false
}
