// Package a26_like_sanitize traces Go argument expressions to verify wildcard sanitization.
package a26_like_sanitize

import (
	"go/ast"
	"go/token"
	"strings"
)

// IsArgumentSanitized traces an AST expression to determine if it is properly sanitized against SQL wildcard injection.
func IsArgumentSanitized(expr ast.Expr, body *ast.BlockStmt) bool {
	if expr == nil {
		return true
	}

	switch e := expr.(type) {
	case *ast.BasicLit:
		// Static constant strings (e.g. "STATUS_%") are safe
		return true

	case *ast.CallExpr:
		return isSanitizerCall(e)

	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			// Concatenation: check non-literal components
			return isBinaryConcatSanitized(e, body)
		}

	case *ast.Ident:
		return isIdentSanitized(e, body)
	}

	return false
}

func isSanitizerCall(call *ast.CallExpr) bool {
	if call == nil {
		return false
	}

	methodName := strings.ToLower(getCallMethodName(call.Fun))
	switch methodName {
	case "sanitizelikepattern", "sanitizelike", "sanitizelikewildcards", "sanitizewildcards",
		"formatlikecontains", "formatlikeprefix", "formatlikesuffix",
		"escapelikepattern", "escapelike", "escapelikewildcards", "escapelikestring",
		"quotelikepattern", "quotelike",
		"cleanlikepattern", "cleanlike":
		return true
	}

	return false
}

func isBinaryConcatSanitized(bin *ast.BinaryExpr, body *ast.BlockStmt) bool {
	leftSafe := IsArgumentSanitized(bin.X, body)
	rightSafe := IsArgumentSanitized(bin.Y, body)
	return leftSafe && rightSafe
}

func isIdentSanitized(ident *ast.Ident, body *ast.BlockStmt) bool {
	if ident == nil || body == nil {
		return false
	}

	var isAssignedSafe bool
	var foundAssignment bool

	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		for i, lhs := range assign.Lhs {
			if id, ok := lhs.(*ast.Ident); ok && id.Name == ident.Name {
				foundAssignment = true
				if i < len(assign.Rhs) {
					if IsArgumentSanitized(assign.Rhs[i], body) {
						isAssignedSafe = true
					}
				}
			}
		}
		return true
	})

	if foundAssignment {
		return isAssignedSafe
	}

	// Function parameter or unresolved variable without prior sanitization
	return false
}

func getCallMethodName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	}
	return ""
}
