// Package a04_orderby traces data flow of variables interpolated into SQL ORDER BY clauses,
// enforcing that dynamic sort identifiers originate from closed-set allowlist maps or switch-case branches.
package a04_orderby

import (
	"go/ast"
	"go/token"
)

// IsSafeOrderBy checks whether a variable used in an ORDER BY clause is safely mapped
// from an allowlist map lookup or a switch-case statement with static string literals.
func IsSafeOrderBy(ident *ast.Ident, body *ast.BlockStmt) bool {
	if ident == nil || body == nil {
		return false
	}

	var isSafe bool
	var hasAssignment bool

	ast.Inspect(body, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range stmt.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok || id.Name != ident.Name || i >= len(stmt.Rhs) {
					continue
				}
				hasAssignment = true
				rhs := stmt.Rhs[i]

				// 1. Allowlist map lookup: val, ok := sortMap[userSort] or val := sortMap[key]
				if _, isIndex := rhs.(*ast.IndexExpr); isIndex {
					isSafe = true
					return true
				}

				// 2. Static string literal assignment: col = "created_at"
				if lit, isLit := rhs.(*ast.BasicLit); isLit && lit.Kind == token.STRING {
					isSafe = true
					return true
				}
			}

		case *ast.SwitchStmt:
			// Check if variable is assigned inside switch-case branches
			if checkSwitchCasesAssign(stmt, ident.Name) {
				hasAssignment = true
				isSafe = true
			}
		}

		return true
	})

	// If variable was assigned from an allowlist or switch-case, it is safe
	if hasAssignment && isSafe {
		return true
	}

	return false
}

// checkSwitchCasesAssign checks if every branch in a switch assigns a static string literal to varName.
func checkSwitchCasesAssign(sw *ast.SwitchStmt, varName string) bool {
	if sw == nil || sw.Body == nil {
		return false
	}

	foundBranch := false
	for _, stmt := range sw.Body.List {
		cc, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}
		for _, s := range cc.Body {
			assign, ok := s.(*ast.AssignStmt)
			if !ok {
				continue
			}
			for i, lhs := range assign.Lhs {
				id, ok := lhs.(*ast.Ident)
				if ok && id.Name == varName && i < len(assign.Rhs) {
					if lit, ok := assign.Rhs[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						foundBranch = true
					}
				}
			}
		}
	}
	return foundBranch
}

// IsSortDirectionSafe verifies that a direction variable strictly evaluates to "ASC" or "DESC".
func IsSortDirectionSafe(ident *ast.Ident, body *ast.BlockStmt) bool {
	if ident == nil || body == nil {
		return false
	}

	isSafe := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.IfStmt:
			// e.g. if dir == "DESC" { ... } else { dir = "ASC" }
			if bin, ok := stmt.Cond.(*ast.BinaryExpr); ok {
				if isIdentComparingLit(bin, ident.Name, `"DESC"`) || isIdentComparingLit(bin, ident.Name, `"ASC"`) {
					isSafe = true
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range stmt.Lhs {
				id, ok := lhs.(*ast.Ident)
				if ok && id.Name == ident.Name && i < len(stmt.Rhs) {
					if lit, ok := stmt.Rhs[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						if lit.Value == `"ASC"` || lit.Value == `"DESC"` {
							isSafe = true
						}
					}
				}
			}
		}
		return true
	})
	return isSafe
}

func isIdentComparingLit(bin *ast.BinaryExpr, identName, litVal string) bool {
	if bin.Op != token.EQL && bin.Op != token.NEQ {
		return false
	}
	if id, ok := bin.X.(*ast.Ident); ok && id.Name == identName {
		if lit, ok := bin.Y.(*ast.BasicLit); ok && lit.Value == litVal {
			return true
		}
	}
	if id, ok := bin.Y.(*ast.Ident); ok && id.Name == identName {
		if lit, ok := bin.X.(*ast.BasicLit); ok && lit.Value == litVal {
			return true
		}
	}
	return false
}
