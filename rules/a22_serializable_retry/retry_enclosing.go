// Package a22_serializable_retry inspects enclosing AST nodes for retry loops and retry helper closures.
package a22_serializable_retry

import (
	"go/ast"
	"strings"
)

// HasRetryEnclosure checks if the node or function is enclosed within a retry loop or retry helper closure.
func HasRetryEnclosure(fn ast.Node, target ast.Node) bool {
	if fn == nil || target == nil {
		return false
	}

	// 1. Check if enclosed within a loop in the AST
	hasEnclosingLoop := false
	ast.Inspect(fn, func(n ast.Node) bool {
		switch loop := n.(type) {
		case *ast.ForStmt:
			if loop.Pos() <= target.Pos() && target.End() <= loop.End() {
				hasEnclosingLoop = true
				return false
			}
		case *ast.RangeStmt:
			if loop.Pos() <= target.Pos() && target.End() <= loop.End() {
				hasEnclosingLoop = true
				return false
			}
		}
		return true
	})

	if hasEnclosingLoop {
		return true
	}

	// 2. Check if the function itself is a closure or named function indicating retry handling
	if funcDecl, ok := fn.(*ast.FuncDecl); ok {
		fnName := strings.ToLower(funcDecl.Name.Name)
		if strings.Contains(fnName, "retry") || strings.Contains(fnName, "txwrapper") {
			return true
		}
	}

	return false
}
