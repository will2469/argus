// Package a10_isolation_level inspects Go AST expressions to verify strong transaction isolation options.
package a10_isolation_level

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// HasStrongIsolation checks if the expression sets Serializable or RepeatableRead.
func HasStrongIsolation(pass *analysis.Pass, expr ast.Expr, body *ast.BlockStmt) bool {
	if expr == nil {
		return false
	}

	if comp, ok := expr.(*ast.CompositeLit); ok {
		return checkTxOptionsComposite(comp)
	}

	if id, ok := expr.(*ast.Ident); ok && body != nil {
		var found bool
		ast.Inspect(body, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, lhs := range assign.Lhs {
				if lid, ok := lhs.(*ast.Ident); ok && isSameObject(pass, lid, id) && i < len(assign.Rhs) {
					if c, ok := assign.Rhs[i].(*ast.CompositeLit); ok {
						if checkTxOptionsComposite(c) {
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

	return false
}

func checkTxOptionsComposite(comp *ast.CompositeLit) bool {
	for _, elt := range comp.Elts {
		if kve, ok := elt.(*ast.KeyValueExpr); ok {
			if key, ok := kve.Key.(*ast.Ident); ok && key.Name == "IsoLevel" {
				if sel, ok := kve.Value.(*ast.SelectorExpr); ok {
					return isStrongIsoLevel(sel.Sel.Name)
				}
				if id, ok := kve.Value.(*ast.Ident); ok {
					return isStrongIsoLevel(id.Name)
				}
			}
		}
	}
	return false
}

func isStrongIsoLevel(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "repeatableread") || strings.Contains(lower, "serializable")
}

func extractClosureArg(call *ast.CallExpr) *ast.FuncLit {
	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.FuncLit); ok {
			return lit
		}
	}
	return nil
}

func isTxEndStmt(stmt ast.Stmt, txVarName string) bool {
	var isEnd bool
	ast.Inspect(stmt, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if (sel.Sel.Name == "Commit" || sel.Sel.Name == "Rollback") && sel.X != nil {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == txVarName {
				isEnd = true
				return false
			}
		}
		return true
	})
	return isEnd
}
