// Package a02_unclosed_rows enforces that every pgx.Rows produced by Query()
// is safely closed via defer rows.Close() or consumed by an auto-closing helper.
package a02_unclosed_rows

import (
	"go/ast"
)

// IsReturned checks whether any active holder of the query resource is returned.
func IsReturned(body *ast.BlockStmt, assign *ast.AssignStmt, rowsVar string) bool {
	if body == nil {
		return false
	}
	aliases := collectAliases(body, assign, rowsVar)
	returned := false

	ast.Inspect(body, func(n ast.Node) bool {
		if returned {
			return false
		}
		if ret, ok := n.(*ast.ReturnStmt); ok {
			for _, res := range ret.Results {
				if id, ok := res.(*ast.Ident); ok && aliases[id.Name] {
					returned = true
					return false
				}
			}
		}
		return true
	})
	return returned
}

// IsAssignSafelyClosed verifies whether this specific assignment of rowsVar is closed via unconditional defer
// or consumed by an auto-closing helper, defending against conditional defer traps and alias reassignments.
func IsAssignSafelyClosed(body *ast.BlockStmt, assign *ast.AssignStmt, rowsVar string) bool {
	if body == nil || assign == nil {
		return false
	}

	stmts, idx := findEnclosingStmtList(body, assign)
	if stmts == nil || idx < 0 {
		return false
	}

	activeHolders := map[string]bool{rowsVar: true}

	for j := idx + 1; j < len(stmts); j++ {
		stmt := stmts[j]

		// 1. Direct unconditional defer statement at this block level
		if defStmt, ok := stmt.(*ast.DeferStmt); ok {
			if isCloseCallOnAnyHolder(defStmt.Call, activeHolders) {
				return true
			}
			if isClosureClosingAnyHolder(defStmt.Call, activeHolders) {
				return true
			}
		}

		// 2. Auto-closing helper consuming the resource (e.g. return pgx.CollectRows(rows))
		if isAutoClosingHelperCall(stmt, activeHolders) {
			return true
		}

		// 3. Statement-level alias creation and reassignment invalidation
		if as, ok := stmt.(*ast.AssignStmt); ok {
			handleAssignAliases(as, activeHolders)
		}
	}

	return false
}

func handleAssignAliases(as *ast.AssignStmt, activeHolders map[string]bool) {
	newAliases := make([]string, 0)
	for i, rhs := range as.Rhs {
		if id, ok := rhs.(*ast.Ident); ok && activeHolders[id.Name] {
			if i < len(as.Lhs) {
				if lhsId, ok := as.Lhs[i].(*ast.Ident); ok && lhsId.Name != "_" {
					newAliases = append(newAliases, lhsId.Name)
				}
			}
		}
	}

	// Decouple existing holders that are overwritten by something other than an active holder
	for i, lhs := range as.Lhs {
		if lhsId, ok := lhs.(*ast.Ident); ok && activeHolders[lhsId.Name] {
			rhsIsHolder := false
			if i < len(as.Rhs) {
				if rhsId, ok := as.Rhs[i].(*ast.Ident); ok && activeHolders[rhsId.Name] {
					rhsIsHolder = true
				}
			}
			if !rhsIsHolder {
				delete(activeHolders, lhsId.Name)
			}
		}
	}

	for _, alias := range newAliases {
		activeHolders[alias] = true
	}
}

func collectAliases(body *ast.BlockStmt, assign *ast.AssignStmt, rowsVar string) map[string]bool {
	aliases := map[string]bool{rowsVar: true}
	if body == nil {
		return aliases
	}
	ast.Inspect(body, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok {
			if assign != nil && as.Pos() <= assign.Pos() {
				return true
			}
			for i, rhs := range as.Rhs {
				if id, ok := rhs.(*ast.Ident); ok && aliases[id.Name] && i < len(as.Lhs) {
					if lhsId, ok := as.Lhs[i].(*ast.Ident); ok && lhsId.Name != "_" {
						aliases[lhsId.Name] = true
					}
				}
			}
		}
		return true
	})
	return aliases
}

func findEnclosingStmtList(root *ast.BlockStmt, target ast.Node) ([]ast.Stmt, int) {
	if root == nil || target == nil {
		return nil, -1
	}

	var bestList []ast.Stmt
	var bestIdx = -1

	var searchList func(stmts []ast.Stmt)
	searchList = func(stmts []ast.Stmt) {
		for i, stmt := range stmts {
			if target.Pos() >= stmt.Pos() && target.End() <= stmt.End() {
				bestList = stmts
				bestIdx = i
				switch s := stmt.(type) {
				case *ast.BlockStmt:
					searchList(s.List)
				case *ast.IfStmt:
					if s.Body != nil {
						searchList(s.Body.List)
					}
					if s.Else != nil {
						if b, ok := s.Else.(*ast.BlockStmt); ok {
							searchList(b.List)
						}
					}
				case *ast.ForStmt:
					if s.Body != nil {
						searchList(s.Body.List)
					}
				case *ast.RangeStmt:
					if s.Body != nil {
						searchList(s.Body.List)
					}
				}
				return
			}
		}
	}

	searchList(root.List)
	return bestList, bestIdx
}

func isCloseCallOnAnyHolder(call *ast.CallExpr, holders map[string]bool) bool {
	if call == nil {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Close" {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	return ok && holders[ident.Name]
}

func isClosureClosingAnyHolder(call *ast.CallExpr, holders map[string]bool) bool {
	if call == nil {
		return false
	}
	fnLit, ok := call.Fun.(*ast.FuncLit)
	if !ok || fnLit.Body == nil {
		return false
	}
	found := false
	ast.Inspect(fnLit.Body, func(n ast.Node) bool {
		if found {
			return false
		}
		if c, ok := n.(*ast.CallExpr); ok {
			if isCloseCallOnAnyHolder(c, holders) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func isAutoClosingHelperCall(stmt ast.Stmt, holders map[string]bool) bool {
	found := false
	ast.Inspect(stmt, func(n ast.Node) bool {
		if found {
			return false
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if isAutoClosingHelper(call, holders) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func isAutoClosingHelper(call *ast.CallExpr, holders map[string]bool) bool {
	if call == nil || len(call.Args) == 0 {
		return false
	}
	argIdent, ok := call.Args[0].(*ast.Ident)
	if !ok || !holders[argIdent.Name] {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch sel.Sel.Name {
	case "CollectRows", "CollectOneRow", "ForEachRow":
		return true
	}
	return false
}
