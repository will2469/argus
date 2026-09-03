// Package a03_context detects database operations executed with unbounded raw contexts
// such as context.Background() or context.TODO(), preventing zombie query resource saturation.
package a03_context

import (
	"go/ast"
)

// IsRawContext checks whether an expression directly or indirectly resolves to
// an unbounded raw context.Background() or context.TODO() within the enclosing block.
func IsRawContext(e ast.Expr, body *ast.BlockStmt) bool {
	if e == nil {
		return false
	}

	// 1. Direct call to context.Background() or context.TODO()
	if call, ok := e.(*ast.CallExpr); ok {
		return isRawContextCall(call)
	}

	// 2. Local variable resolution with alias tracing
	ident, ok := e.(*ast.Ident)
	if !ok || body == nil {
		return false
	}

	return resolveIdentIsRaw(ident.Name, body, make(map[string]bool))
}

func resolveIdentIsRaw(varName string, body *ast.BlockStmt, visited map[string]bool) bool {
	if visited[varName] {
		return false
	}
	visited[varName] = true

	var rawSource bool
	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range assign.Lhs {
			id, ok := lhs.(*ast.Ident)
			if !ok || id.Name != varName {
				continue
			}

			var rhs ast.Expr
			if i < len(assign.Rhs) {
				rhs = assign.Rhs[i]
			} else if len(assign.Rhs) == 1 {
				rhs = assign.Rhs[0]
			}
			if rhs == nil {
				continue
			}

			if call, ok := rhs.(*ast.CallExpr); ok {
				if isRawContextCall(call) {
					rawSource = true
				} else if isBoundedContextCall(call) {
					rawSource = false
				}
			} else if aliasIdent, ok := rhs.(*ast.Ident); ok {
				if resolveIdentIsRaw(aliasIdent.Name, body, visited) {
					rawSource = true
				}
			}
		}
		return true
	})

	return rawSource
}

func isRawContextCall(call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != "context" {
		return false
	}
	return sel.Sel.Name == "Background" || sel.Sel.Name == "TODO"
}

func isBoundedContextCall(call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	// Case 1: r.Context()
	if sel.Sel.Name == "Context" {
		return true
	}
	// Case 2: context.WithTimeout, context.WithDeadline
	ident, ok := sel.X.(*ast.Ident)
	if ok && ident.Name == "context" {
		return sel.Sel.Name == "WithTimeout" || sel.Sel.Name == "WithDeadline"
	}
	return false
}
