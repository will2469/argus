// Package a04_orderby validates switch-case statements used to map dynamic ORDER BY columns,
// ensuring that all execution paths either assign approved static literals or terminate control flow.
package a04_orderby

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// isExhaustiveSafeSwitch checks if every branch in a switch assigns a safe identifier or terminates.
// Returns (isSafe, hasUnsafeAssignment).
func isExhaustiveSafeSwitch(sw *ast.SwitchStmt, varName string, body *ast.BlockStmt, file *ast.File, pass *analysis.Pass, allowedCols []string) (bool, bool) {
	if sw == nil || sw.Body == nil || len(sw.Body.List) == 0 {
		return false, false
	}

	hasPriorSafeInit := isVarSafelyInitializedPrior(varName, sw, body, file, pass, allowedCols)
	var hasDefault bool
	var branchCount int
	var anyAssignToVar bool

	for _, stmt := range sw.Body.List {
		cc, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}
		branchCount++
		if len(cc.List) == 0 {
			hasDefault = true
		}

		if isBranchTerminating(cc.Body) {
			continue
		}

		hasSafe, hasUnsafe := checkCaseClauseAssign(cc, varName, body, file, pass, allowedCols)
		if hasSafe || hasUnsafe {
			anyAssignToVar = true
		}

		if hasUnsafe {
			return false, true
		}

		if !hasSafe {
			if !hasPriorSafeInit {
				return false, anyAssignToVar
			}
		}
	}

	if branchCount == 0 || !anyAssignToVar {
		return false, false
	}

	if !hasDefault && !hasPriorSafeInit {
		return false, false
	}

	return true, false
}

func checkCaseClauseAssign(cc *ast.CaseClause, varName string, body *ast.BlockStmt, file *ast.File, pass *analysis.Pass, allowedCols []string) (hasSafe bool, hasUnsafe bool) {
	for _, s := range cc.Body {
		assign, ok := s.(*ast.AssignStmt)
		if !ok {
			continue
		}
		for i, lhs := range assign.Lhs {
			id, ok := lhs.(*ast.Ident)
			if !ok || id.Name != varName || i >= len(assign.Rhs) {
				continue
			}
			rhs := assign.Rhs[i]

			if lit, ok := rhs.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if len(allowedCols) > 0 {
					if isColAllowed(unquoteString(lit.Value), allowedCols) {
						hasSafe = true
					} else {
						hasUnsafe = true
					}
				} else {
					hasSafe = true
				}
				continue
			}

			if idx, ok := rhs.(*ast.IndexExpr); ok {
				if isAllowlistMapLookup(idx, body, file, pass, allowedCols) {
					hasSafe = true
				} else {
					hasUnsafe = true
				}
				continue
			}

			hasUnsafe = true
		}
	}
	return hasSafe, hasUnsafe
}

func isBranchTerminating(stmts []ast.Stmt) bool {
	for _, s := range stmts {
		switch stmt := s.(type) {
		case *ast.ReturnStmt:
			return true
		case *ast.ExprStmt:
			if isCallTerminating(stmt.X) {
				return true
			}
		}
	}
	return false
}

func isCallTerminating(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "panic" {
		return true
	}
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		switch sel.Sel.Name {
		case "Fatal", "Fatalf", "Fatalln", "Panic", "Panicf", "Panicln", "Exit":
			return true
		}
	}
	return false
}

func isVarSafelyInitializedPrior(varName string, targetNode ast.Node, body *ast.BlockStmt, file *ast.File, pass *analysis.Pass, allowedCols []string) bool {
	if body == nil {
		return false
	}
	var safelyInit bool
	for _, s := range body.List {
		if s.Pos() >= targetNode.Pos() {
			break
		}
		assign, ok := s.(*ast.AssignStmt)
		if !ok {
			continue
		}
		for i, lhs := range assign.Lhs {
			id, ok := lhs.(*ast.Ident)
			if !ok || id.Name != varName || i >= len(assign.Rhs) {
				continue
			}
			rhs := assign.Rhs[i]
			if lit, ok := rhs.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if len(allowedCols) > 0 {
					safelyInit = isColAllowed(unquoteString(lit.Value), allowedCols)
				} else {
					safelyInit = true
				}
			} else if idx, ok := rhs.(*ast.IndexExpr); ok {
				safelyInit = isAllowlistMapLookup(idx, body, file, pass, allowedCols)
			} else {
				safelyInit = false
			}
		}
	}
	return safelyInit
}
