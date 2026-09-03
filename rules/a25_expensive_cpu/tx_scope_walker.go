// Package a25_expensive_cpu traverses database transaction scopes and verifies call sites.
package a25_expensive_cpu

import (
	"go/ast"
	"go/token"

	"github.com/will2469/argus/shared/directives"
)

// ExtractTxClosure detects closure-based transactions (BeginFunc, ExecuteTx, ExecuteLockedTx, WithTx).
func ExtractTxClosure(call *ast.CallExpr) *ast.FuncLit {
	if call == nil {
		return nil
	}
	methodName := getCallMethodName(call.Fun)
	switch methodName {
	case "BeginFunc", "ExecuteTx", "ExecuteLockedTx", "WithTx":
		for _, arg := range call.Args {
			if lit, ok := arg.(*ast.FuncLit); ok {
				return lit
			}
			if id, ok := arg.(*ast.Ident); ok && id.Obj != nil {
				if assign, ok := id.Obj.Decl.(*ast.AssignStmt); ok {
					for _, rhs := range assign.Rhs {
						if lit, ok := rhs.(*ast.FuncLit); ok {
							return lit
						}
					}
				}
			}
		}
	}
	return nil
}

// InspectExplicitTxRanges detects statements executed while holding an open transaction (pool.Begin ... tx.Commit).
func InspectExplicitTxRanges(body *ast.BlockStmt, onStmtInTx func(stmt ast.Stmt)) {
	if body == nil {
		return
	}

	var inTx bool
	var txVarName string

	for _, stmt := range body.List {
		if !inTx {
			// Detect tx, err := pool.Begin(...) or pool.BeginTx(...)
			if assign, ok := stmt.(*ast.AssignStmt); ok {
				for i, rhs := range assign.Rhs {
					if call, ok := rhs.(*ast.CallExpr); ok {
						name := getCallMethodName(call.Fun)
						if name == "Begin" || name == "BeginTx" {
							if i < len(assign.Lhs) {
								if id, ok := assign.Lhs[i].(*ast.Ident); ok {
									inTx = true
									txVarName = id.Name
								}
							}
						}
					}
				}
			}
			continue
		}

		// When inTx == true:
		if _, isDefer := stmt.(*ast.DeferStmt); isDefer {
			continue
		}

		if isTxEndStmt(stmt, txVarName) {
			inTx = false
			txVarName = ""
			continue
		}

		onStmtInTx(stmt)
	}
}

// CheckTxNode inspects an AST node inside a transaction and evaluates expensive CPU calls or helper functions.
func CheckTxNode(node ast.Node, funcDecls map[string]*ast.FuncDecl, visited map[string]bool, fset *token.FileSet, dm *directives.DirectiveMap, report func(pos token.Pos, reason string)) {
	if node == nil {
		return
	}

	call, ok := node.(*ast.CallExpr)
	if !ok {
		return
	}

	if dm != nil && (dm.IsIgnored(fset, call.Pos(), RuleCode) || dm.IsIgnored(fset, call.Pos(), RuleCode+".EXPENSIVE-CPU")) {
		return
	}

	if isExp, reason := MatchExpensiveCPUCall(call); isExp {
		report(call.Pos(), reason)
		return
	}

	// Follow intra-package helper calls
	if ident, ok := call.Fun.(*ast.Ident); ok {
		fnName := ident.Name
		if helperFn, exists := funcDecls[fnName]; exists && helperFn.Body != nil {
			if !visited[fnName] {
				visited[fnName] = true
				ast.Inspect(helperFn.Body, func(innerNode ast.Node) bool {
					CheckTxNode(innerNode, funcDecls, visited, fset, dm, report)
					return true
				})
				visited[fnName] = false
			}
		}
	}
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

func getCallMethodName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	}
	return ""
}
