// Package a22_serializable_retry verifies SQLSTATE error handling for serialization failures.
package a22_serializable_retry

import (
	"go/ast"
	"strings"
)

// HasSerializationErrorCheck checks if the function contains error inspection for SQLSTATE 40001 or 40P01.
func HasSerializationErrorCheck(fn ast.Node) bool {
	if fn == nil {
		return false
	}

	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok {
			val := strings.Trim(lit.Value, `"'`)
			if val == "40001" || val == "40P01" {
				found = true
				return false
			}
		}
		if id, ok := n.(*ast.Ident); ok {
			lower := strings.ToLower(id.Name)
			if strings.Contains(lower, "serializationfailure") ||
				strings.Contains(lower, "deadlock") ||
				strings.Contains(lower, "errserialization") {
				found = true
				return false
			}
		}
		return true
	})

	return found
}
